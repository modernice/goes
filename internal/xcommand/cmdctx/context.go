package cmdctx

import (
	"context"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/done"
)

type cmdctx struct {
	cmd      command.Command
	whenDone func(context.Context, ...done.Option) error
}

// Option is a Context option.
type Option func(*cmdctx)

// WhenDone returns an Option that makes the delegates calls to ctx.Done() to
// fn.
func WhenDone(fn func(context.Context, ...done.Option) error) Option {
	return func(ctx *cmdctx) {
		ctx.whenDone = fn
	}
}

// New returns a Context for the given Command.
func New(cmd command.Command, opts ...Option) command.Context {
	ctx := cmdctx{cmd: cmd}
	for _, opt := range opts {
		opt(&ctx)
	}
	return &ctx
}

func (ctx *cmdctx) Command() command.Command {
	return ctx.cmd
}

func (ctx *cmdctx) Done(c context.Context, opts ...done.Option) error {
	if ctx.whenDone != nil {
		return ctx.whenDone(c, opts...)
	}
	return nil
}
