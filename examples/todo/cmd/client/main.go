package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/examples/todo"
	"github.com/modernice/goes/examples/todo/cmd"
)

func main() {
	var setup cmd.Setup

	ctx, cancel := setup.Context()
	defer cancel()

	ebus, _, ereg, disconnect := setup.Events(ctx, "client")
	defer disconnect()

	cbus, _ := setup.Commands(ereg, ebus)

	// Wait a bit to ensure that the todo server is running before dispatching commands.
	<-time.After(3 * time.Second)

	// Create a new todo list and add some tasks.
	listID := uuid.New()
	for i := 0; i < 10; i++ {
		sleepRandom()

		cmd := todo.AddTask(listID, fmt.Sprintf("Task %d", i+1))
		if err := cbus.Dispatch(ctx, cmd.Any()); err != nil {
			log.Panicf("Failed to dispatch command: %v [cmd=%v, task=%q]", err, cmd.Name(), cmd.Payload())
		}
	}

	// Then remove every second task.
	for i := 0; i < 10; i += 2 {
		sleepRandom()

		cmd := todo.RemoveTask(listID, fmt.Sprintf("Task %d", i+1))
		if err := cbus.Dispatch(ctx, cmd.Any()); err != nil {
			log.Panicf("Failed to dispatch command: %v [cmd=%v, task=%q]", err, cmd.Name(), cmd.Payload())
		}
	}

	// Remaining tasks: Task 2, Task 4, Task 6, Task 8, Task 10

	// Then mark "Task 6" and "Task 10" as done.
	sleepRandom()

	cmd := todo.DoneTasks(listID, "Task 6", "Task 10")
	if err := cbus.Dispatch(ctx, cmd.Any()); err != nil {
		log.Panicf("Failed to dispatch command: %v [cmd=%v, tasks=%v]", err, cmd.Name(), cmd.Payload())
	}
}

func sleepRandom() {
	dur := time.Duration(rand.Intn(1000)) * time.Millisecond
	log.Printf("Waiting %s before dispatching next command ...", dur)
	time.Sleep(dur)
}
