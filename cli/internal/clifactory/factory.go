package clifactory

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

var (
	// ErrConnectorUnavailable is returned when the CLI cannot connect to a
	// Connector.
	ErrConnectorUnavailable = errors.New("connector unavailable")

	// DefaultConnectorAddress is the default address of a Connector.
	DefaultConnectorAddress = "localhost:8000"
)

// Factory is used by commands to provide common configuration.
type Factory struct {
	Context          context.Context
	ConnectorAddress string
	ConnectTimeout   time.Duration
	TestListener     *bufconn.Listener

	mux  sync.Mutex
	conn *grpc.ClientConn
}

// Option is a Factory option.
type Option func(*Factory)

// Context returns an Option that sets the Context of a Factory.
func Context(ctx context.Context) Option {
	return func(f *Factory) {
		f.Context = ctx
	}
}

// ConnectorAddress returns an Option that specifies the address of the
// Connector service.
func ConnectorAddress(addr string) Option {
	return func(f *Factory) {
		f.ConnectorAddress = addr
	}
}

// ConnectTimeout returns an Option that specifies the timeout for connecting to
// a Connector.
func ConnectTimeout(d time.Duration) Option {
	return func(f *Factory) {
		f.ConnectTimeout = d
	}
}

// TestListener returns an Option that provides a Factory with a
// *bufconn.Listener. When connecting to a Connector, Factory will use the
// Listener to dial the gRPC server instead of using the ConnectorAddress.
func TestListener(lis *bufconn.Listener) Option {
	return func(f *Factory) {
		f.TestListener = lis
	}
}

// New returns a new Factory.
func New(opts ...Option) *Factory {
	f := Factory{ConnectorAddress: DefaultConnectorAddress}
	for _, opt := range opts {
		opt(&f)
	}
	if f.Context == nil {
		f.Context = context.Background()
	}
	return &f
}

// Connect connects to the Connector service and returns the *grpc.ClientConn.
// If the connection to the Connector cannot be established within the
// configured ConnectTimeout Duration, ErrConnectorUnavailable is returned.
func (f *Factory) Connect(ctx context.Context) (*grpc.ClientConn, error) {
	f.mux.Lock()
	defer f.mux.Unlock()

	if f.conn != nil {
		return f.conn, nil
	}

	if f.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.ConnectTimeout)
		defer cancel()
	}

	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBlock(),
	}

	if f.TestListener != nil {
		opts = append(opts, grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return f.TestListener.Dial()
		}))
	}

	conn, err := grpc.DialContext(ctx, f.ConnectorAddress, opts...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrConnectorUnavailable
		}
		return nil, fmt.Errorf("grpc.DialContext: %w", err)
	}
	f.conn = conn

	return conn, nil
}
