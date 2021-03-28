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
	var reserved []Item
	return saga.New(
		saga.Action("ReserveStock", func(ctx action.Context) error {
			// Reserve each Item.
			for _, item := range items {
				cmd := stock.Reserve(item.ProductID, orderID, item.Quantity)
				if err := ctx.Dispatch(ctx, cmd); err != nil {
					return err
				}

				// Mark Item as reserved (needed for rollback)
				reserved = append(reserved, item)
			}
			return nil
		}),

		saga.Action("ReleaseStock", func(ctx action.Context) error {
			// SAGA failed -> release reserved Items.
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
				// Fetch the Product to get its name.
				p := product.New(item.ProductID)
				if err := ctx.Fetch(ctx, p); err != nil {
					return fmt.Errorf("fetch product: %w", err)
				}

				oitems[i] = order.Item{
					ProductID: p.AggregateID(),
					Name:      p.Name(),
					Quantity:  item.Quantity,
					UnitPrice: p.UnitPrice(),
				}
			}

			// Dispatch the "order.place" Command.
			if err := ctx.Dispatch(ctx, order.Place(orderID, cus, oitems...)); err != nil {
				return err
			}
			return nil
		}),

		saga.Action("CancelOrder", func(ctx action.Context) error {
			return ctx.Dispatch(ctx, order.Cancel(orderID))
		}),

		saga.Sequence("ReserveStock", "PlaceOrder"),
		saga.Compensate("ReserveStock", "ReleaseStock"),
		saga.Compensate("PlaceOrder", "CancelOrder"),
	)
}
