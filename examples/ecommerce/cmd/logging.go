package cmd

import (
	"fmt"
	"log"
	"sync"

	"github.com/logrusorgru/aurora"
)

// LogErrors logs errors from the provided error channels.
func LogErrors(errs ...<-chan error) <-chan struct{} {
	done := make(chan struct{})
	if len(errs) == 0 {
		close(done)
		return done
	}

	var wg sync.WaitGroup
	wg.Add(len(errs))
	for _, errs := range errs {
		errs := errs
		go func() {
			defer wg.Done()
			for err := range errs {
				log.Println(aurora.Red(fmt.Errorf("[command]: %w", err)))
			}
		}()
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	return done
}
