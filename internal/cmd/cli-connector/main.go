package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/modernice/goes/cli"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventstore/memstore"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

var port = flag.Int("port", int(cli.DefaultPort), "Connector port")

func main() {
	flag.Parse()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-shutdown
		cancel()
	}()

	bus := chanbus.New()
	store := memstore.New()

	foo := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})
	bar := schedule.Periodically(store, 30*time.Second, []string{"foo", "bar", "baz"})

	fooErrors, err := foo.Subscribe(ctx, func(projection.Job) error {
		log.Printf("%q schedule received job", "foo")
		<-time.After(time.Second)
		return nil
	})
	if err != nil {
		log.Fatalf("subscribe to %q schedule: %v", "foo", err)
	}
	log.Printf("Subscribed to %q schedule.\n", "foo")

	barErrors, err := bar.Subscribe(ctx, func(projection.Job) error {
		log.Printf("%q schedule received job", "bar")
		<-time.After(time.Second)
		return nil
	})
	if err != nil {
		log.Fatalf("subscribe to %q schedule: %v", "bar", err)
	}
	log.Printf("Subscribed to %q schedule.\n", "bar")

	svc := projection.NewService(
		bus,
		projection.RegisterSchedule("foo", foo),
		projection.RegisterSchedule("bar", bar),
	)

	serviceErrors, err := svc.Run(ctx)
	if err != nil {
		log.Fatalf("run projection service: %v", err)
	}

	connector := cli.NewConnector(svc)
	serveError := make(chan error)

	go func() {
		defer close(serveError)
		if err := connector.Serve(ctx, cli.Port(uint16(*port))); err != nil {
			serveError <- fmt.Errorf("serve connector: %w", err)
		}
	}()

	log.Printf(aurora.Blue("Serving CLI Connector on port %d. Logging errors...\n\n").String(), *port)

	<-logErrors(ctx, fooErrors, barErrors, serviceErrors, serveError)

	log.Println("Shutting down...")

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			log.Fatalf("shutdown: %v", ctx.Err())
		case <-shutdown:
			cancel()
		case err, ok := <-serveError:
			if ok {
				log.Fatal(err)
			}
			return
		}
	}
}

func logErrors(ctx context.Context, in ...<-chan error) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		errs := fanIn(ctx, in...)
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errs:
				if !ok {
					return
				}
				log.Println(aurora.Red(err))
			}
		}
	}()
	return done
}

func fanIn(ctx context.Context, in ...<-chan error) <-chan error {
	out := make(chan error)
	var wg sync.WaitGroup
	wg.Add(len(in))
	for _, errs := range in {
		go func(errs <-chan error) {
			defer wg.Done()
			for err := range errs {
				for {
					select {
					case <-ctx.Done():
						return
					case out <- err:
					}
				}
			}
		}(errs)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
