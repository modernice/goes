package lookup

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
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
	schedule *schedule.Continuous

	sync.RWMutex
	providers map[aggregate.Ref]*provider

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

// New returns a new lookup table. The lookup table becomes ready after the
// first projection job has been applied. Use the l.Ready() method of the
// returned *Lookup to wait for the lookup table to become ready. Use l.Run()
// to start the projection of the lookup table.
func New(store event.Store, bus event.Bus, events []string, opts ...schedule.ContinuousOption) *Lookup {
	l := &Lookup{
		schedule:  schedule.Continuously(bus, store, events, opts...),
		providers: make(map[event.AggregateRef]*provider),
		ready:     make(chan struct{}),
	}
	return l
}

// Ready returns a channel that is closed when the lookup table is ready. The
// lookup table becomes ready after the first projection job has been applied.
// Call l.Run() to start the projection of the lookup table.
func (l *Lookup) Ready() <-chan struct{} {
	return l.ready
}

// Provider returns the Provider for the given aggregate. The returned Provider
// is is thread-safe.
func (l *Lookup) Provider(aggregateName string, aggregateID uuid.UUID) Provider {
	ref := aggregate.Ref{
		Name: aggregateName,
		ID:   aggregateID,
	}

	l.RLock()
	if prov, ok := l.providers[ref]; ok {
		l.RUnlock()
		return &provider{
			mux:    &l.RWMutex,
			values: prov.values,
		}
	}
	l.RUnlock()

	l.Lock()
	defer l.Unlock()

	return &provider{
		mux:    &l.RWMutex,
		values: l.newProvider(ref).values,
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

	l.RLock()
	defer l.RUnlock()

	ref := aggregate.Ref{Name: aggregateName, ID: aggregateID}
	provider, ok := l.providers[ref]
	if !ok {
		return nil, false
	}

	return provider.get(key)
}

// Run runs the projection of the lookup table until ctx is canceled. Any
// asynchronous errors are sent into the returned channel.
func (l *Lookup) Run(ctx context.Context) (<-chan error, error) {
	errs, err := l.schedule.Subscribe(ctx, func(ctx projection.Job) error {
		defer l.once.Do(func() { close(l.ready) })
		l.Lock()
		defer l.Unlock()
		return ctx.Apply(ctx, l)
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe to projection schedule: %w", err)
	}

	go l.schedule.Trigger(ctx)

	return errs, nil
}

// ApplyEvent implements projection.EventApplier.
func (l *Lookup) ApplyEvent(evt event.Event) {
	data, ok := evt.Data().(Data)
	if !ok {
		return
	}

	id, name, _ := evt.Aggregate()
	ref := aggregate.Ref{
		Name: name,
		ID:   id,
	}

	prov, ok := l.providers[ref]
	if !ok {
		prov = l.newProvider(ref)
	}
	data.ProvideLookup(prov)
}

func (l *Lookup) newProvider(ref aggregate.Ref) *provider {
	prov := &provider{values: make(map[string]any)}
	l.providers[ref] = prov
	return prov
}

type provider struct {
	mux    *sync.RWMutex // only for providers returned by (*Lookup).Provider()
	values map[string]any
}

func (p *provider) get(key string) (any, bool) {
	if p.mux != nil {
		p.mux.RLock()
		defer p.mux.RUnlock()
	}
	v, ok := p.values[key]
	return v, ok
}

func (p *provider) Provide(key string, val any) {
	if p.mux != nil {
		p.mux.Lock()
		defer p.mux.Unlock()
	}
	p.values[key] = val
}

func (p *provider) Remove(keys ...string) {
	if p.mux != nil {
		p.mux.Lock()
		defer p.mux.Unlock()
	}
	for _, key := range keys {
		delete(p.values, key)
	}
}
