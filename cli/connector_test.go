package cli_test

import (
	"context"
	"testing"

	"github.com/modernice/goes/cli"
	"github.com/modernice/goes/cli/internal/clitest"
	"github.com/modernice/goes/cli/internal/proto"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestConnector_Serve(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, bus, store := clitest.SetupEvents()

	schedule := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})
	received := make(chan struct{})
	subscribeErrors, err := schedule.Subscribe(ctx, func(projection.Job) error {
		close(received)
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe to schedule: %v", err)
	}

	svc := projection.NewService(bus, projection.RegisterSchedule("example", schedule))
	serviceErrors, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("run projection service: %v", err)
	}

	c := cli.NewConnector(svc)

	_, lis, serveError, closed := serve(t, ctx, c)

	conn := clitest.ClientConn(t, lis)
	client := proto.NewProjectionServiceClient(conn)

	resp, err := client.Trigger(ctx, &proto.TriggerRequest{Schedule: "example"})
	if err != nil {
		t.Fatalf("Trigger failed with %q", err)
	}

	if !resp.Accepted {
		t.Fatalf("invalid response. Accepted should be %v; is %v", true, resp.Accepted)
	}

L:
	for {
		select {
		case err := <-subscribeErrors:
			t.Fatal(err)
		case err := <-serveError:
			t.Fatal(err)
		case err := <-serviceErrors:
			t.Fatal(err)
		case <-received:
			break L
		}
	}

	cancel()
	<-closed
}

func serve(t *testing.T, ctx context.Context, c *cli.Connector) (*grpc.Server, *bufconn.Listener, <-chan error, <-chan struct{}) {
	srv, _, lis := clitest.NewServer(t, nil)
	serveError := make(chan error)
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		if err := c.Serve(ctx, cli.Server(srv), cli.Listener(lis)); err != nil {
			serveError <- err
		}
	}()
	return srv, lis, serveError, closed
}
