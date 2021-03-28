package cli

import (
	"context"
	"ecommerce/cmd"
	"ecommerce/order"
	"ecommerce/product"
	"ecommerce/saga/placeorder"
	"ecommerce/stock"
	"fmt"
	"log"

	"github.com/AlecAivazis/survey/v2"
	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/logrusorgru/aurora"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/saga"
)

var cli struct {
	Product productCmd `cmd:"" help:"Manage Products."`
	Order   orderCmd   `cmd:"" help:"Manage Orders."`
	Stock   stockCmd   `cmd:"" help:"Manage Stock."`
}

type productCmd struct {
	Create productCreateCmd `cmd:"" help:"Create a Product."`
}

type productCreateCmd struct {
	Name  string `help:"Product name."`
	Price int    `help:"Unit price in cents."`
}

type orderCmd struct {
	Place    orderPlaceCmd    `cmd:"" help:"Place an Order."`
	Timeline orderTimelineCmd `cmd:"" help:"Display the Timeline of an Order."`
}

type orderPlaceCmd struct{}

type orderTimelineCmd struct{}

type stockCmd struct {
	Fill stockFillCmd `cmd:"" help:"Fill a Stock."`
}

type stockFillCmd struct {
	ProductID string `arg:""`
	Amount    int    `arg:""`
}

func New(
	events event.Bus,
	commands command.Bus,
	aggregates aggregate.Repository,
	timelines order.TimelineRepository,
) *kong.Context {
	ctx := cmd.NewContext()
	exec := saga.NewExecutor(
		saga.CommandBus(commands),
		saga.Repository(aggregates),
		saga.EventBus(events),
	)
	return kong.Parse(
		&cli,
		kong.BindTo(ctx, (*context.Context)(nil)),
		kong.BindTo(events, (*event.Bus)(nil)),
		kong.BindTo(commands, (*command.Bus)(nil)),
		kong.BindTo(aggregates, (*aggregate.Repository)(nil)),
		kong.BindTo(timelines, (*order.TimelineRepository)(nil)),
		kong.Bind(exec),
	)
}

func (create *productCreateCmd) Run(ctx context.Context, commands command.Bus, aggregates aggregate.Repository) error {
	id := uuid.New()
	cmd := command.New(
		product.CreateCommand,
		product.CreatePayload{
			Name:      create.Name,
			UnitPrice: create.Price,
		},
		command.Aggregate(product.AggregateName, id),
	)
	if err := commands.Dispatch(ctx, cmd, dispatch.Synchronous()); err != nil {
		return err
	}
	p := product.New(id)
	if err := aggregates.Fetch(ctx, p); err != nil {
		return fmt.Errorf("fetch Product: %w", err)
	}
	log.Println(aurora.Green(aurora.Bold("Product created.")))
	log.Println(fmt.Sprintf("ID: %s", p.AggregateID()))
	log.Println(fmt.Sprintf("Name: %s", p.Name()))
	log.Println(fmt.Sprintf("Price: %d cents", p.UnitPrice()))
	return nil
}

func (place *orderPlaceCmd) Run(
	ctx context.Context,
	commands command.Bus,
	aggregates aggregate.Repository,
	exec *saga.Executor,
) error {
	str, errs, err := aggregates.Query(ctx, query.New(query.Name(product.AggregateName)))
	if err != nil {
		return fmt.Errorf("query Products: %w", err)
	}

	histories, err := aggregate.Drain(ctx, str, errs)
	if err != nil {
		return fmt.Errorf("drain Histories: %w", err)
	}

	products := make(map[string]*product.Product)
	names := make([]string, len(histories))
	for i, h := range histories {
		p := product.New(h.AggregateID())
		if err := aggregates.Fetch(ctx, p); err != nil {
			return fmt.Errorf("fetch Product: %w", err)
		}
		name := fmt.Sprintf("%s (%d cents)", p.Name(), p.UnitPrice())
		products[name] = p
		names[i] = name
	}

	var orderItems []placeorder.Item
	for {
		var selected string
		if err := survey.AskOne(&survey.Select{
			Message: "Select a Product.",
			Options: names,
		}, &selected); err != nil {
			return err
		}

		p := products[selected]
		s := stock.New(p.AggregateID())
		if err := aggregates.Fetch(ctx, s); err != nil {
			return fmt.Errorf("fetch Stock: %w", err)
		}

		var quantity int
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Select a quantity (%d available).", s.Available()),
			Default: "1",
		}, &quantity); err != nil {
			return err
		}

		orderItems = append(orderItems, placeorder.Item{
			ProductID: p.AggregateID(),
			Quantity:  quantity,
		})

		var yesNo string
		if err := survey.AskOne(&survey.Select{
			Message: "Add more Products?",
			Options: []string{"Yes", "No"},
		}, &yesNo); err != nil {
			return err
		}

		if yesNo == "No" {
			break
		}
	}

	var cus order.Customer
	if err := survey.Ask([]*survey.Question{
		{Name: "name", Prompt: &survey.Input{Message: "What't your name?"}},
		{Name: "email", Prompt: &survey.Input{Message: "What't your email address?"}},
	}, &cus); err != nil {
		return err
	}

	id := uuid.New()
	setup := placeorder.Setup(id, cus, orderItems...)
	if err := exec.Execute(ctx, setup); err != nil {
		return err
	}

	o := order.New(id)
	if err := aggregates.Fetch(ctx, o); err != nil {
		return fmt.Errorf("fetch Order: %w", err)
	}

	log.Println(aurora.Green(aurora.Bold("Order placed.")))
	log.Println(fmt.Sprintf("ID: %s", o.AggregateID()))
	log.Println(fmt.Sprintf("Customer: %v", o.Customer()))
	log.Println(fmt.Sprintf("Items: %v", o.Items()))
	log.Println(fmt.Sprintf("Total: %d cents", o.Total()))

	return nil
}

func (fill *stockFillCmd) Run(ctx context.Context, commands command.Bus) error {
	id, err := uuid.Parse(fill.ProductID)
	if err != nil {
		return err
	}
	cmd := stock.Fill(id, fill.Amount)
	return commands.Dispatch(ctx, cmd, dispatch.Synchronous())
}

func (*orderTimelineCmd) Run(ctx context.Context, aggregates aggregate.Repository, timelines order.TimelineRepository) error {
	str, errs, err := aggregates.Query(ctx, query.New(query.Name(order.AggregateName)))
	if err != nil {
		return fmt.Errorf("query Orders: %w", err)
	}

	histories, err := aggregate.Drain(ctx, str, errs)
	if err != nil {
		return fmt.Errorf("drain Histories: %w", err)
	}

	orders := make([]*order.Order, len(histories))
	options := make([]string, len(histories))
	optionMap := make(map[string]*order.Order)
	for i, his := range histories {
		o := order.New(his.AggregateID())
		his.Apply(o)
		orders[i] = o
		options[i] = fmt.Sprintf("%s (%s)", o.AggregateID(), o.Customer().Name)
		optionMap[options[i]] = o
	}

	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: "Choose an Order.",
		Options: options,
	}, &selected); err != nil {
		return err
	}

	o := optionMap[selected]
	tl, err := timelines.Fetch(ctx, o.AggregateID())
	if err != nil {
		return fmt.Errorf("fetch Timeline: %w", err)
	}

	for i, step := range tl.Steps {
		log.Println(aurora.Yellow(fmt.Sprintf("[%d] %s", i+1, step.Desc)))
	}

	return nil
}
