package product

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
)

const (
	CreateCommand = "product.create"
)

type CreatePayload struct {
	Name      string
	UnitPrice int
}

func Create(id uuid.UUID, name string, unitPrice int) command.Command {
	return command.New(CreateCommand, CreatePayload{
		Name:      name,
		UnitPrice: unitPrice,
	}, command.Aggregate(AggregateName, id))
}

func RegisterCommands(r command.Registry) {
	r.Register(CreateCommand, func() command.Payload { return CreatePayload{} })
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
	); err != nil {
		cancel()
		return nil, fmt.Errorf("handle %q Commands: %w", CreateCommand, err)
	}

	return h.Errors(ctx), nil
}
