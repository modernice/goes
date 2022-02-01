package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/command/cmdbus/dispatch"
	"github.com/modernice/goes/examples/todo"
	"github.com/modernice/goes/examples/todo/cmd"
)

func main() {
	var setup cmd.Setup

	ctx, cancel := setup.Context()
	defer cancel()

	ebus, _, ereg, disconnect := setup.Events(ctx)
	defer disconnect()

	cbus, _ := setup.Commands(ereg.Registry, ebus)

	// Wait a bit to ensure that the todo server is running before dispatching commands.
	<-time.After(3 * time.Second)

	// Create a new todo list and add some tasks.
	listID := uuid.New()
	for i := 0; i < 10; i++ {
		cmd := todo.AddTask(listID, fmt.Sprintf("Task %d", i+1))
		if err := cbus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
			log.Panicf("Failed to dispatch command: %v [cmd=%v, task=%q]", err, cmd.Name(), cmd.Payload())
		}
	}

	// Then remove every second task.
	for i := 0; i < 10; i += 2 {
		cmd := todo.RemoveTask(listID, fmt.Sprintf("Task %d", i+1))
		if err := cbus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
			log.Panicf("Failed to dispatch command: %v [cmd=%v, task=%q]", err, cmd.Name(), cmd.Payload())
		}
	}

	// Remaining tasks: Task 2, Task 4, Task 6, Task 8, Task 10

	// Then mark "Task 6" and "Task 10" as done.
	cmd := todo.DoneTasks(listID, "Task 6", "Task 10")
	if err := cbus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
		log.Panicf("Failed to dispatch command: %v [cmd=%v, tasks=%v]", err, cmd.Name(), cmd.Payload())
	}

	// Give the "server" service time to run projections before Docker stops all services.
	<-time.After(3 * time.Second)
}
