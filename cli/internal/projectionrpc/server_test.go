package projectionrpc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/modernice/goes/cli/internal/clitest"
	"github.com/modernice/goes/cli/internal/projectionrpc"
	"github.com/modernice/goes/cli/internal/proto"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/internal/projectiontest"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTrigger_unregisteredSchedule(t *testing.T) {
	bus := eventbus.New()
	svc := projection.NewService(bus, projection.TriggerTimeout[any](20*time.Millisecond))

	_, conn, _ := clitest.NewRunningServer(t, func(s *grpc.Server) {
		proto.RegisterProjectionServiceServer(s, projectionrpc.NewServer(svc))
	})
	defer conn.Close()

	client := proto.NewProjectionServiceClient(conn)

	_, err := client.Trigger(context.Background(), &proto.TriggerRequest{
		Schedule: "example",
	})

	stat := status.Convert(err)
	wantMsg := fmt.Sprintf("Trigger for %q schedule not accepted. Forgot to register the schedule in a projection service?", "example")

	if err == nil || stat.Code() != codes.NotFound || stat.Message() != wantMsg {
		t.Fatalf(
			"Trigger should fail with code %d and message %q; got code %d %q",
			codes.NotFound,
			wantMsg,
			stat.Code(),
			stat.Message(),
		)
	}
}

func TestTrigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bus, store := clitest.SetupEvents()
	schedule := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})

	received := make(chan struct{})
	scheduleErrors, err := schedule.Subscribe(ctx, func(projection.Job) error {
		close(received)
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe to schedule: %v", err)
	}

	svc := projection.NewService(bus, projection.RegisterSchedule[any]("example", schedule))
	errs, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("run projection service: %v", err)
	}

	_, conn, _ := clitest.NewRunningServer(t, func(s *grpc.Server) {
		proto.RegisterProjectionServiceServer(s, projectionrpc.NewServer(projection.NewService[any](bus)))
	})
	defer conn.Close()

	client := proto.NewProjectionServiceClient(conn)

	resp, err := client.Trigger(context.Background(), &proto.TriggerRequest{
		Schedule: "example",
	})
	if err != nil {
		t.Fatalf("Trigger failed with %q", err)
	}

	if !resp.Accepted {
		t.Fatalf("TriggerResponse.Accepted should be %v; is %v", true, resp.Accepted)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Fatal("timed out")
	case err := <-errs:
		t.Fatal(err)
	case err := <-scheduleErrors:
		t.Fatal(err)
	case <-received:
	}
}

func TestTrigger_reset(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bus, store := clitest.SetupEvents()
	schedule := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})

	proj := projectiontest.NewMockResetProjection(5)
	applied := make(chan struct{})
	scheduleErrors, err := schedule.Subscribe(ctx, func(job projection.Job) error {
		defer close(applied)
		return job.Apply(job, proj)
	})
	if err != nil {
		t.Fatalf("subscribe to schedule: %v", err)
	}

	svc := projection.NewService(bus, projection.RegisterSchedule[any]("example", schedule))
	errs, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("run projection service: %v", err)
	}

	_, conn, _ := clitest.NewRunningServer(t, func(s *grpc.Server) {
		proto.RegisterProjectionServiceServer(s, projectionrpc.NewServer(projection.NewService[any](bus)))
	})
	defer conn.Close()

	client := proto.NewProjectionServiceClient(conn)

	resp, err := client.Trigger(context.Background(), &proto.TriggerRequest{
		Schedule: "example",
		Reset_:   true,
	})
	if err != nil {
		t.Fatalf("Trigger failed with %q", err)
	}

	if !resp.Accepted {
		t.Fatalf("TriggerResponse.Accepted should be %v; is %v", true, resp.Accepted)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Fatal("timed out")
	case err := <-errs:
		t.Fatal(err)
	case err := <-scheduleErrors:
		t.Fatal(err)
	case <-applied:
	}

	if proj.Foo != 0 {
		t.Fatalf("Projection should have been reset")
	}
}
