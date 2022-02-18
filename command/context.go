package command

import (
	"context"
	"sync"

	"github.com/modernice/goes"
	"github.com/modernice/goes/command/finish"
)

type cmdctx[P any, ID goes.ID] struct {
	context.Context
	Of[P, ID]

	mux sync.Mutex
	options
	finished bool
}

// ContextOption is a Context option.
type ContextOption func(*options)

type options struct {
	whenDone func(context.Context, finish.Config) error
}

// WhenDone returns an Option that makes the delegates calls to ctx.Done() to
// fn.
func WhenDone(fn func(context.Context, finish.Config) error) ContextOption {
	return func(opts *options) {
		opts.whenDone = fn
	}
}

// NewContext returns a context for the given command.
func NewContext[P any, ID goes.ID](base context.Context, cmd Of[P, ID], opts ...ContextOption) ContextOf[P, ID] {
	ctx := cmdctx[P, ID]{
		Context: base,
		Of:      cmd,
	}
	for _, opt := range opts {
		opt(&ctx.options)
	}
	return &ctx
}

func (ctx *cmdctx[P, ID]) AggregateID() ID {
	id, _ := ctx.Aggregate()
	return id
}

func (ctx *cmdctx[P, ID]) AggregateName() string {
	_, name := ctx.Aggregate()
	return name
}

func (ctx *cmdctx[P, ID]) Finish(c context.Context, opts ...finish.Option) error {
	ctx.mux.Lock()
	defer ctx.mux.Unlock()
	if ctx.finished {
		return ErrAlreadyFinished
	}
	ctx.finished = true
	if ctx.whenDone != nil {
		return ctx.whenDone(c, finish.Configure(opts...))
	}
	return nil
}

func TryCastContext[To, From any, ID goes.ID](ctx ContextOf[From, ID]) (ContextOf[To, ID], bool) {
	cmd, ok := TryCast[To, From, ID](ctx)
	if !ok {
		return nil, false
	}

	var opts []ContextOption
	if ctx, ok := ctx.(*cmdctx[From, ID]); ok {
		opts = append(opts, WhenDone(ctx.whenDone))
	}

	return NewContext[To, ID](ctx, cmd, opts...), true
}

func CastContext[To, From any, ID goes.ID](ctx ContextOf[From, ID]) ContextOf[To, ID] {
	cmd := Cast[To, From, ID](ctx)

	var opts []ContextOption
	if ctx, ok := ctx.(*cmdctx[From, ID]); ok {
		opts = append(opts, WhenDone(ctx.whenDone))
	}

	return NewContext[To, ID](ctx, cmd, opts...)
}
