package clitest

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// Connect connects to
func Connect(t *testing.T, init func(*grpc.Server)) *grpc.ClientConn {
	_, lis := newServer(init)

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

func newServer(init func(*grpc.Server)) (*grpc.Server, *bufconn.Listener) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	init(srv)

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	return srv, lis
}
