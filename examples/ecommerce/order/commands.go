package order

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
)

const (
	PlaceCommand  = "order.place"
	CancelCommand = "order.cancel"
)

type PlacePayload struct {
	Customer Customer
	Items    []Item
}

type CancelPayload struct{}

func Place(id uuid.UUID, cus Customer, items ...Item) command.Command {
	return command.New(PlaceCommand, PlacePayload{
		Customer: cus,
		Items:    items,
	}, command.Aggregate(AggregateName, id))
}

func Cancel(id uuid.UUID) command.Command {
	return command.New(CancelCommand, CancelPayload{}, command.Aggregate(AggregateName, id))
}

func RegisterCommands(r command.Registry) {
	r.Register(PlaceCommand, func() command.Payload { return PlacePayload{} })
	r.Register(CancelCommand, func() command.Payload { return CancelPayload{} })
}

func HandleCommands(ctx context.Context, bus command.Bus, repo aggregate.Repository) (<-chan error, error) {
	parentCtx := ctx
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-parentCtx.Done()
		cancel()
	}()

	h := command.NewHandler(bus)

	if err := h.Handle(
		ctx, PlaceCommand,
		func(ctx command.Context) error {
			cmd := ctx.Command()
			load := cmd.Payload().(PlacePayload)
			o := New(cmd.AggregateID())
			if err := repo.Fetch(ctx, o); err != nil {
				return fmt.Errorf("fetch Order: %w", err)
			}
			if err := o.Place(load.Customer, load.Items); err != nil {
				return err
			}
			if err := repo.Save(ctx, o); err != nil {
				return fmt.Errorf("save Order: %w", err)
			}
			return nil
		},
	); err != nil {
		cancel()
		return nil, fmt.Errorf("handle %q Commands: %w", PlaceCommand, err)
	}

	if err := h.Handle(
		ctx, CancelCommand,
		func(ctx command.Context) error {
			cmd := ctx.Command()
			o := New(cmd.AggregateID())
			if err := repo.Fetch(ctx, o); err != nil {
				return fmt.Errorf("fetch Order: %w", err)
			}
			if err := o.Cancel(); err != nil {
				return err
			}
			if err := repo.Save(ctx, o); err != nil {
				return fmt.Errorf("save Order: %w", err)
			}
			return nil
		},
	); err != nil {
		cancel()
		return nil, fmt.Errorf("handle %q Commands: %w", CancelCommand, err)
	}

	return h.Errors(ctx), nil
}
