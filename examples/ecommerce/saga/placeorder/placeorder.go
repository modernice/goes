package placeorder

import (
	"ecommerce/order"
	"ecommerce/product"
	"ecommerce/stock"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/saga"
	"github.com/modernice/goes/saga/action"
)

// Item is an item of an order.
type Item struct {
	ProductID uuid.UUID
	Quantity  int
}

// Setup returns the SAGA Setup for placing an Order.
func Setup(orderID uuid.UUID, cus order.Customer, items ...Item) saga.Setup {
	// reserved holds the successfully reserved Items of the Order.
	// reserved is needed by the "ReleaseStock" Action to compensate the
	// "ReserveStock" Action.
	var reserved []Item

	// executed holds the successfully executed Stock reservations for the Order.
	// executed is needed by the "RefillStock" Action to compensate the
	// "ExecuteStock" Action.
	var executed []Item

	return saga.New(
		saga.Action("ReserveStock", func(ctx action.Context) error {
			for _, item := range items {
				// Create the Command to reserve the Item.
				cmd := stock.Reserve(item.ProductID, orderID, item.Quantity)

				// Here we dispatch the Command directly through the
				// action.Context. This only works when the SAGA Executor that
				// executes this SAGA was provided with a Command Bus. Otherwise
				// Dispatch would return action.ErrMissingBus.
				//
				// A dispatch through an action.Context is always made
				// synchronous so that the SAGA knows when it needs to
				// compensate a failed Action.
				if err := ctx.Dispatch(ctx, cmd); err != nil {
					// Manually run the compensating Action for this Action here,
					// because the "ReleaseStock" Action will not be executed
					// when this Action returns an error (compensating Actions
					// are only executed to compensate succeeded Actions)
					if err := ctx.Run(ctx, "ReleaseStock"); err != nil {
						return fmt.Errorf("release Stock after failed dispatch: %w", err)
					}

					// When this error is returned, the SAGA will be marked as
					// failed and the compensating Actions for the *previously
					// succeeded* Actions will be executed, which in this case
					// would be none, because "ReserveStock" is the first Action
					// of the SAGA. For this reason we manually call the
					// "ReleaseStock" Action above when this Action fails to
					// ensure that the Stock is released when this Action fails.
					return fmt.Errorf("dispatch %q Command for Product %s: %w", cmd.Name(), item.ProductID, err)
				}

				// We add every successfully reserved Items to the `reserved`
				// Items, so that we know for which Products we need to release
				// the Stock should the SAGA fail.
				reserved = append(reserved, item)
			}

			return nil
		}),

		saga.Action("ReleaseStock", func(ctx action.Context) error {
			for _, item := range reserved {
				cmd := stock.Release(item.ProductID, orderID)
				if err := ctx.Dispatch(ctx, cmd); err != nil {
					return err
				}
			}
			return nil
		}),

		saga.Action("PlaceOrder", func(ctx action.Context) error {
			oitems := make([]order.Item, len(items))
			for i, item := range items {
				// For every Item fetch the Product to get its name and price.
				p := product.New(item.ProductID)

				// Here we use the Fetch helper on the action.Context to fetch
				// the Product Aggregate from the Aggregate Repository. Note
				// that this only works when the SAGA Executor that executes this
				// SAGA was provided with an Aggregate Repository. Otherwise
				// Fetch would return action.ErrMissingRepository.
				if err := ctx.Fetch(ctx, p); err != nil {
					return fmt.Errorf("fetch product: %w", err)
				}

				// Constructs the actual order.Items for the Order service.
				oitems[i] = order.Item{
					ProductID: p.AggregateID(),
					Name:      p.Name(),
					Quantity:  item.Quantity,
					UnitPrice: p.UnitPrice(),
				}
			}

			cmd := order.Place(orderID, cus, oitems...)
			if err := ctx.Dispatch(ctx, cmd); err != nil {
				return fmt.Errorf("dispatch %q Command: %w", cmd.Name(), err)
			}

			return nil
		}),

		saga.Action("CancelOrder", func(ctx action.Context) error {
			return ctx.Dispatch(ctx, order.Cancel(orderID))
		}),

		saga.Action("ExecuteStock", func(ctx action.Context) error {
			// After the Order is placed, it's time to execute the Stock for the
			// Products of the Order so that the reserved stock is marked as
			// "claimed" and fully removed from the Stocks quantity.
			for _, item := range reserved {
				cmd := stock.Execute(item.ProductID, orderID)
				if err := ctx.Dispatch(ctx, cmd); err != nil {
					return fmt.Errorf("dispatch %q Command for Product %s: %w", cmd.Name(), item.ProductID, err)
				}
				executed = append(executed, item)
			}
			return nil
		}),

		saga.Action("RefillStock", func(ctx action.Context) error {
			// This Action could never actually run, because it is the
			// compensating Action for the "ExecuteStock" Action and that Action
			// is the last Action in the sequence of this SAGA. Compensating
			// Actions are only run for succeeded Actions, so the "RefillStock"
			// Action could only be run if the "ExecuteStock" Action succeeded
			// and a following Action of the SAGA failed.
			//
			// So why define this Action then? It definitely doesn't hurt having
			// this compensating Action already in place should the SAGA be
			// extended with more Actions in the future so that "ExecuteStock"
			// isn't the last Action in the sequence anymore. Other Actions
			// could also theoretically call this Action directly through their
			// action.Context.
			for _, item := range executed {
				cmd := stock.Fill(item.ProductID, item.Quantity)
				if err := ctx.Dispatch(ctx, cmd); err != nil {
					return fmt.Errorf("dispatch %q Command for Product %s: %w", cmd.Name(), item.ProductID, err)
				}
			}
			return nil
		}),

		// Here we configure the flow of the SAGA.

		// saga.Sequence defines the Actions that should be run sequentially if
		// no Action fails. We want to "reserve the stock", then "place the
		// order" and finally "execute/claim the previously reserved stock".
		saga.Sequence("ReserveStock", "PlaceOrder", "ExecuteStock"),

		// saga.Compensate defines the compensating Action for a failed Action.
		// We want to "release stock" for products we "reserved stock" for.
		saga.Compensate("ReserveStock", "ReleaseStock"),

		// You should get this one.
		saga.Compensate("PlaceOrder", "CancelOrder"),

		// When "stock is executed", it quantity is removed from the Stock of a
		// Product. We need to "refill the stock" for these Products when the
		// SAGA fails after Stock execution.
		saga.Compensate("ExecuteStock", "RefillStock"),
	)
}
