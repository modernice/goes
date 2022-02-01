package cmdctx

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/finish"
)

type cmdctx[P any] struct {
	context.Context
	command.Of[P]

	whenDone func(context.Context, finish.Config) error
	mux      sync.Mutex
	finished bool
}

// Option is a Context option.
type Option[P any] func(*cmdctx[P])

// WhenDone returns an Option that makes the delegates calls to ctx.Done() to
// fn.
func WhenDone[P any](fn func(context.Context, finish.Config) error) Option[P] {
	return func(ctx *cmdctx[P]) {
		ctx.whenDone = fn
	}
}

// New returns a Context for the given Command.
func New[P any](base context.Context, cmd command.Of[P], opts ...Option[P]) command.ContextOf[P] {
	ctx := cmdctx[P]{
		Context: base,
		Of:      cmd,
	}
	for _, opt := range opts {
		opt(&ctx)
	}
	return &ctx
}

func (ctx *cmdctx[P]) AggregateID() uuid.UUID {
	id, _ := ctx.Aggregate()
	return id
}

func (ctx *cmdctx[P]) AggregateName() string {
	_, name := ctx.Aggregate()
	return name
}

func (ctx *cmdctx[P]) Finish(c context.Context, opts ...finish.Option) error {
	ctx.mux.Lock()
	defer ctx.mux.Unlock()
	if ctx.finished {
		return command.ErrAlreadyFinished
	}
	ctx.finished = true
	if ctx.whenDone != nil {
		return ctx.whenDone(c, finish.Configure(opts...))
	}
	return nil
}
