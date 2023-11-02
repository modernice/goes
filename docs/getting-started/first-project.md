# Setting Up Your First Project

::: info
**goes** requires **Go 1.21** or higher.
:::

goes can be integrated into any Go project. The following steps will guide you
through setting up the core components of goes in a new project.

## Add goes to your project

```bash
go get -u github.com/modernice/goes
```

## Setup the components 

In your main.go (or wherever you setup your application), create an event store and event bus:

```go
package main

import (
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
)

var (
	eventStore = eventstore.New() // creates an in-memory event store
	eventBus = eventbus.New() // creates an in-memory event bus
)

func main() {}
```
