package todo

// const (
// 	AddTaskCmd    = "todo.list.add_task"
// 	RemoveTaskCmd = "todo.list.remove_task"
// 	DoneTaskCmd   = "todo.list.done_task"
// )

// type addTaskPayload struct {
// 	Task string
// }

// func AddTask(listID uuid.UUID, task string) command.Cmd[addTaskPayload] {
// 	return command.New(AddTaskCmd, addTaskPayload{task}, command.Aggregate[addTaskPayload](listID, ListAggregate))
// }

// type removeTaskPayload struct {
// 	Task string
// }

// func RemoveTask(listID uuid.UUID, task string) command.Cmd[removeTaskPayload] {
// 	return command.New(RemoveTaskCmd, removeTaskPayload{task}, command.Aggregate[removeTaskPayload](listID, ListAggregate))
// }

// type doneTaskPayload struct {
// 	Task string
// }

// func DoneTask(listID uuid.UUID, task string) command.Cmd[doneTaskPayload] {
// 	return command.New(DoneTaskCmd, doneTaskPayload{task}, command.Aggregate[doneTaskPayload](listID, ListAggregate))
// }

// func RegisterCommands(r *codec.GobRegistry) {
// 	r.GobRegister(AddTaskCmd, func() any { return addTaskPayload{} })
// 	r.GobRegister(RemoveTaskCmd, func() any { return removeTaskPayload{} })
// 	r.GobRegister(DoneTaskCmd, func() any { return doneTaskPayload{} })
// }

// func HandleCommands(ctx context.Context, bus command.Bus, repo aggregate.Repository) <-chan error {
// 	// addErrors := command.MustHandle(ctx, bus, AddTaskCmd, func(c command.Context[addTaskPayload]) error {})

// 	// h := command.NewHandlerOf[addTaskPayload](bus)

// 	// h.MustHandle(ctx, AddTaskCmd, func(c command.Context[any]) error {

// 	// })
// }
