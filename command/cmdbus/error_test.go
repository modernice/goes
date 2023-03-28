package cmdbus_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/command/finish"
	"github.com/modernice/goes/helper/streams"
)

type errorCode int

var (
	errEnriched = command.NewError(10, errors.New("underlying"))
)

func TestBus_enrichedError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subBus, ebus, reg := newBus(context.Background(), cmdbus.ReceiveTimeout(0), cmdbus.Debug(false))
	pubBus, _, _ := newBusWith(context.Background(), reg, ebus, cmdbus.AssignTimeout(0), cmdbus.Debug(false))

	commands, errs, err := subBus.Subscribe(ctx, "foo-cmd")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	go streams.Walk(ctx, func(ctx command.Context) error {
		return ctx.Finish(ctx, finish.WithError(errEnriched))
	}, commands, errs)

	cmd := command.New("foo-cmd", mockPayload{})

	dispatchError := pubBus.Dispatch(ctx, cmd.Any(), dispatch.Sync())

	if dispatchError == nil {
		t.Fatal("expected dispatch to return an error")
	}

	cmdError := command.Error[int](dispatchError)

	if cmdError.Code() != errEnriched.Code() {
		t.Fatalf("expected code %d, got %d", errEnriched.Code(), cmdError.Code())
	}

	if cmdError.Underlying().Error() != errEnriched.Underlying().Error() {
		t.Fatalf("expected underlying error %q, got %q", errEnriched.Underlying(), cmdError.Underlying())
	}
}
