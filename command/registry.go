package command

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
	// ErrUnregistered is returned when trying to create Command Payload which
	// hasn't been registered in a Registry.
	ErrUnregistered = errors.New("unregistered Command")
)

// An Encoder encodes & decodes Command Payloads.
type Encoder interface {
	// Encode encodes pl and writes the result into w.
	Encode(w io.Writer, name string, pl Payload) error

	// Decode decodes the Payload in r based on the specified Command name.
	Decode(name string, r io.Reader) (Payload, error)
}

// A Registry is an Encoder that also allows to register new Commands into it
// and instantiate Payload from Command names.
type Registry interface {
	Encoder

	// Register registers a Payload factory for Commands with the given name.
	Register(name string, newPayload func() Payload)

	// New returns new Payload for a Command with the given name.
	New(name string) (Payload, error)
}

// A Marshaler knows how to encode itself into a byte slice. Payloads can
// implement Marshaler to override the default marshaling strategy which is by
// default using encoding/gob to encode the Payload.
//
// Example using encoding/json instead of encoding/gob:
//	type FooCommand struct {
//		A string
//		B int
//		C bool
//	}
//
//	func (cmd FooCommand) MarshalCommand() ([]byte, error) {
//		return json.Marshal(cmd)
//	}
type Marshaler interface {
	MarshalCommand() ([]byte, error)
}

// An Unmarshaler knows how to decode itself from a bytes. Payloads can
// implement Unmarshaler to override the default unmarshaling strategy which is
// by default using encoding/gob to decode the Event Data.
//
// Example using encoding/json instead of encoding/gob:
//	type FooCommand struct {
//		A string
//		B int
//		C bool
//	}
//
//	func (cmd *FooCommand) UnmarshalCommand(data []byte) error {
//		return json.Unmarshal(data, e)
//	}
type Unmarshaler interface {
	UnmarshalCommand([]byte) error
}

// A RegistryOption configures a Registry.
type RegistryOption func(*registry)

type registry struct {
	gobName         func(string) string
	factoriesMux    sync.RWMutex
	factories       map[string]func() Payload
	unmarshalersMux sync.RWMutex
	unmarshalers    map[string]bool
	marshalersMux   sync.RWMutex
	marshalers      map[string]bool
}

// GobNameFunc returns a RegistryOption that specifies under which name Payload
// is registered with gob.RegisterName. The default name under which Commands
// are registered is "goes.command(commandName)" where commandName is the name
// of the Command.
func GobNameFunc(fn func(string) string) RegistryOption {
	return func(r *registry) {
		r.gobName = fn
	}
}

// NewRegistry returns a new Command Registry.
func NewRegistry(opts ...RegistryOption) Registry {
	reg := &registry{
		factories:    make(map[string]func() Payload),
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

func (r *registry) Encode(w io.Writer, name string, load Payload) error {
	r.marshalersMux.RLock()
	isMarshaler, checkedMarshal := r.marshalers[name]
	r.marshalersMux.RUnlock()

	if !checkedMarshal {
		m, ok := load.(Marshaler)
		r.marshalersMux.Lock()
		r.marshalers[name] = ok
		r.marshalersMux.Unlock()

		if ok {
			b, err := m.MarshalCommand()
			if err != nil {
				return fmt.Errorf("marshal Payload: %w", err)
			}

			if _, err := w.Write(b); err != nil {
				return err
			}

			return nil
		}
	}

	if isMarshaler {
		m := load.(Marshaler)
		b, err := m.MarshalCommand()
		if err != nil {
			return fmt.Errorf("marshal Payload: %w", err)
		}

		if _, err := w.Write(b); err != nil {
			return err
		}
	}

	if err := gob.NewEncoder(w).Encode(&load); err != nil {
		return err
	}

	return nil
}

func (r *registry) Decode(name string, load io.Reader) (Payload, error) {
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
			b, err := io.ReadAll(load)
			if err != nil {
				return nil, fmt.Errorf("read Payload: %w", err)
			}

			if err := m.UnmarshalCommand(b); err != nil {
				return nil, fmt.Errorf("unmarshal Payload: %w", err)
			}

			return deref(p), nil
		}
	}

	if isUnmarshaler {
		pl, err := r.New(name)
		if err != nil {
			return nil, err
		}
		m := pl.(Unmarshaler)

		b, err := io.ReadAll(load)
		if err != nil {
			return nil, fmt.Errorf("read Payload: %w", err)
		}

		if err := m.UnmarshalCommand(b); err != nil {
			return nil, fmt.Errorf("unmarshal Payload: %w", err)
		}

		return pl, nil
	}

	pl, err := r.New(name)
	if err != nil {
		return nil, err
	}

	if err := gob.NewDecoder(load).Decode(&pl); err != nil {
		return nil, err
	}

	return pl, nil
}

func (r *registry) Register(name string, newPayload func() Payload) {
	if newPayload == nil {
		panic("nil factory")
	}
	r.factoriesMux.Lock()
	r.factories[name] = newPayload
	r.factoriesMux.Unlock()
	r.gobRegister(name, newPayload())
}

func (r *registry) New(name string) (Payload, error) {
	r.factoriesMux.RLock()
	f, ok := r.factories[name]
	r.factoriesMux.RUnlock()
	if !ok {
		return nil, ErrUnregistered
	}
	return f(), nil
}

func (r *registry) newPointer(name string) (Payload, error) {
	r.factoriesMux.RLock()
	f, ok := r.factories[name]
	r.factoriesMux.RUnlock()
	if !ok {
		return nil, ErrUnregistered
	}

	load := f()
	lv := reflect.ValueOf(load)
	v := reflect.New(lv.Type())
	v.Elem().Set(lv)

	return v.Interface(), nil
}

func (r *registry) gobRegister(name string, load Payload) {
	if gobName := r.gobName(name); gobName != "" {
		gob.RegisterName(gobName, load)
		return
	}
	gob.Register(load)
}

func defaultGobName(commandName string) string {
	return "goes.command(" + commandName + ")"
}

func deref(p Payload) Payload {
	return reflect.ValueOf(p).Elem().Interface()
}
