package order

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/helper/fanin"
)

const (
	// PlaceCommand is the Command name for placing an Order.
	PlaceCommand = "order.place"

	// CancelCommand is the Command name for canceling an order.
	CancelCommand = "order.cancel"
)

// PlacePayload is the payload for placing an Order.
type PlacePayload struct {
	Customer Customer
	Items    []Item
}

// CancelPayload is the payload for canceling an Order.
type CancelPayload struct{}

// Place returns the Command for placing an Order with the given UUID and Items
// for the provided Customer.
func Place(id uuid.UUID, cus Customer, items ...Item) command.Command {
	return command.New(
		PlaceCommand,
		PlacePayload{
			Customer: cus,
			Items:    items,
		},
		// Add Aggregate information to the Command so that the Command handler
		// can create the Order with the correct UUID.
		command.Aggregate(AggregateName, id),
	)
}

// Cancel returns the Command for canceling the Order with the given UUID.
func Cancel(id uuid.UUID) command.Command {
	return command.New(CancelCommand, CancelPayload{}, command.Aggregate(AggregateName, id))
}

// RegisterCommand registers the Order Commands into a Command Registry.
func RegisterCommands(r command.Registry) {
	r.Register(PlaceCommand, func() command.Payload { return PlacePayload{} })
	r.Register(CancelCommand, func() command.Payload { return CancelPayload{} })
}

// HandleCommands starts handling Order Commands in the background and returns a
// channel of asynchronous errors.
func HandleCommands(ctx context.Context, bus command.Bus, repo aggregate.Repository) (<-chan error, error) {
	h := command.NewHandler(bus)

	placeErrors, err := h.Handle(
		ctx, PlaceCommand,
		func(ctx command.Context) error {
			// Get the Command from the Context
			cmd := ctx.Command()

			// Get the Command Payload
			load := cmd.Payload().(PlacePayload)

			// Create an Order with the AggregateID from the Command.
			o := New(cmd.AggregateID())

			// Place the Order on the actual Order Aggregate.
			if err := o.Place(load.Customer, load.Items); err != nil {
				return err
			}

			// Save the Order (Events) in the Aggregate Repository.
			if err := repo.Save(ctx, o); err != nil {
				return fmt.Errorf("save Order: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("register %q Command handler: %w", PlaceCommand, err)
	}

	cancelErrors, err := h.Handle(
		ctx, CancelCommand,
		func(ctx command.Context) error {
			cmd := ctx.Command()

			// Create the Order with the AggregateID from the Command.
			o := New(cmd.AggregateID())

			// Fetch the existing Events for the Order and apply them to build
			// the current state of the Order.
			if err := repo.Fetch(ctx, o); err != nil {
				return fmt.Errorf("fetch Order: %w", err)
			}

			// Cancel the Order on the actual Order Aggregate.
			if err := o.Cancel(); err != nil {
				return err
			}

			// Save the new Order Events to the Aggregate Repository.
			if err := repo.Save(ctx, o); err != nil {
				return fmt.Errorf("save Order: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("register %q Command handler: %w", CancelCommand, err)
	}

	// Merge the error channels from the Command handlers into a single error
	// channel and close it when ctx is canceled.
	out, stop := fanin.Errors(placeErrors, cancelErrors)
	go func() {
		<-ctx.Done()
		stop()
	}()

	return out, nil
}
