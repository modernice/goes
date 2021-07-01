package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/modernice/goes/cli/internal/clifactory"
)

// Main is the entrypoint for the CLI. Call Main from an actual main function.
func Main() {
	log.SetFlags(0)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := New(clifactory.Context(ctx), clifactory.ConnectTimeout(time.Second))

	err := app.Run()
	if err != nil {
		if errors.Is(err, clifactory.ErrConnectorUnavailable) {
			addr := app.Factory().ConnectorAddress
			if strings.HasPrefix(addr, ":") {
				addr = fmt.Sprintf("localhost%s", addr)
			}
			log.Fatalf(
				aurora.Red("Unable to connect to your application. Did you setup the CLI Connector at %s?").String(),
				addr,
			)
		}
		log.Fatal(aurora.Red(err))
	}
}
