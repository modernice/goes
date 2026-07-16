package schedule_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

func TestSubscribe_StartupAsync(t *testing.T) {
	tests := map[string]projection.Schedule{
		"continuous": schedule.Continuously(eventbus.New(), eventstore.New(), []string{"foo"}),
		"periodic":   schedule.Periodically(eventstore.New(), time.Hour, []string{"foo"}),
	}

	for name, sch := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			startupStarted := make(chan struct{})
			releaseStartup := make(chan struct{})
			defer func() {
				select {
				case <-releaseStartup:
				default:
					close(releaseStartup)
				}
			}()

			startupErr := errors.New("startup failed")
			type subscribeResult struct {
				errs <-chan error
				err  error
			}
			subscribed := make(chan subscribeResult, 1)

			go func() {
				errs, err := sch.Subscribe(ctx, func(projection.Job) error {
					close(startupStarted)
					<-releaseStartup
					return startupErr
				}, projection.StartupAsync())
				subscribed <- subscribeResult{errs: errs, err: err}
			}()

			var result subscribeResult
			select {
			case result = <-subscribed:
			case <-time.After(time.Second):
				t.Fatal("Subscribe blocked on the asynchronous startup job")
			}
			if result.err != nil {
				t.Fatalf("Subscribe failed: %v", result.err)
			}

			select {
			case <-startupStarted:
			case <-time.After(time.Second):
				t.Fatal("asynchronous startup job was not applied")
			}
			close(releaseStartup)

			select {
			case err := <-result.errs:
				if !errors.Is(err, startupErr) {
					t.Fatalf("expected startup error; got %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("startup error was not reported asynchronously")
			}
		})
	}
}
