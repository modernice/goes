package todo

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

const ListAggregate = "todo.list"

type List struct {
	*aggregate.Base

	tasks   []string
	archive []string
}

func New(id uuid.UUID) *List {
	return &List{
		Base: aggregate.New(ListAggregate, id),
	}
}

func (list *List) Add(task string) error {
	for _, t := range list.tasks {
		if strings.ToLower(t) == strings.ToLower(task) {
			return errors.New("task already exists")
		}
	}

	aggregate.NextEvent(list, TaskAdded, TaskAddedEvent{task})

	return nil
}

func (list *List) add(evt event.EventOf[TaskAddedEvent]) {
	list.tasks = append(list.tasks, evt.Data().Task)
}

func (list *List) Remove(task string) error {
	ltask := strings.ToLower(task)
	for _, t := range list.tasks {
		if strings.ToLower(t) == ltask {
			aggregate.NextEvent(list, TaskRemoved, TaskRemovedEvent{task})
			return nil
		}
	}
	return errors.New("task not found")
}

func (list *List) remove(evt event.EventOf[TaskRemovedEvent]) {
	for i, task := range list.tasks {
		if strings.ToLower(task) == strings.ToLower(evt.Data().Task) {
			list.tasks = append(list.tasks[:i], list.tasks[i+1:]...)
			return
		}
	}
}

func (list *List) Done(task string) error {
	ltask := strings.ToLower(task)
	for _, t := range list.tasks {
		if strings.ToLower(t) == ltask {
			aggregate.NextEvent[any](list, TaskDone, TaskDoneEvent{task})
			return nil
		}
	}
	return errors.New("task not found")
}

func (list *List) done(evt event.EventOf[TaskDoneEvent]) {
	for i, t := range list.tasks {
		task := evt.Data().Task
		if strings.ToLower(t) == strings.ToLower(task) {
			list.tasks = append(list.tasks[:i], list.tasks[i+1:]...)
			list.archive = append(list.archive, task)
			return
		}
	}
}

func (list *List) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case TaskAdded:
		list.add(event.Cast[TaskAddedEvent](evt))
	case TaskRemoved:
		list.remove(event.Cast[TaskRemovedEvent](evt))
	case TaskDone:
		list.done(event.Cast[TaskDoneEvent](evt))
	}
}
