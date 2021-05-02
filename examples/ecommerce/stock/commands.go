package stock

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/helper/fanin"
)

const (
	// FillCommand is the Command for filling Stock.
	FillCommand = "stock.fill"

	// ReduceCommand is the Command name for reducing Stock.
	ReduceCommand = "stock.reduce"

	// ReserveCommand is the Command name for reserving Stock.
	ReserveCommand = "stock.reserve"

	// ReleaseCommand is the Command name for releasing Stock.
	ReleaseCommand = "stock.release"

	// ExecuteCommand is the Command name for executing the Stock of an Order.
	ExecuteCommand = "stock.execute"
)

// FillPayload is the Payload of FillCommand.
type FillPayload struct {
	Quantity int
}

// ReducePayload is the Payload for ReduceCommand.
type ReducePayload struct {
	Quantity int
}

// ReservePayload is the Payload for ReserveCommand.
type ReservePayload struct {
	OrderID  uuid.UUID
	Quantity int
}

// ReleasePayload is the Payload for ReleaseCommand.
type ReleasePayload struct {
	OrderID uuid.UUID
}

// ExecutePayload is the Payload for ExecuteCommand.
type ExecutePayload struct {
	OrderID uuid.UUID
}

// Fill returns the Command to fill the Stock of the given Product by the given
// quantity.
func Fill(productID uuid.UUID, quantity int) command.Command {
	return command.New(FillCommand, FillPayload{
		Quantity: quantity,
	}, command.Aggregate(AggregateName, productID))
}

// Reduce returns the Command for reducing the Stock of the given Product by the
// given quantity.
func Reduce(productID uuid.UUID, quantity int) command.Command {
	return command.New(ReduceCommand, ReducePayload{
		Quantity: quantity,
	}, command.Aggregate(AggregateName, productID))
}

// Reserve returns the Command for reserving Stock of the given Product for the
// given Order.
func Reserve(productID, orderID uuid.UUID, quantity int) command.Command {
	return command.New(ReserveCommand, ReservePayload{
		OrderID:  orderID,
		Quantity: quantity,
	}, command.Aggregate(AggregateName, productID))
}

// Release returns the Command for releasing the reserved Stock of the given
// Product for the given Order.
func Release(productID, orderID uuid.UUID) command.Command {
	return command.New(ReleaseCommand, ReleasePayload{
		OrderID: orderID,
	}, command.Aggregate(AggregateName, productID))
}

// Execute returns the Command for executing the Stock for the given Order and
// Product.
func Execute(productID, orderID uuid.UUID) command.Command {
	return command.New(ExecuteCommand, ExecutePayload{
		OrderID: orderID,
	}, command.Aggregate(AggregateName, productID))
}

// RegisterCommands registers the Stock Commands into a Command Registry.
func RegisterCommands(r command.Registry) {
	r.Register(FillCommand, func() command.Payload { return FillPayload{} })
	r.Register(ReduceCommand, func() command.Payload { return ReducePayload{} })
	r.Register(ReserveCommand, func() command.Payload { return ReservePayload{} })
	r.Register(ReleaseCommand, func() command.Payload { return ReleasePayload{} })
	r.Register(ExecuteCommand, func() command.Payload { return ExecutePayload{} })
}

// HandleCommands starts handling Stock Commands in the background and returns a
// channel of asynchronous errors that is closed when ctx is canceled.
func HandleCommands(ctx context.Context, bus command.Bus, repo aggregate.Repository) (<-chan error, error) {
	h := command.NewHandler(bus)

	fillErrors, err := h.Handle(ctx, FillCommand, func(ctx command.Context) error {
		cmd := ctx.Command()
		load := cmd.Payload().(FillPayload)
		s := New(cmd.AggregateID())

		if err := repo.Fetch(ctx, s); err != nil {
			return fmt.Errorf("fetch Stock: %w", err)
		}

		if err := s.Fill(load.Quantity); err != nil {
			return fmt.Errorf("fill Stock: %w", err)
		}

		if err := repo.Save(ctx, s); err != nil {
			return fmt.Errorf("save Stock: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("register %q Command handler: %w", FillCommand, err)
	}

	reduceErrors, err := h.Handle(ctx, ReduceCommand, func(ctx command.Context) error {
		cmd := ctx.Command()
		load := cmd.Payload().(ReducePayload)
		s := New(cmd.AggregateID())

		if err := repo.Fetch(ctx, s); err != nil {
			return fmt.Errorf("fetch Stock: %w", err)
		}

		if err := s.Reduce(load.Quantity); err != nil {
			return err
		}

		if err := repo.Save(ctx, s); err != nil {
			return fmt.Errorf("save Stock: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("register %q Command handler: %w", ReduceCommand, err)
	}

	reserveErrors, err := h.Handle(
		ctx, ReserveCommand,
		func(ctx command.Context) error {
			cmd := ctx.Command()
			load := cmd.Payload().(ReservePayload)
			s := New(cmd.AggregateID())

			if err := repo.Fetch(ctx, s); err != nil {
				return fmt.Errorf("fetch Stock: %w", err)
			}

			if err := s.Reserve(load.Quantity, load.OrderID); err != nil {
				return fmt.Errorf("reserve Stock: %w", err)
			}

			if err := repo.Save(ctx, s); err != nil {
				return fmt.Errorf("save Stock: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("register %q Command handler: %w", ReserveCommand, err)
	}

	releaseErrors, err := h.Handle(ctx, ReleaseCommand, func(ctx command.Context) error {
		cmd := ctx.Command()
		load := cmd.Payload().(ReleasePayload)
		s := New(cmd.AggregateID())

		if err := repo.Fetch(ctx, s); err != nil {
			return fmt.Errorf("fetch Stock: %w", err)
		}

		if err := s.Release(load.OrderID); err != nil {
			return err
		}

		if err := repo.Save(ctx, s); err != nil {
			return fmt.Errorf("save Stock: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("register %q Command handler: %w", ReserveCommand, err)
	}

	executeErrors, err := h.Handle(
		ctx, ExecuteCommand,
		func(ctx command.Context) error {
			cmd := ctx.Command()
			load := cmd.Payload().(ExecutePayload)
			s := New(cmd.AggregateID())

			if err := repo.Fetch(ctx, s); err != nil {
				return fmt.Errorf("fetch Stock: %w", err)
			}

			if err := s.Execute(load.OrderID); err != nil {
				return err
			}

			if err := repo.Save(ctx, s); err != nil {
				return fmt.Errorf("save Stock: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("register %q Command handler: %w", ExecuteCommand, err)
	}

	out, stop := fanin.Errors(fillErrors, reduceErrors, reserveErrors, releaseErrors, executeErrors)
	go func() {
		<-ctx.Done()
		stop()
	}()

	return out, nil
}
