// Package eventstoreui provides the standalone web UI server for goes event stores.
package eventstoreui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/modernice/goes/codec"
	internal "github.com/modernice/goes/internal/eventstoreui"
)

type Config = internal.Config
type StoreConfig = internal.StoreConfig

type options struct {
	decoder internal.Decoder
}

// Option configures an App.
type Option func(*options)

// WithEncoding decodes event data with the application's own goes codec.
// The encoding must register every event name that should be inspectable.
func WithEncoding(encoding codec.Encoding) Option {
	return func(opts *options) {
		opts.decoder = encoding
	}
}

// LoadConfig loads GOES_UI_* environment configuration.
func LoadConfig() (Config, error) { return internal.LoadConfig() }

// App is the standalone event-store UI application.
type App struct {
	internal *internal.App
}

// New builds an App. Without WithEncoding, event payloads are decoded as generic JSON.
func New(cfg Config, appOptions ...Option) (*App, error) {
	opts := options{decoder: internal.JSONDecoder{}}
	for _, option := range appOptions {
		option(&opts)
	}
	app, err := internal.New(cfg, opts.decoder)
	if err != nil {
		return nil, err
	}
	return &App{internal: app}, nil
}

// Handler returns the complete API and frontend HTTP handler.
func (app *App) Handler() http.Handler { return app.internal.Handler() }

// Close releases all database clients.
func (app *App) Close() { app.internal.Close() }

// Run serves the application until ctx is canceled.
func (app *App) Run(ctx context.Context, address string) error {
	server := &http.Server{
		Addr: address, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		case <-stopped:
		}
	}()
	err := server.ListenAndServe()
	close(stopped)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Main loads environment configuration and runs an App until SIGINT or SIGTERM.
// It is the shortest entry point for both the stock binary and custom-codec builds.
func Main(appOptions ...Option) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("configure goes UI: %w", err)
	}
	app, err := New(cfg, appOptions...)
	if err != nil {
		return fmt.Errorf("initialize goes UI: %w", err)
	}
	defer app.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if !cfg.AuthenticationEnabled() {
		log.Printf("WARNING: goes UI authentication is disabled; do not expose this service in production")
	}
	log.Printf("goes UI listening on %s with %d configured store(s)", cfg.ListenAddress, len(cfg.Stores))
	if err := app.Run(ctx, cfg.ListenAddress); err != nil {
		return fmt.Errorf("serve goes UI: %w", err)
	}
	return nil
}
