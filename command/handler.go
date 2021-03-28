package command

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modernice/goes/command/finish"
)

type Handler struct {
	bus Bus

	mux  sync.Mutex
	errs []chan error
}

func NewHandler(bus Bus) *Handler {
	return &Handler{
		bus: bus,
	}
}

func (h *Handler) Errors(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() {
		<-ctx.Done()
		h.mux.Lock()
		defer h.mux.Unlock()
		for i, errs := range h.errs {
			if errs == out {
				h.errs = append(h.errs[:i], h.errs[i+1:]...)
				return
			}
		}
	}()
	h.mux.Lock()
	defer h.mux.Unlock()
	h.errs = append(h.errs, out)
	return out
}

func (h *Handler) Handle(ctx context.Context, name string, fn func(Context) error) error {
	commands, errs, err := h.bus.Subscribe(ctx, name)
	if err != nil {
		return fmt.Errorf("subscribe to %q Commands: %w", name, err)
	}
	go h.handle(commands, errs, fn)
	return nil
}

func (h *Handler) handle(commands <-chan Context, errs <-chan error, fn func(Context) error) {
	for {
		if errs == nil && commands == nil {
			return
		}
		select {
		case err, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			h.error(err)
		case ctx, ok := <-commands:
			if !ok {
				commands = nil
				break
			}

			start := time.Now()
			err := fn(ctx)
			dur := time.Since(start)
			if err != nil {
				err = fmt.Errorf("handle %q Command: %w", ctx.Command().Name(), err)
				h.error(err)
			}

			if err = ctx.Finish(ctx, finish.WithError(err), finish.WithRuntime(dur)); err != nil {
				err = fmt.Errorf("finish %q Command: %w", ctx.Command().Name(), err)
				h.error(err)
			}
		}
	}
}

func (h *Handler) error(err error) {
	h.mux.Lock()
	defer h.mux.Unlock()
	for _, out := range h.errs {
		out <- err
	}
}
