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
	// ErrNotFound indicates that no value exists for the key.
	ErrNotFound = errors.New("value for key not found")

	// ErrWrongType indicates a type mismatch when casting a lookup value.
	ErrWrongType = errors.New("value is not of the expected type")
)

// Lookup maintains lookup tables populated from events.
type Lookup struct {
	scheduleOpts []schedule.ContinuousOption
	applyEvent   func(event.Event)
	schedule     *schedule.Continuous

	mux       sync.RWMutex
	providers map[string]*provider

	once  sync.Once
	ready chan struct{}
}

// Option configures a [Lookup].
type Option func(*Lookup)

// Data marks events that can update the lookup table.
type Data interface {
	// ProvideLookup accepts a Provider that can be used to update the lookup
	// table of a specific aggregate.
	ProvideLookup(Provider)
}

// Provider updates lookup entries for a specific aggregate.
type Provider interface {
	// Provide stores value under key.
	Provide(key string, value any)

	// Remove deletes values for the given keys.
	Remove(keys ...string)
}

type lookup interface {
	// Lookup returns the lookup value for the given key of the given aggregate, or
	// false if no value for the key exists.
	Lookup(context.Context, string, string, uuid.UUID) (any, bool)
}

// Expect fetches and casts a lookup value.
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

// Contains reports whether a value of type Value exists for key.
func Contains[Value any](ctx context.Context, l lookup, aggregateName, key string, aggregateID uuid.UUID) bool {
	val, ok := l.Lookup(ctx, aggregateName, key, aggregateID)
	if !ok {
		return false
	}
	_, ok = val.(Value)
	return ok
}

// ScheduleOptions appends options for the internal Continuous schedule.
func ScheduleOptions(opts ...schedule.ContinuousOption) Option {
	return func(l *Lookup) {
		l.scheduleOpts = append(l.scheduleOpts, opts...)
	}
}

// ApplyEventsWith overrides how events are applied to the table.
func ApplyEventsWith(fn func(evt event.Event, original func(event.Event))) Option {
	return func(l *Lookup) {
		l.applyEvent = func(evt event.Event) {
			fn(evt, l.defaultApplyEvent)
		}
	}
}

// New constructs a Lookup. Call Run and wait on Ready before querying.
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

// Ready closes once the lookup table has processed its first job.
func (l *Lookup) Ready() <-chan struct{} {
	return l.ready
}

// Schedule exposes the underlying schedule.
func (l *Lookup) Schedule() *schedule.Continuous {
	return l.schedule
}

// Map returns all values in a nested map:
//
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

// Provider returns a thread-safe provider for the aggregate.
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

// Provide is a method of the Provider interface that allows events to update
// the lookup table of a specific aggregate. It accepts a string key and an
// arbitrary value. The value can be any type that is comparable or an
// interface{} type. If the value is not comparable, it will not be indexed for
// reverse lookups. If the Provider is created using the *Lookup.Provider
// method, it will be thread-safe.
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

// Remove removes the lookup values for the given keys from the Provider.
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
