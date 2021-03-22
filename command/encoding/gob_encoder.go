package encoding

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/modernice/goes/command"
)

// GobEncoder encodes Command Payloads using the "encoding/gob" package.
type GobEncoder struct {
	mux sync.RWMutex
	new map[string]func() command.Payload
}

// NewGobEncoder returns a new GobEncoder.
func NewGobEncoder() *GobEncoder {
	return &GobEncoder{
		new: make(map[string]func() command.Payload),
	}
}

// Encode encodes the given Payload and writes the result into w.
func (enc *GobEncoder) Encode(w io.Writer, name string, pl command.Payload) error {
	if !enc.registered(name) {
		return ErrUnregisteredCommand
	}
	if err := gob.NewEncoder(w).Encode(&pl); err != nil {
		return fmt.Errorf("gob encode %v: %w", pl, err)
	}
	return nil
}

// Decode decodes and returns the Payload in r.
func (enc *GobEncoder) Decode(name string, r io.Reader) (command.Payload, error) {
	pl, err := enc.New(name)
	if err != nil {
		return nil, err
	}

	if err := gob.NewDecoder(r).Decode(&pl); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("gob decode %#v: %w", pl, err)
	}

	return pl, nil
}

// Register registers a Payload factory into the Encoder.
func (enc *GobEncoder) Register(name string, new func() command.Payload) {
	if new == nil {
		panic("nil factory")
	}
	gobRegister(name, new())
	enc.mux.Lock()
	defer enc.mux.Unlock()
	enc.new[name] = new
}

// New makes and returns a fresh Payload for a Command with the given name.
func (enc *GobEncoder) New(name string) (command.Payload, error) {
	if !enc.registered(name) {
		return nil, fmt.Errorf("%s: %w", name, ErrUnregisteredCommand)
	}
	enc.mux.RLock()
	defer enc.mux.RUnlock()
	return enc.new[name](), nil
}

func (enc *GobEncoder) registered(name string) bool {
	enc.mux.RLock()
	defer enc.mux.RUnlock()
	_, ok := enc.new[name]
	return ok
}

func gobRegister(name string, load command.Payload) {
	gob.RegisterName(gobName(name), load)
}

func gobName(name string) string {
	return "goes.cmd." + name
}
