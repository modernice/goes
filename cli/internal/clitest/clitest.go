package clitest

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// SetupEvents returns an event registry, bus and store.
func SetupEvents() (*codec.Registry, event.Bus, event.Store) {
	reg := codec.New()
	bus := eventbus.New()
	store := eventstore.New()
	return reg, bus, store
}

// NewServer returns a new *grpc.Server and a *grpc.ClientConn that is connected
// to the returned *bufconn.Listener.
func NewServer(t *testing.T, init func(*grpc.Server)) (*grpc.Server, *grpc.ClientConn, *bufconn.Listener) {
	srv, lis := newServer()
	if init != nil {
		init(srv)
	}
	return srv, ClientConn(t, lis), lis
}

// NewRunningServer returns the same as NewServer, but also starts the server in a new goroutine.
func NewRunningServer(t *testing.T, init func(*grpc.Server)) (*grpc.Server, *grpc.ClientConn, *bufconn.Listener) {
	srv, conn, lis := NewServer(t, init)
	go func() {
		if err := srv.Serve(lis); err != nil {
			panic(err)
		}
	}()
	return srv, conn, lis
}

// ClientConn returns a *grpc.ClientConn that dials using the provided
// *bufconn.Listener.
func ClientConn(t *testing.T, lis *bufconn.Listener) *grpc.ClientConn {
	conn, err := grpc.DialContext(
		context.Background(), "",
		grpc.WithInsecure(),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
	)
	if err != nil {
		t.Fatal(fmt.Errorf("grpc.DialContext: %w", err))
	}
	return conn
}

func newServer() (*grpc.Server, *bufconn.Listener) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	return srv, lis
}
