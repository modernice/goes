package main

import (
	"flag"

	"github.com/google/uuid"
	"github.com/modernice/goes/cli"
)

var port = flag.Int("port", int(cli.DefaultPort), "Connector port")

func main() {
	flag.Parse()
	cli.ConnectorMain(uuid.New, uint16(*port))
}
