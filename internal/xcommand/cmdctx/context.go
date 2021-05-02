package cmdctx

import (
	"context"
	"sync"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/finish"
)

type cmdctx struct {
	context.Context

	cmd      command.Command
	whenDone func(context.Context, finish.Config) error
	mux      sync.Mutex
	finished bool
}

// Option is a Context option.
type Option func(*cmdctx)

// WhenDone returns an Option that makes the delegates calls to ctx.Done() to
// fn.
func WhenDone(fn func(context.Context, finish.Config) error) Option {
	return func(ctx *cmdctx) {
		ctx.whenDone = fn
	}
}

// New returns a Context for the given Command.
func New(base context.Context, cmd command.Command, opts ...Option) command.Context {
	ctx := cmdctx{
		Context: base,
		cmd:     cmd,
	}
	for _, opt := range opts {
		opt(&ctx)
	}
	return &ctx
}

func (ctx *cmdctx) Command() command.Command {
	return ctx.cmd
}

func (ctx *cmdctx) Finish(c context.Context, opts ...finish.Option) error {
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
