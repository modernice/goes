package encoding

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/modernice/goes/event"
)

// GobEncoder encodes and decodes event.Data using the "encoding/gob" package.
type GobEncoder struct {
	mux sync.RWMutex
	new map[string]func() event.Data
}

// NewGobEncoder returns a new GobEncoder.
func NewGobEncoder() *GobEncoder {
	return &GobEncoder{
		new: make(map[string]func() event.Data),
	}
}

// Register registers data in the "gob" registry under name.
func (enc *GobEncoder) Register(name string, new func() event.Data) {
	if new == nil {
		panic("nil factory")
	}
	gobRegister(name, new())
	enc.mux.Lock()
	defer enc.mux.Unlock()
	enc.new[name] = new
}

// New returns a new zero-value instance of event.Data that has been registered
// for Events with the specified name.
func (enc *GobEncoder) New(name string) (event.Data, error) {
	if !enc.registered(name) {
		return nil, fmt.Errorf("%s: %w", name, ErrUnregisteredEvent)
	}
	enc.mux.RLock()
	defer enc.mux.RUnlock()
	return enc.new[name](), nil
}

// Encode encodes data using "encoding/gob" and writes the result into w.
func (enc *GobEncoder) Encode(w io.Writer, name string, data event.Data) error {
	if !enc.registered(name) {
		return ErrUnregisteredEvent
	}
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

	if err := gob.NewDecoder(r).Decode(&data); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("gob decode into %#v: %w", data, err)
	}

	return data, nil
}

func (enc *GobEncoder) registered(name string) bool {
	enc.mux.RLock()
	defer enc.mux.RUnlock()
	_, ok := enc.new[name]
	return ok
}

func gobRegister(name string, d event.Data) {
	gob.RegisterName(gobName(name), d)
}

func gobName(name string) string {
	return "goes.event." + name
}
