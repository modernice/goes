package projectioncmd_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/modernice/goes/cli"
	"github.com/modernice/goes/cli/internal/clifactory"
	"github.com/modernice/goes/cli/internal/clitest"
	"github.com/modernice/goes/cli/internal/cmd/projectioncmd"
	"github.com/modernice/goes/cli/internal/cmdtest"
	"github.com/modernice/goes/cli/internal/projectionrpc"
	"github.com/modernice/goes/cli/internal/proto"
	"github.com/modernice/goes/internal/projectiontest"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
	"google.golang.org/grpc"
)

func TestCommand_Trigger_unhandledTrigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bus, _ := clitest.SetupEvents()
	svc := projection.NewService(bus, projection.TriggerTimeout(20*time.Millisecond))
	_, conn, lis := clitest.NewServer(t, func(s *grpc.Server) {
		proto.RegisterProjectionServiceServer(s, projectionrpc.NewServer(svc))
	})
	defer conn.Close()

	f := clifactory.New(
		clifactory.Context(ctx),
		clifactory.ConnectorAddress(lis.Addr().String()),
		clifactory.ConnectTimeout(50*time.Millisecond),
	)

	cmd := projectioncmd.New(f)
	cmdtest.Error(t, cmd, []string{"trigger", "foo", "bar", "baz"}, clifactory.ErrConnectorUnavailable)
}

func TestCommand_Trigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bus, store := clitest.SetupEvents()
	schedule := schedule.Continuously(bus, store, []string{"foo"})
	received := make(chan struct{})
	subscribeErrors, err := schedule.Subscribe(ctx, func(projection.Job) error {
		received <- struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe to schedule: %v", err)
	}
	svc := projection.NewService(
		bus,
		projection.RegisterSchedule("foo", schedule),
		projection.RegisterSchedule("bar", schedule),
		projection.RegisterSchedule("baz", schedule),
	)
	serviceErrors, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("run projection service: %v", err)
	}

	srv, conn, lis := clitest.NewServer(t, nil)
	defer conn.Close()

	connector := cli.NewConnector(svc)
	go func() {
		if err := connector.Serve(ctx, cli.Listener(lis), cli.Server(srv)); err != nil {
			panic(err)
		}
	}()

	f := clifactory.New(
		clifactory.Context(ctx),
		clifactory.TestListener(lis),
	)

	cmd := projectioncmd.New(f)

	cmdtest.TableOutput(t, cmd, []string{"trigger", "foo", "bar", "baz"}, [][]string{
		{"Schedules triggered."},
		{},
		{"Schedules:", fmt.Sprint([]string{"foo", "bar", "baz"})},
		{"Reset:", fmt.Sprint(false)},
	}, aurora.Green)

	var receiveCount int
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			t.Fatal("timed out")
		case err := <-subscribeErrors:
			t.Fatal(err)
		case err := <-serviceErrors:
			t.Fatal(err)
		case <-received:
			receiveCount++
			if receiveCount == 3 {
				return
			}
		}
	}
}

func TestCommand_Trigger_reset(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bus, store := clitest.SetupEvents()
	schedule := schedule.Continuously(bus, store, []string{"foo"})
	proj := projectiontest.NewMockResetProjection(8)
	applied := make(chan struct{})
	subscribeErrors, err := schedule.Subscribe(ctx, func(job projection.Job) error {
		defer close(applied)
		return job.Apply(job, proj)
	})
	if err != nil {
		t.Fatalf("subscribe to schedule: %v", err)
	}
	svc := projection.NewService(bus, projection.RegisterSchedule("foo", schedule))
	serviceErrors, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("run projection service: %v", err)
	}

	srv, conn, lis := clitest.NewServer(t, nil)
	defer conn.Close()

	connector := cli.NewConnector(svc)
	go func() {
		if err := connector.Serve(ctx, cli.Listener(lis), cli.Server(srv)); err != nil {
			panic(err)
		}
	}()

	f := clifactory.New(
		clifactory.Context(ctx),
		clifactory.TestListener(lis),
	)

	cmd := projectioncmd.New(f)
	cmdtest.TableOutput(t, cmd, []string{"trigger", "foo", "--reset"}, [][]string{
		{"Schedules triggered."},
		{},
		{"Schedules:", fmt.Sprint([]string{"foo"})},
		{"Reset:", fmt.Sprint(true)},
	}, aurora.Green)

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			t.Fatal("timed out")
		case err := <-subscribeErrors:
			t.Fatal(err)
		case err := <-serviceErrors:
			t.Fatal(err)
		case <-applied:
			if proj.Foo != 0 {
				t.Fatalf("Projection should have been reset")
			}
			return
		}
	}
}
