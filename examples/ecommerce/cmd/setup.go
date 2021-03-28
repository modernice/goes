package cmd

import (
	"context"
	"ecommerce/order"
	"ecommerce/product"
	"ecommerce/stock"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/command/encoding"
	"github.com/modernice/goes/event"
	eventenc "github.com/modernice/goes/event/encoding"
	"github.com/modernice/goes/event/eventbus/natsbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/eventstore/mongostore"
	"github.com/nats-io/stan.go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-exit
		cancel()
	}()
	return ctx
}

func NewEventRegistry() event.Registry {
	r := eventenc.NewGobEncoder()
	RegisterEvents(r)
	return r
}

var clientIDRE = regexp.MustCompile("(?i)[^a-z0-9]")

func NewEventBus(enc event.Encoder, serviceName string) *natsbus.Bus {
	id := [16]byte(uuid.New())
	b64 := base64.StdEncoding.EncodeToString(id[:])
	nb64 := clientIDRE.ReplaceAllString(b64, "")
	clientID := fmt.Sprintf("%s_%s", serviceName, nb64)
	return natsbus.New(
		enc,
		natsbus.Use(natsbus.Streaming("test-cluster", clientID, stan.ConnectWait(10*time.Second))),
		natsbus.Durable(),
		natsbus.QueueGroupByFunc(func(eventName string) string {
			return fmt.Sprintf("%s_%s", serviceName, eventName)
		}),
	)
}

func NewEventStore(enc event.Encoder, bus event.Bus) event.Store {
	return eventstore.WithBus(mongostore.New(enc, mongostore.Transactions(false)), bus)
}

func NewCommandBus(events event.Bus) command.Bus {
	r := NewCommandRegistry()
	return cmdbus.New(r, events)
}

func NewCommandRegistry() command.Registry {
	r := encoding.NewGobEncoder()
	RegisterCommands(r)
	return r
}

func NewRepository(store event.Store) aggregate.Repository {
	return repository.New(store)
}

func RegisterCommands(r command.Registry) {
	stock.RegisterCommands(r)
	order.RegisterCommands(r)
	product.RegisterCommands(r)
}

func RegisterEvents(r event.Registry) {
	cmdbus.RegisterEvents(r)
	stock.RegisterEvents(r)
	order.RegisterEvents(r)
	product.RegisterEvents(r)
}

func InitMongo(ctx context.Context, serviceName string) (*mongo.Client, *mongo.Database, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URL")))
	if err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}
	if err = client.Ping(ctx, nil); err != nil {
		return client, nil, fmt.Errorf("ping: %w", err)
	}
	return client, client.Database(serviceName), nil
}
