package encoding

import (
	"encoding/gob"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
)

var (
	gobRegisteredMux      sync.RWMutex
	gobRegisteredCommands = make(map[string]bool)
)

type GobEncoder struct {
	mux    sync.RWMutex
	config map[string]reflect.Type
}

func NewGobEncoder() *GobEncoder {
	return &GobEncoder{
		config: make(map[string]reflect.Type),
	}
}

func (enc *GobEncoder) Encode(w io.Writer, pl command.Payload) error {
	if err := gob.NewEncoder(w).Encode(&pl); err != nil {
		return fmt.Errorf("gob encode %v: %w", pl, err)
	}
	return nil
}

func (enc *GobEncoder) Decode(name string, r io.Reader) (command.Payload, error) {
	pl, err := enc.New(name)
	if err != nil {
		return nil, err
	}

	enc.mux.RLock()
	defer enc.mux.RUnlock()

	if err := gob.NewDecoder(r).Decode(&pl); err != nil {
		return nil, fmt.Errorf("gob decode %v: %w", pl, err)
	}

	return pl, nil
}

func (enc *GobEncoder) Register(name string, pl command.Payload) {
	typ := reflect.TypeOf(pl)
	if !gobRegistered(name) {
		gobRegister(name, pl)
	}
	enc.mux.Lock()
	defer enc.mux.Unlock()
	enc.config[name] = typ
}

func (enc *GobEncoder) RegisterMany(m map[string]command.Payload) {
	for name, pl := range m {
		enc.Register(name, pl)
	}
}

func (enc *GobEncoder) New(name string) (command.Payload, error) {
	if !enc.registered(name) {
		return nil, fmt.Errorf("%s: %w", name, ErrUnregisteredCommand)
	}
	enc.mux.RLock()
	defer enc.mux.RUnlock()
	typ := enc.config[name]
	return reflect.New(typ).Elem().Interface().(command.Payload), nil
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
	return gobRegisteredCommands[name]
}

func gobRegister(name string, data event.Data) {
	gobRegisteredMux.Lock()
	defer gobRegisteredMux.Unlock()
	gob.RegisterName(name, data)
	gobRegisteredCommands[name] = true
}
