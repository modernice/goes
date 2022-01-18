package todo_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/examples/todo"
	"github.com/modernice/goes/test"
)

func TestNew(t *testing.T) {
	test.NewAggregate(t, todo.New, todo.ListAggregate)
}

func TestList_Add(t *testing.T) {
	list := todo.New(uuid.New())

	if err := list.Add("foo"); err != nil {
		t.Fatalf("Add(%q) failed with %q", "foo", err)
	}

	if !list.Contains("foo") {
		t.Fatalf("List should contain the task %q after adding", "foo")
	}

	test.Change(t, list, todo.TaskAdded, test.EventData(todo.TaskAddedEvent{
		Task: "foo",
	}))
}

func TestList_Add_alreadyExists(t *testing.T) {
	list := todo.New(uuid.New())

	list.Add("foo")
	if err := list.Add("foo"); err != nil {
		t.Fatalf("Add() shouldn't fail when provided an existing task; got %q", err)
	}

	if !list.Contains("foo") {
		t.Fatalf("List should contain the task %q", "foo")
	}

	var count int
	for _, task := range list.Tasks() {
		if task == "foo" {
			count++
		}
	}

	if count != 1 {
		t.Fatalf("Tasks should be unique")
	}

	test.Change(t, list, todo.TaskAdded, test.EventData(todo.TaskAddedEvent{
		Task: "foo",
	}), test.Exactly[todo.TaskAddedEvent](1))
}

func TestList_Remove(t *testing.T) {
	list := todo.New(uuid.New())

	list.Add("foo")

	if err := list.Remove("foo"); err != nil {
		t.Fatalf("Remove(%q) failed with %q", "foo", err)
	}

	if list.Contains("foo") {
		t.Fatalf("List shouldn't contain the task %q after removing", "foo")
	}

	test.Change(t, list, todo.TaskRemoved, test.EventData(todo.TaskRemovedEvent{
		Task: "foo",
	}))
}

func TestList_Done(t *testing.T) {
	list := todo.New(uuid.New())

	list.Add("foo")

	if err := list.Done("foo"); err != nil {
		t.Fatalf("Done(%q) failed with %q", "foo", err)
	}

	if list.Contains("foo") {
		t.Fatalf("List shouldn't contain the task %q after marking as done", "foo")
	}

	test.Change(t, list, todo.TaskDone, test.EventData(todo.TaskDoneEvent{
		Task: "foo",
	}))
}
