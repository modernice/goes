package lookup

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/modernice/goes"
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
type Lookup[ID goes.ID] struct {
	schedule *schedule.Continuous[ID]

	mux       sync.RWMutex
	providers map[string]*provider[ID]

	once  sync.Once
	ready chan struct{}
}

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

type lookup[ID goes.ID] interface {
	Lookup(context.Context, string, string, ID) (any, bool)
}

// Expect calls l.Lookup with the given arguments and casts the result to the
// given generic type. If the lookup value cannot be found, an error that
// unwraps to ErrNotFound is returned. If the lookup value is not of the
// expected type, an error that unwraps to ErrWrongType is returned.
func Expect[Value any, ID goes.ID](ctx context.Context, l lookup[ID], aggregateName, key string, aggregateID ID) (Value, error) {
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
func Contains[Value any, ID goes.ID](ctx context.Context, l lookup[ID], aggregateName, key string, aggregateID ID) bool {
	val, ok := l.Lookup(ctx, aggregateName, key, aggregateID)
	if !ok {
		return false
	}
	_, ok = val.(Value)
	return ok
}

// New returns a new lookup table. The lookup table becomes ready after the
// first projection job has been applied. Use the l.Ready() method of the
// returned *Lookup to wait for the lookup table to become ready. Use l.Run()
// to start the projection of the lookup table.
func New[ID goes.ID](store event.Store[ID], bus event.Bus[ID], events []string, opts ...schedule.ContinuousOption[ID]) *Lookup[ID] {
	l := &Lookup[ID]{
		schedule:  schedule.Continuously(bus, store, events, opts...),
		providers: make(map[string]*provider[ID]),
		ready:     make(chan struct{}),
	}
	return l
}

// Ready returns a channel that is closed when the lookup table is ready. The
// lookup table becomes ready after the first projection job has been applied.
// Call l.Run() to start the projection of the lookup table.
func (l *Lookup[ID]) Ready() <-chan struct{} {
	return l.ready
}

// Provider returns the Provider for the given aggregate. The returned Provider
// is is thread-safe.
func (l *Lookup[ID]) Provider(aggregateName string, aggregateID ID) Provider {
	l.mux.RLock()
	if prov, ok := l.providers[aggregateName]; ok {
		l.mux.RUnlock()
		return &provider[ID]{
			mux:    &l.mux,
			stores: make(map[ID]*store[ID]),
			active: prov.store(aggregateID),
		}
	}
	l.mux.RUnlock()

	l.mux.Lock()
	defer l.mux.Unlock()

	return &provider[ID]{
		mux:    &l.mux,
		stores: make(map[ID]*store[ID]),
		active: l.provider(aggregateName).store(aggregateID),
	}
}

// Lookup returns the lookup value for the given key of the given aggregate, or
// false if no value for the key exists.
func (l *Lookup[ID]) Lookup(ctx context.Context, aggregateName, key string, aggregateID ID) (any, bool) {
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
func (l *Lookup[ID]) Reverse(ctx context.Context, aggregateName, key string, value any) (ID, bool) {
	var zero ID

	if !isKeyable(value) {
		return zero, false
	}

	select {
	case <-ctx.Done():
		return zero, false
	case <-l.Ready():
	}

	l.mux.RLock()
	defer l.mux.RUnlock()

	return l.provider(aggregateName).id(value)
}

// Run runs the projection of the lookup table until ctx is canceled. Any
// asynchronous errors are sent into the returned channel.
func (l *Lookup[ID]) Run(ctx context.Context) (<-chan error, error) {
	errs, err := l.schedule.Subscribe(ctx, func(ctx projection.Job[ID]) error {
		defer l.once.Do(func() { close(l.ready) })
		l.mux.Lock()
		defer l.mux.Unlock()
		return ctx.Apply(ctx, l)
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe to projection schedule: %w", err)
	}

	go l.schedule.Trigger(ctx)

	return errs, nil
}

// ApplyEvent implements projection.EventApplier.
func (l *Lookup[ID]) ApplyEvent(evt event.Of[any, ID]) {
	data, ok := evt.Data().(Data)
	if !ok {
		return
	}

	id, name, _ := evt.Aggregate()

	prov := l.provider(name)
	prov.active = prov.store(id)

	data.ProvideLookup(prov)
}

func (l *Lookup[ID]) provider(aggregateName string) *provider[ID] {
	if p, ok := l.providers[aggregateName]; ok {
		return p
	}
	prov := &provider[ID]{
		stores: make(map[ID]*store[ID]),
		ids:    make(map[any]ID),
	}
	l.providers[aggregateName] = prov
	return prov
}

type provider[ID goes.ID] struct {
	mux    *sync.RWMutex // only for providers returned by (*Lookup).Provider()
	stores map[ID]*store[ID]
	ids    map[any]ID
	active *store[ID]
}

func (p *provider[ID]) Provide(key string, val any) {
	if p.mux != nil {
		p.mux.Lock()
		defer p.mux.Unlock()
	}
	p.active.provide(key, val)

	if p.active != nil && isKeyable(val) {
		p.ids[val] = p.active.aggregateID
	}
}

func (p *provider[ID]) Remove(keys ...string) {
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

func (p *provider[ID]) id(val any) (ID, bool) {
	if p.mux != nil {
		p.mux.Lock()
		defer p.mux.Unlock()
	}
	id, ok := p.ids[val]
	return id, ok
}

func (p *provider[ID]) store(aggregateID ID) *store[ID] {
	if s, ok := p.stores[aggregateID]; ok {
		return s
	}
	s := &store[ID]{
		aggregateID: aggregateID,
		values:      make(map[string]any),
	}
	p.stores[aggregateID] = s
	return s
}

type store[ID goes.ID] struct {
	aggregateID ID
	values      map[string]any
}

func (s *store[ID]) get(key string) (any, bool) {
	v, ok := s.values[key]
	return v, ok
}

func (s *store[ID]) provide(key string, val any) {
	s.values[key] = val
}

func (s *store[ID]) remove(key string) (any, bool) {
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

	if rtk := rt.Kind(); rtk <= reflect.Array || rtk == reflect.Pointer || rtk == reflect.String {
		return true
	}

	return false
}
