package lookup

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

var (
	// ErrNotFound is returned by Expect when the value for the given key cannot be found.
	ErrNotFound = errors.New("value for key not found")

	// ErrWrongType is returned by Expect when the value for the given key is not of the expected type.
	ErrWrongType = errors.New("value is not of the expected type")
)

// Lookup is a projection that provides a lookup table for a given set of
// events. The lookup table is populated by events that implment the Data
// interface. A *Lookup is thread-safe.
type Lookup struct {
	scheduleOpts []schedule.ContinuousOption
	applyEvent   func(event.Event)
	schedule     *schedule.Continuous

	mux       sync.RWMutex
	providers map[string]*provider

	once  sync.Once
	ready chan struct{}
}

type Option func(*Lookup)

// Data is the interface that must be implemented by events that want to
// populate the lookup table. The Provider that is passed to ProvideLookup
// allows the event to update the lookup table of a specific aggregate. The
// Provider that is provided to the ProvideLookup method is restricted to the
// aggregate that the event belongs to. The Provider is not thread-safe and must
// not be used outside of the ProvideLookup method. To retrieve a thread-safe
// Provider, use the l.Provider() method on the *Lookup type.
//
//	// UserRegisteredData is the event data for a registered user.
//	type UserRegisteredData struct {
//		Email string
//	}
//
//	func (data UserRegisteredData) ProvideLookup(p lookup.Provider) {
//		p.Provide("email", data.Email)
//	}
type Data interface {
	// ProvideLookup accepts a Provider that can be used to update the lookup
	// table of a specific aggregate.
	ProvideLookup(Provider)
}

// Provider allows events to update the lookup table of a specific aggregate.
type Provider interface {
	// Provide provides the lookup value for the given key.
	Provide(key string, value any)

	// Remove removes the lookup values for the given keys.
	Remove(keys ...string)
}

type lookup interface {
	Lookup(context.Context, string, string, uuid.UUID) (any, bool)
}

// Expect calls l.Lookup with the given arguments and casts the result to the
// given generic type. If the lookup value cannot be found, an error that
// unwraps to ErrNotFound is returned. If the lookup value is not of the
// expected type, an error that unwraps to ErrWrongType is returned.
func Expect[Value any](ctx context.Context, l lookup, aggregateName, key string, aggregateID uuid.UUID) (Value, error) {
	val, ok := l.Lookup(ctx, aggregateName, key, aggregateID)
	if !ok {
		var zero Value
		return zero, fmt.Errorf("%w [key=%v, aggregateName=%v, aggregateId=%v]", ErrNotFound, key, aggregateName, aggregateID)
	}

	if v, ok := val.(Value); ok {
		return v, nil
	}

	var zero Value
	return zero, fmt.Errorf("%w: want=%T, got=%T [key=%v, aggregateName=%v, aggregateId=%v]", ErrWrongType, zero, val, key, aggregateName, aggregateID)
}

// Contains returns whether the lookup table contains the given key for the
// given aggregate.
func Contains[Value any](ctx context.Context, l lookup, aggregateName, key string, aggregateID uuid.UUID) bool {
	val, ok := l.Lookup(ctx, aggregateName, key, aggregateID)
	if !ok {
		return false
	}
	_, ok = val.(Value)
	return ok
}

// ScheduleOptions returns an Option that configures the continuous schedule
// that is created by the lookup.
func ScheduleOptions(opts ...schedule.ContinuousOption) Option {
	return func(l *Lookup) {
		l.scheduleOpts = append(l.scheduleOpts, opts...)
	}
}

// ApplyEventsWith returns an Option that overrides the default function that
// applies events to the lookup table. When an event is applied, the provided
// function is called with the event as its first argument and the original
// event applier function as its second argument.
func ApplyEventsWith(fn func(evt event.Event, original func(event.Event))) Option {
	return func(l *Lookup) {
		l.applyEvent = func(evt event.Event) {
			fn(evt, l.defaultApplyEvent)
		}
	}
}

// New returns a new lookup table. The lookup table becomes ready after the
// first projection job has been applied. Use the l.Ready() method of the
// returned *Lookup to wait for the lookup table to become ready. Use l.Run()
// to start the projection of the lookup table.
func New(store event.Store, bus event.Bus, events []string, opts ...Option) *Lookup {
	l := &Lookup{
		providers: make(map[string]*provider),
		ready:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(l)
	}

	l.schedule = schedule.Continuously(bus, store, events, l.scheduleOpts...)

	if l.applyEvent == nil {
		l.applyEvent = func(e event.Event) { l.defaultApplyEvent(e) }
	}

	return l
}

// Ready returns a channel that is closed when the lookup table is ready. The
// lookup table becomes ready after the first projection job has been applied.
// Call l.Run() to start the projection of the lookup table.
func (l *Lookup) Ready() <-chan struct{} {
	return l.ready
}

// Schedule returns the projection schedule for the lookup table.
func (l *Lookup) Schedule() *schedule.Continuous {
	return l.schedule
}

// Map returns the all values in the lookup table as a nested map, structured as follows:
//	map[AGGREGATE_NAME]map[AGGREGATE_ID]map[LOOKUP_KEY]LOOKUP_VALUE
func (l *Lookup) Map() map[string]map[uuid.UUID]map[any]any {
	l.mux.RLock()
	defer l.mux.RUnlock()
	out := make(map[string]map[uuid.UUID]map[any]any)
	for name, p := range l.providers {
		out[name] = make(map[uuid.UUID]map[any]any)
		for id, store := range p.stores {
			out[name][id] = make(map[any]any)
			for k, v := range store.values {
				out[name][id][k] = v
			}
		}
	}
	return out
}

// Provider returns the lookup provider for the given aggregate. The returned Provider
// is thread-safe.
func (l *Lookup) Provider(aggregateName string, aggregateID uuid.UUID) Provider {
	l.mux.RLock()
	if prov, ok := l.providers[aggregateName]; ok {
		l.mux.RUnlock()
		return &provider{
			mux:    &l.mux,
			stores: make(map[uuid.UUID]*store),
			active: prov.store(aggregateID),
		}
	}
	l.mux.RUnlock()

	l.mux.Lock()
	defer l.mux.Unlock()

	return &provider{
		mux:    &l.mux,
		stores: make(map[uuid.UUID]*store),
		active: l.provider(aggregateName).store(aggregateID),
	}
}

// Lookup returns the lookup value for the given key of the given aggregate, or
// false if no value for the key exists.
func (l *Lookup) Lookup(ctx context.Context, aggregateName, key string, aggregateID uuid.UUID) (any, bool) {
	select {
	case <-ctx.Done():
		return nil, false
	case <-l.Ready():
	}

	l.mux.RLock()
	defer l.mux.RUnlock()

	s := l.provider(aggregateName).store(aggregateID)

	return s.get(key)
}

// Reverse returns the aggregate id that has the given value as the lookup value
// for the given lookup key. If value is not comparable or if the value does not
// exists for the given aggregate, uuid.Nil and false are returned. Otherwise
// the aggregate id and true are returned.
func (l *Lookup) Reverse(ctx context.Context, aggregateName, key string, value any) (uuid.UUID, bool) {
	if !isKeyable(value) {
		return uuid.Nil, false
	}

	select {
	case <-ctx.Done():
		return uuid.Nil, false
	case <-l.Ready():
	}

	l.mux.RLock()
	defer l.mux.RUnlock()

	return l.provider(aggregateName).id(value)
}

// Run runs the projection of the lookup table until ctx is canceled. Any
// asynchronous errors are sent into the returned channel.
func (l *Lookup) Run(ctx context.Context) (<-chan error, error) {
	errs, err := l.schedule.Subscribe(ctx, l.ApplyJob)
	if err != nil {
		return nil, fmt.Errorf("subscribe to projection schedule: %w", err)
	}

	go l.schedule.Trigger(ctx)

	return errs, nil
}

// ApplyJob applies the given projection job on the lookup table.
func (l *Lookup) ApplyJob(ctx projection.Job) error {
	defer l.once.Do(func() { close(l.ready) })
	l.mux.Lock()
	defer l.mux.Unlock()
	return ctx.Apply(ctx, l)
}

// ApplyEvent implements projection.EventApplier.
func (l *Lookup) ApplyEvent(evt event.Event) {
	l.applyEvent(evt)
}

func (l *Lookup) defaultApplyEvent(evt event.Event) {
	data, ok := evt.Data().(Data)
	if !ok {
		return
	}

	id, name, _ := evt.Aggregate()

	prov := l.provider(name)
	prov.active = prov.store(id)

	data.ProvideLookup(prov)
}

func (l *Lookup) provider(aggregateName string) *provider {
	if p, ok := l.providers[aggregateName]; ok {
		return p
	}
	prov := &provider{
		stores: make(map[uuid.UUID]*store),
		ids:    make(map[any]uuid.UUID),
	}
	l.providers[aggregateName] = prov
	return prov
}

type provider struct {
	mux    *sync.RWMutex // only for providers returned by (*Lookup).Provider()
	stores map[uuid.UUID]*store
	ids    map[any]uuid.UUID
	active *store
}

func (p *provider) Provide(key string, val any) {
	p.Remove(key)

	if p.mux != nil {
		p.mux.Lock()
		defer p.mux.Unlock()
	}
	p.active.provide(key, val)

	if p.active != nil && isKeyable(val) {
		p.ids[val] = p.active.aggregateID
	}
}

func (p *provider) Remove(keys ...string) {
	if p.mux != nil {
		p.mux.Lock()
		defer p.mux.Unlock()
	}

	for _, key := range keys {
		val, ok := p.active.remove(key)
		if !ok || p.active == nil {
			continue
		}

		if isKeyable(val) {
			delete(p.ids, val)
		}
	}
}

func (p *provider) id(val any) (uuid.UUID, bool) {
	if p.mux != nil {
		p.mux.Lock()
		defer p.mux.Unlock()
	}
	id, ok := p.ids[val]
	return id, ok
}

func (p *provider) store(aggregateID uuid.UUID) *store {
	if s, ok := p.stores[aggregateID]; ok {
		return s
	}
	s := &store{
		aggregateID: aggregateID,
		values:      make(map[string]any),
	}
	p.stores[aggregateID] = s
	return s
}

type store struct {
	aggregateID uuid.UUID
	values      map[string]any
}

func (s *store) get(key string) (any, bool) {
	v, ok := s.values[key]
	return v, ok
}

func (s *store) provide(key string, val any) {
	s.values[key] = val
}

func (s *store) remove(key string) (any, bool) {
	if val, ok := s.values[key]; ok {
		delete(s.values, key)
		return val, true
	}
	return nil, false
}

func isKeyable(val any) bool {
	rt := reflect.TypeOf(val)
	for rtk := rt.Kind(); rtk == reflect.Interface; {
		rt = rt.Elem()
	}

	if rtk := rt.Kind(); rtk <= reflect.Array || rtk == reflect.Pointer || rtk == reflect.String || rt.Comparable() {
		return true
	}

	return false
}
