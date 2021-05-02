package product

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
)

const (
	// CreateCommand is the Command name for creating a Product.
	CreateCommand = "product.create"
)

// CreatePayload is the Payload for creating a Product.
type CreatePayload struct {
	Name      string
	UnitPrice int
}

// Create returns the Command for creating a Product.
func Create(id uuid.UUID, name string, unitPrice int) command.Command {
	return command.New(CreateCommand, CreatePayload{
		Name:      name,
		UnitPrice: unitPrice,
	}, command.Aggregate(AggregateName, id))
}

// RegisterCommands registers the Product Commands into a Command Registry.
func RegisterCommands(r command.Registry) {
	r.Register(CreateCommand, func() command.Payload { return CreatePayload{} })
}

// HandleCommands starts handling Product Commands in the background and returns
// a channel of asynchronous errors that is closed when ctx is canceled.
func HandleCommands(ctx context.Context, bus command.Bus, repo aggregate.Repository) (<-chan error, error) {
	h := command.NewHandler(bus)

	errs, err := h.Handle(
		ctx, CreateCommand,
		func(ctx command.Context) error {
			cmd := ctx.Command()
			load := cmd.Payload().(CreatePayload)
			p := New(cmd.AggregateID())

			if err := repo.Fetch(ctx, p); err != nil {
				return fmt.Errorf("fetch Product: %w", err)
			}

			if err := p.Create(load.Name, load.UnitPrice); err != nil {
				return err
			}

			if err := repo.Save(ctx, p); err != nil {
				return fmt.Errorf("save Product: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("register %q Command handler: %w", CreateCommand, err)
	}

	return errs, nil
}
