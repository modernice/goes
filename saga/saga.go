package saga

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/xtime"
	"github.com/modernice/goes/saga/action"
	"github.com/modernice/goes/saga/report"
)

var (
	// ErrActionNotFound is returned when trying to run an Action which is not
	// configured.
	ErrActionNotFound = errors.New("action not found")

	// ErrEmptyName is returned when an Action is configured with an empty name.
	ErrEmptyName = errors.New("empty action name")

	// ErrCompensateTimeout is returned when the compensation of a SAGA fails
	// due to the CompensateTimeout being exceeded.
	ErrCompensateTimeout = errors.New("compensation timed out")
)

var (
	// DefaultCompensateTimeout is the default timeout for compensating Actions
	// of a failed SAGA.
	DefaultCompensateTimeout = 10 * time.Second
)

// Setup is the setup for a SAGA.
type Setup[ID goes.ID] interface {
	// Sequence returns the names of the actions that should be run sequentially.
	Sequence() []string

	// Compensator finds and returns the name of the compensating action for the
	// Action with the given name. If Compensator returns an empty string, there
	// is no compensator for the given action configured.
	Compensator(string) string

	// Action returns the action with the given name. Action returns nil if no
	// Action with that name was configured.
	Action(string) action.Action[ID]
}

// A Reporter reports the result of a SAGA.
type Reporter[ID goes.ID] interface {
	Report(report.Report[ID])
}

// Option is a Setup option.
type Option[ID goes.ID] func(*setup[ID])

// ExecutorOption is an option for the Execute function.
type ExecutorOption[ID goes.ID] func(*Executor[ID])

// CompensateErr is returned when the compensation of a failed SAGA fails.
type CompensateErr struct {
	Err         error
	ActionError error
}

// An Executor executes SAGAs. Use NewExector to create an Executor.
type Executor[ID goes.ID] struct {
	Setup[ID]

	reporter Reporter[ID]
	eventBus event.Bus[ID]
	cmdBus   command.Bus[ID]
	repo     aggregate.RepositoryOf[ID]

	skipValidate      bool
	compensateTimeout time.Duration

	sequence []action.Action[ID]
	reports  []action.Report[ID]
}

type setup[ID goes.ID] struct {
	actions      []action.Action[ID]
	actionMap    map[string]action.Action[ID]
	sequence     []string
	compensators map[string]string
	startWith    string
}

// Action returns an Option that adds an Action to a SAGA. The first configured
// Action of a SAGA is also its starting Action, unless the StartWith Option is
// used.
func Action[ID goes.ID](name string, run func(action.Context[ID]) error) Option[ID] {
	return Add(action.New(name, run))
}

// Add adds Actions to the SAGA.
func Add[ID goes.ID](acts ...action.Action[ID]) Option[ID] {
	return func(s *setup[ID]) {
		for _, act := range acts {
			if act == nil {
				continue
			}
			s.actions = append(s.actions, act)
		}
	}
}

// StartWith returns an Option that configures the starting Action if a SAGA.
func StartWith[ID goes.ID](action string) Option[ID] {
	return func(s *setup[ID]) {
		s.startWith = action
	}
}

// Sequence returns an Option that defines the sequence of Actions that should
// be run when the SAGA is executed.
//
// When the Sequence Option is used, the StartWith Option has no effect and the
// first Action of the sequence is used as the starting Action.
//
// Example:
//
//	s := saga.New(
//		saga.Action("foo", func(action.Context[ID]) error { return nil }),
//		saga.Action("bar", func(action.Context[ID]) error { return nil }),
//		saga.Action("baz", func(action.Context[ID]) error { return nil }),
//		saga.Sequence("bar", "foo", "baz"),
//	)
//	err := saga.Execute(context.TODO(), s)
//	// would run "bar", "foo" & "baz" sequentially
func Sequence[ID goes.ID](actions ...string) Option[ID] {
	return func(s *setup[ID]) {
		s.sequence = actions
	}
}

// Compensate returns an Option that configures the compensating Action for a
// failed Action.
func Compensate[ID goes.ID](failed, compensateWith string) Option[ID] {
	return func(s *setup[ID]) {
		s.compensators[failed] = compensateWith
	}
}

// Report returns an ExecutorOption that configures the Reporter r to be used by
// the SAGA to report the execution result of the SAGA.
func Report[ID goes.ID](r Reporter[ID]) ExecutorOption[ID] {
	return func(e *Executor[ID]) {
		e.reporter = r
	}
}

// SkipValidation returns an ExecutorOption that disables validation of a Setup
// before it is executed.
func SkipValidation[ID goes.ID]() ExecutorOption[ID] {
	return func(e *Executor[ID]) {
		e.skipValidate = true
	}
}

// EventBus returns an ExecutorOption that provides a SAGA with an event.Bus[ID].
// Actions within that SAGA that receive an action.Context may publish Events
// through that Context over the provided Bus.
//
// Example:
//
//	s := saga.New(saga.Action("foo", func(ctx action.Context[ID]) {
//		evt := event.New(uuid.New(), "foo", fooData{})
//		err := ctx.Publish(ctx, evt)
//		// handle err
//	}))
//
//	var bus event.Bus[ID]
//	err := saga.Execute(context.TODO(), s, saga.EventBus(bus))
//	// handle err
func EventBus[ID goes.ID](bus event.Bus[ID]) ExecutorOption[ID] {
	return func(e *Executor[ID]) {
		e.eventBus = bus
	}
}

// CommandBus returns an ExecutorOption that provides a SAGA with an command.Bus[ID].
// Actions within that SAGA that receive an action.Context may dispatch Commands
// through that Context over the provided Bus. Dispatches over the Command Bus
// are automatically made synchronous.
//
// Example:
//
//	s := saga.New(saga.Action("foo", func(ctx action.Context[ID]) {
//		cmd := command.New("foo", fooPayload{})
//		err := ctx.Dispatch(ctx, cmd)
//		// handle err
//	}))
//
//	var bus command.Bus[ID]
//	err := saga.Execute(context.TODO(), s, saga.CommandBus(bus))
//	// handle err
func CommandBus[ID goes.ID](bus command.Bus[ID]) ExecutorOption[ID] {
	return func(e *Executor[ID]) {
		e.cmdBus = bus
	}
}

// Repository returns an ExecutorOption that provides a SAGA with an
// aggregate.RepositoryOf[ID] Action within that SAGA that receive an action.Context
// may fetch Aggregates through that Context from the provided Repository.
//
// Example:
//
//	s := saga.New(saga.Action("foo", func(ctx action.Context[ID]) {
//		foo := newFooAggregate()
//		err := ctx.Fetch(ctx, foo)
//		// handle err
//	}))
//
//	var repo aggregate.Repository
//	err := saga.Execute(context.TODO(), s, saga.Repository(repo))
//	// handle err
func Repository[ID goes.ID](r aggregate.RepositoryOf[ID]) ExecutorOption[ID] {
	return func(e *Executor[ID]) {
		e.repo = r
	}
}

// CompensateTimeout returns an ExecutorOption that sets the timeout for
// compensating a failed SAGA.
func CompensateTimeout[ID goes.ID](d time.Duration) ExecutorOption[ID] {
	return func(e *Executor[ID]) {
		e.compensateTimeout = d
	}
}

// New returns a reusable Setup that can be safely executed concurrently.
//
// Define Actions
//
// The core of a SAGA are Actions. Actions are basically just named functions
// that can be composed to orchestrate the execution flow of a SAGA. Action can
// be configured with the Action Option:
//
//	s := saga.New(
//		saga.Action("foo", func(action.Context[ID]) error { return nil })),
//		saga.Action("bar", func(action.Context[ID]) error { return nil })),
//	)
//
// By default, the first configured Action is the starting point for the SAGA
// when it's executed. The starting Action can be overriden with the StartWith
// Option:
//
//	s := saga.New(
//		saga.Action("foo", func(action.Context[ID]) error { return nil })),
//		saga.Action("bar", func(action.Context[ID]) error { return nil })),
//		saga.StartWith("bar"),
//	)
//
// Alternatively define a sequence of Actions that should be run when the SAGA
// is executed. The first Action of the sequence will be used as the starting
// Action for the SAGA:
//
//	s := saga.New(
//		saga.Action("foo", func(action.Context[ID]) error { return nil }),
//		saga.Action("bar", func(action.Context[ID]) error { return nil }),
//		saga.Action("baz", func(action.Context[ID]) error { return nil }),
//		saga.Sequence("bar", "foo", "baz"),
//	)
//	// would run "bar", "foo" & "baz" sequentially
//
// Compensate Actions
//
// Every Action a can be assigned a compensating Action c that is called when
// the SAGA fails. Actions are compensated in reverse order and only failed
// Actions will be compensated.
//
// Example:
//
//	s := saga.New(
//		saga.Action("foo", func(action.Context[ID]) error { return nil }),
//		saga.Action("bar", func(action.Context[ID]) error { return nil }),
//		saga.Action("baz", func(action.Context[ID]) error { return errors.New("whoops") }),
//		saga.Action("compensate-foo", func(action.Context[ID]) error { return nil }),
//		saga.Action("compensate-bar", func(action.Context[ID]) error { return nil }),
//		saga.Action("compensate-baz", func(action.Context[ID]) error { return nil }),
//		saga.Compensate("foo", "compensate-foo"),
//		saga.Compensate("bar", "compensate-bar"),
//		saga.Compensate("baz", "compensate-baz"),
//		saga.Sequence("foo", "bar", "baz"),
//	)
//
// The above Setup would run the following Actions in order:
//	`foo`, `bar`, `baz`, `compensate-bar`, `compensate-foo`
//
// A SAGA that successfully compensated every Action still returns the error
// that triggered the compensation. In order to check if a SAGA was compensated,
// unwrap the error into a *CompensateErr:
//
//	var s saga.Setup
//	if err := saga.Execute(context.TODO(), s); err != nil {
//		if compError, ok := saga.CompensateError(err); ok {
//			log.Println(fmt.Sprintf("Compensation failed: %s", compError))
//		} else {
//			log.Println(fmt.Sprintf("SAGA failed: %s", err))
//		}
//	}
//
// Action Context
//
// An Action has access to an action.Context that allows the Action to run
// other Actions in the same SAGA and, depending on the passed ExecutorOptions,
// publish Events and dispatch Commands:
//
//	s := saga.New(
//		saga.Action("foo", func(ctx action.Context[ID]) error {
//			if err := ctx.Run(ctx, "bar"); err != nil {
//				return fmt.Errorf("run %q: %w", "bar", err)
//			}
//			if err := ctx.Publish(ctx, event.New(...)); err != nil {
//				return fmt.Errorf("publish event: %w", err)
//			}
//			if err := ctx.Dispatch(ctx, command.New(...)); err != nil {
//				return fmt.Errorf("publish command: %w", err)
//			}
//			return nil
//		}),
//	)
func New[ID goes.ID](opts ...Option[ID]) Setup[ID] {
	s := setup[ID]{
		actionMap:    make(map[string]action.Action[ID]),
		compensators: make(map[string]string),
	}
	for _, opt := range opts {
		opt(&s)
	}
	for _, act := range s.actions {
		s.actionMap[act.Name()] = act
	}
	if s.startWith == "" && len(s.sequence) == 0 && len(s.actions) > 0 && s.actions[0] != nil {
		s.startWith = s.actions[0].Name()
	}
	return &s
}

// Validate validates that a Setup is configured safely and returns an error in
// the following cases:
//	- `ErrEmptyName` if an Action has an empty name (or just whitespace)
//	- `ErrActionNotFound` if the sequence contains an unconfigured Action
//	- `ErrActionNotFound` if an unknown Action is configured as a compensating Action
func Validate[ID goes.ID](s Setup[ID]) error {
	for _, name := range s.Sequence() {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("%q action: %w", name, ErrEmptyName)
		}

		if s.Action(name) == nil {
			return fmt.Errorf("%q action: %w", name, ErrActionNotFound)
		}

		if cname := s.Compensator(name); cname != "" {
			if s.Action(cname) == nil {
				return fmt.Errorf("%q action: %w", cname, ErrActionNotFound)
			}
		}
	}

	return nil
}

// CompensateError unwraps the *CompensateErr from the given error.
func CompensateError(err error) (*CompensateErr, bool) {
	if err == nil {
		return nil, false
	}
	var cerr *CompensateErr
	if !errors.As(err, &cerr) {
		return nil, false
	}
	return cerr, true
}

// Execute executes the given Setup.
//
// Execution error
//
// Execution can fail because of multiple reasons. Execute returns an error in
// the following cases:
//
//	- Setup validation fails (see Validate)
//	- An Action fails and compensation is disabled
//	- An Action and any of the subsequent compensations fail
//
// Compensation is disabled when no compensators are configured.
//
// Skip validation
//
// Validation of the Setup can be skipped by providing the SkipValidation
// ExecutorOption, but should only be used when the Setup s is ensured to be
// valid (e.g. by calling Validate manually beforehand).
//
// Reporting
//
// When a Reporter r is provided using the Report ExecutorOption, Execute will
// call r.Report with a report.Report which contains detailed information about
// the execution of the SAGA (a *report.Report is also a Reporter[ID]):
//
//	s := saga.New(...)
//	var r report.Report
//	err := saga.Execute(context.TODO(), s, saga.Report(&r))
//	// err == r.Error()
func Execute[ID goes.ID](ctx context.Context, s Setup[ID], opts ...ExecutorOption[ID]) error {
	return NewExecutor(opts...).Execute(ctx, s)
}

// NewExecutor returns a SAGA executor.
func NewExecutor[ID goes.ID](opts ...ExecutorOption[ID]) *Executor[ID] {
	e := Executor[ID]{compensateTimeout: DefaultCompensateTimeout}
	for _, opt := range opts {
		opt(&e)
	}
	return &e
}

func (s *setup[ID]) Actions() []action.Action[ID] {
	acts := make([]action.Action[ID], len(s.actions))
	for i, act := range s.actions {
		acts[i] = act
	}
	return acts
}

func (s *setup[ID]) Compensators() map[string]string {
	m := make(map[string]string, len(s.compensators))
	for k, v := range s.compensators {
		m[k] = v
	}
	return m
}

func (s *setup[ID]) Compensator(name string) string {
	return s.compensators[name]
}

func (s *setup[ID]) Sequence() []string {
	if len(s.actions) == 0 {
		return nil
	}

	if len(s.sequence) == 0 && s.startWith != "" {
		return []string{s.startWith}
	}

	return s.sequence
}

func (s *setup[ID]) Action(name string) action.Action[ID] {
	return s.actionMap[name]
}

// Execute executes the given SAGA.
func (e *Executor[ID]) Execute(ctx context.Context, s Setup[ID]) error {
	e = e.clone(s)

	if !e.skipValidate {
		if err := Validate(s); err != nil {
			return fmt.Errorf("validate setup: %w", err)
		}
	}

	start := xtime.Now()

	for _, name := range s.Sequence() {
		act, err := e.action(name)
		if err != nil {
			return e.finish(start, fmt.Errorf("find %q action: %w", name, err))
		}

		if actionError := e.run(ctx, act); actionError != nil {
			if !e.shouldRollback() {
				return e.finish(start, actionError)
			}

			if err := e.rollback(); err != nil {
				return e.finish(start, &CompensateErr{
					Err:         err,
					ActionError: actionError,
				})
			}

			return e.finish(start, actionError)
		}
	}

	return e.finish(start, nil)
}

func (e *Executor[ID]) action(name string) (action.Action[ID], error) {
	act := e.Action(name)
	if act == nil {
		return act, ErrActionNotFound
	}
	return act, nil
}

func (e *Executor[ID]) finish(start time.Time, err error) error {
	if e.reporter == nil {
		return err
	}

	end := xtime.Now()
	e.reporter.Report(report.New(
		start, end, report.Add(e.reports...),
		report.Error[ID](err),
	))

	return err
}

func (e *Executor[ID]) run(ctx context.Context, act action.Action[ID]) error {
	actionCtx := e.newActionContext(ctx, act)
	return e.runContext(actionCtx)
}

func (e *Executor[ID]) newActionContext(ctx context.Context, act action.Action[ID]) action.Context[ID] {
	return action.NewContext(
		ctx,
		act,
		action.WithRunner[ID](e.runAction),
		action.WithEventBus[ID](e.eventBus),
		action.WithCommandBus[ID](e.cmdBus),
		action.WithRepository[ID](e.repo),
	)
}

func (e *Executor[ID]) runContext(ctx action.Context[ID]) error {
	start := xtime.Now()
	act := ctx.Action()
	err := act.Run(ctx)
	end := xtime.Now()
	e.reports = append(e.reports, action.NewReport(
		act,
		start, end,
		action.Error[ID](err),
	))
	return err
}

func (e *Executor[ID]) runAction(ctx context.Context, name string) error {
	act, err := e.action(name)
	if err != nil {
		return fmt.Errorf("find %q action: %w", name, err)
	}
	if err = e.run(ctx, act); err != nil {
		return err
	}
	return nil
}

func (e *Executor[ID]) shouldRollback() bool {
	if len(e.reports) == 0 {
		return false
	}
	for _, rep := range e.reports {
		if name := e.Compensator(rep.Action.Name()); name != "" {
			return true
		}
	}
	return false
}

func (e *Executor[ID]) rollback() error {
	ctx, cancel := context.WithTimeout(context.Background(), e.compensateTimeout)
	defer cancel()

	for i := len(e.reports) - 1; i >= 0; i-- {
		res := e.reports[i]
		if res.Error != nil {
			continue
		}

		var err error
		select {
		case <-ctx.Done():
			return fmt.Errorf("rollback %q Action: %w", res.Action.Name(), ErrCompensateTimeout)
		case err = <-e.rollbackAction(ctx, res):
		}

		if err != nil {
			return fmt.Errorf("rollback %q action: %w", res.Action.Name(), err)
		}
	}
	return nil
}

func (e *Executor[ID]) rollbackAction(ctx context.Context, rep action.Report[ID]) <-chan error {
	out := make(chan error)
	ret := func(err error) {
		select {
		case <-ctx.Done():
		case out <- err:
		}
	}
	go func() {
		name := e.Compensator(rep.Action.Name())
		if name == "" {
			ret(nil)
			return
		}
		comp := e.Action(name)
		if comp == nil {
			ret(fmt.Errorf("find %q action: %w", name, ErrActionNotFound))
			return
		}
		ret(e.run(ctx, comp))
	}()
	return out
}

func (e Executor[ID]) clone(s Setup[ID]) *Executor[ID] {
	e.Setup = s
	e.sequence = nil
	e.reports = nil
	return &e
}

func (err *CompensateErr) Error() string {
	return err.Err.Error()
}

func (err *CompensateErr) Unwrap() error {
	return err.Err
}
