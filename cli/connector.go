package cli

import (
	"context"
	"fmt"
	"net"

	protoprojection "github.com/modernice/goes/api/proto/gen/projection"
	"github.com/modernice/goes/cli/internal/projectionrpc"
	"github.com/modernice/goes/projection"
	"google.golang.org/grpc"
)

const (
	// DefaultPort is the default port used to serve the Connector.
	DefaultPort = uint16(8000)
)

// Connector provides the gRPC server for CLI commands.
type Connector struct {
	projectionService *projection.Service
}

// ServeOption is an option for serving a Connetor.
type ServeOption func(*serveConfig)

// Port returns a ServeOption that specifies the port to use when creating the
// Listener for a Connector. Default port is 8000. Port has no effect when
// providing a custom Listener through the Listener ServeOption.
func Port(p uint16) ServeOption {
	return func(cfg *serveConfig) {
		cfg.port = p
	}
}

// Server returns a ServeOption that specifies the underlying grpc.Server to
// use.
func Server(srv *grpc.Server) ServeOption {
	return func(cfg *serveConfig) {
		cfg.server = srv
	}
}

// Listener returns a ServeOption that provides a Connector with a custom
// Listener. When a Listener is provided, the Port ServeOption has no effect.
func Listener(lis net.Listener) ServeOption {
	return func(cfg *serveConfig) {
		cfg.lis = lis
	}
}

// NewConnector returns a new CLI Connector.
func NewConnector(svc *projection.Service) *Connector {
	return &Connector{
		projectionService: svc,
	}
}

// Serve serves the Connector until ctx is canceled.
//
//	c := cli.NewConnector(...)
//	err := c.Serve(context.TODO(), cli.Port(8080))
func (c *Connector) Serve(ctx context.Context, opts ...ServeOption) error {
	cfg, err := c.newServeConfig(opts...)
	if err != nil {
		return err
	}

	serveError := c.serve(ctx, cfg.server, cfg.lis)
	stopped := c.stopOnCancel(ctx, cfg.server)

	select {
	case err := <-serveError:
		return err
	case <-stopped:
		return nil
	}
}

func (c *Connector) newServeConfig(opts ...ServeOption) (serveConfig, error) {
	cfg := serveConfig{port: DefaultPort}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.server == nil {
		cfg.server = grpc.NewServer()
	}
	c.register(cfg.server)

	if cfg.lis == nil {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.port))
		if err != nil {
			return cfg, fmt.Errorf("create Listener: %w", err)
		}
		cfg.lis = lis
	}

	return cfg, nil
}

func (c *Connector) register(srv *grpc.Server) {
	protoprojection.RegisterProjectionServiceServer(srv, projectionrpc.NewServer(c.projectionService))
}

func (c *Connector) serve(ctx context.Context, srv *grpc.Server, lis net.Listener) <-chan error {
	serveError := make(chan error)
	go func() {
		if err := srv.Serve(lis); err != nil {
			select {
			case <-ctx.Done():
			case serveError <- err:
			}
		}
	}()
	return serveError
}

func (c *Connector) stopOnCancel(ctx context.Context, srv *grpc.Server) <-chan struct{} {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		srv.GracefulStop()
	}()
	return stopped
}

type serveConfig struct {
	port   uint16
	server *grpc.Server
	lis    net.Listener
}
