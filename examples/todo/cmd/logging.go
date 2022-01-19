package cmd

import (
	"context"
	"log"

	"github.com/modernice/goes/helper/streams"
)

// LogErrors logs all errors from the provided channels.
func LogErrors(ctx context.Context, errs ...<-chan error) {
	log.Printf("Logging errors ...")

	in := streams.FanInContext(ctx, errs...)
	for err := range in {
		log.Println(err)
	}
}
