package event

//go:generate mockgen -source=registry.go -destination=./mocks/registry.go

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
)

var (
	// ErrUnregistered is returned when trying to create Event Data which hasn't
	// been registered in a Registry.
	ErrUnregistered = errors.New("unregistered Event")
)

// An Encoder encodes & decodes Event Data.
type Encoder interface {
	// Encode encodes d and writes the result into w.
	Encode(w io.Writer, name string, d Data) error

	// Decode decodes the Data in r based on the specified Event name.
	Decode(name string, r io.Reader) (Data, error)
}

// A Registry is an Encoder that also allows to register new Events into it and
// instantiate Data from Event names.
type Registry interface {
	Encoder

	// Register registers a Data factory for Events with the given name.
	Register(name string, newData func() Data)

	// New returns new Data for an Event with the given name.
	New(name string) (Data, error)
}

// A Marshaler knows how to encode itself into a byte slice. Event Data can
// implement Marshaler to override the default marshaling strategy which is by
// default using encoding/gob to encode the Event Data.
//
// Example using encoding/json instead of encoding/gob:
//	type FooEvent struct {
//		A string
//		B int
//		C bool
//	}
//
//	func (e FooEvent) MarshalEvent() ([]byte, error) {
//		return json.Marshal(e)
//	}
type Marshaler interface {
	MarshalEvent() ([]byte, error)
}

// An Unmarshaler knows how to decode itself from a bytes. Event Data can
// implement Unmarshaler to override the default unmarshaling strategy which is
// by default using encoding/gob to decode the Event Data.
//
// Example using encoding/json instead of encoding/gob:
//	type FooEvent struct {
//		A string
//		B int
//		C bool
//	}
//
//	func (e *FooEvent) UnmarshalEvent(data []byte) error {
//		return json.Unmarshal(data, e)
//	}
type Unmarshaler interface {
	UnmarshalEvent([]byte) error
}

// A RegistryOption configures a Registry.
type RegistryOption func(*registry)

type registry struct {
	gobName         func(string) string
	factoriesMux    sync.RWMutex
	factories       map[string]func() Data
	unmarshalersMux sync.RWMutex
	unmarshalers    map[string]bool
	marshalersMux   sync.RWMutex
	marshalers      map[string]bool
}

// GobNameFunc returns a RegistryOption that specifies under which name Event
// Data is registered with gob.RegisterName. The default name under which Events
// are registered is "goes.event(eventName)" where eventName is the name of the
// Event.
func GobNameFunc(fn func(string) string) RegistryOption {
	return func(r *registry) {
		r.gobName = fn
	}
}

// NewRegistry returns a new Event Registry.
func NewRegistry(opts ...RegistryOption) Registry {
	reg := &registry{
		factories:    make(map[string]func() Data),
		unmarshalers: make(map[string]bool),
		marshalers:   make(map[string]bool),
	}
	for _, opt := range opts {
		opt(reg)
	}
	if reg.gobName == nil {
		reg.gobName = defaultGobName
	}
	return reg
}

func (r *registry) Encode(w io.Writer, name string, data Data) error {
	r.marshalersMux.RLock()
	isMarshaler, checkedMarshal := r.marshalers[name]
	r.marshalersMux.RUnlock()

	if !checkedMarshal {
		m, ok := data.(Marshaler)
		r.marshalersMux.Lock()
		r.marshalers[name] = ok
		r.marshalersMux.Unlock()

		if ok {
			b, err := m.MarshalEvent()
			if err != nil {
				return fmt.Errorf("marshal Data: %w", err)
			}

			if _, err := w.Write(b); err != nil {
				return err
			}

			return nil
		}
	}

	if isMarshaler {
		m := data.(Marshaler)
		b, err := m.MarshalEvent()
		if err != nil {
			return fmt.Errorf("marshal Data: %w", err)
		}

		if _, err := w.Write(b); err != nil {
			return err
		}
	}

	if err := gob.NewEncoder(w).Encode(&data); err != nil {
		return err
	}

	return nil
}

func (r *registry) Decode(name string, data io.Reader) (Data, error) {
	r.unmarshalersMux.RLock()
	isUnmarshaler, checkedUnmarshal := r.unmarshalers[name]
	r.unmarshalersMux.RUnlock()

	if !checkedUnmarshal {
		p, err := r.newPointer(name)
		if err != nil {
			return nil, err
		}

		m, ok := p.(Unmarshaler)
		r.unmarshalersMux.Lock()
		r.unmarshalers[name] = ok
		r.unmarshalersMux.Unlock()

		if ok {
			b, err := io.ReadAll(data)
			if err != nil {
				return nil, fmt.Errorf("read Data: %w", err)
			}

			if err := m.UnmarshalEvent(b); err != nil {
				return nil, fmt.Errorf("unmarshal Data: %w", err)
			}

			return deref(p), nil
		}
	}

	if isUnmarshaler {
		d, err := r.New(name)
		if err != nil {
			return nil, err
		}
		m := d.(Unmarshaler)

		b, err := io.ReadAll(data)
		if err != nil {
			return nil, fmt.Errorf("read Data: %w", err)
		}

		if err := m.UnmarshalEvent(b); err != nil {
			return nil, fmt.Errorf("unmarshal Data: %w", err)
		}

		return d, nil
	}

	d, err := r.New(name)
	if err != nil {
		return nil, err
	}

	if err := gob.NewDecoder(data).Decode(&d); err != nil {
		return nil, err
	}

	return d, nil
}

func (r *registry) Register(name string, newData func() Data) {
	if newData == nil {
		panic("nil factory")
	}
	r.factoriesMux.Lock()
	r.factories[name] = newData
	r.factoriesMux.Unlock()
	r.gobRegister(name, newData())
}

func (r *registry) New(name string) (Data, error) {
	r.factoriesMux.RLock()
	f, ok := r.factories[name]
	r.factoriesMux.RUnlock()
	if !ok {
		return nil, ErrUnregistered
	}
	return f(), nil
}

func (r *registry) newPointer(name string) (Data, error) {
	r.factoriesMux.RLock()
	f, ok := r.factories[name]
	r.factoriesMux.RUnlock()
	if !ok {
		return nil, ErrUnregistered
	}

	d := f()
	dv := reflect.ValueOf(d)
	v := reflect.New(dv.Type())
	v.Elem().Set(dv)

	return v.Interface(), nil
}

func (r *registry) gobRegister(name string, data Data) {
	if gobName := r.gobName(name); gobName != "" {
		gob.RegisterName(gobName, data)
		return
	}
	gob.Register(data)
}

func defaultGobName(eventName string) string {
	return "goes.event(" + eventName + ")"
}

func deref(p Data) Data {
	return reflect.ValueOf(p).Elem().Interface()
}
