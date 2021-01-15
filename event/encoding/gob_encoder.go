package encoding

import (
	"encoding/gob"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/modernice/goes/event"
)

// use globals because "gob" registers types globally and we have to ensure
// gob.RegisterName() is called only once for every name.
var (
	gobRegisteredMux    sync.RWMutex
	gobRegisteredEvents = make(map[string]bool)
)

// GobEncoder encodes and decodes event.Data using the "encoding/gob" package.
type GobEncoder struct {
	mux    sync.RWMutex
	config map[string]reflect.Type
}

// NewGobEncoder returns a new GobEncoder.
func NewGobEncoder() *GobEncoder {
	return &GobEncoder{
		config: make(map[string]reflect.Type),
	}
}

// Register registers data in the "gob" registry under name.
func (enc *GobEncoder) Register(name string, data event.Data) {
	typ := reflect.TypeOf(data)
	if !gobRegistered(name) {
		gobRegister(name, data)
	}
	enc.mux.Lock()
	defer enc.mux.Unlock()
	enc.config[name] = typ
}

// RegisterMany registers multiple event.Data in one go.
func (enc *GobEncoder) RegisterMany(m map[string]event.Data) {
	for name, data := range m {
		enc.Register(name, data)
	}
}

// New returns a new zero-value instance of event.Data that has been registered
// for Events with the specified name.
func (enc *GobEncoder) New(name string) (event.Data, error) {
	if !enc.registered(name) {
		return nil, fmt.Errorf("%s: %w", name, ErrUnregisteredEvent)
	}
	enc.mux.RLock()
	defer enc.mux.RUnlock()
	typ := enc.config[name]
	return reflect.New(typ).Elem().Interface().(event.Data), nil
}

// Encode encodes data using "encoding/gob" and writes the result into w.
func (enc *GobEncoder) Encode(w io.Writer, data event.Data) error {
	if err := gob.NewEncoder(w).Encode(&data); err != nil {
		return fmt.Errorf("gob encode %v: %w", data, err)
	}
	return nil
}

// Decode decodes the event.Data in r using "encoding/gob" and the provided
// Event name. If name hasn't been registered in enc, Decode() returns
// ErrUnregisteredEvent.
func (enc *GobEncoder) Decode(name string, r io.Reader) (event.Data, error) {
	data, err := enc.New(name)
	if err != nil {
		return nil, err
	}

	enc.mux.RLock()
	defer enc.mux.RUnlock()

	if err := gob.NewDecoder(r).Decode(&data); err != nil {
		return nil, fmt.Errorf("gob decode %v: %w", data, err)
	}

	return data, nil
}

func (enc *GobEncoder) registered(name string) bool {
	enc.mux.RLock()
	defer enc.mux.RUnlock()
	_, ok := enc.config[name]
	return ok && gobRegistered(name)
}

func gobRegistered(name string) bool {
	gobRegisteredMux.RLock()
	defer gobRegisteredMux.RUnlock()
	return gobRegisteredEvents[name]
}

func gobRegister(name string, data event.Data) {
	gobRegisteredMux.Lock()
	defer gobRegisteredMux.Unlock()
	gob.RegisterName(name, data)
	gobRegisteredEvents[name] = true
}
