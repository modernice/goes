package codec

import (
	"encoding/gob"
	"fmt"
	"io"
	"reflect"
)

// A GobRegistry allows registering data into a Registry using factory
// functions. Data that is registered via a GobRegistry will be encoded and
// decoded using the encoding/gob package.
type GobRegistry struct {
	*Registry
	gobNameFunc func(name string) (gobName string)
}

// GobOption is an option for a GobRegistry.
type GobOption func(*GobRegistry)

// GobNameFunc returns a GobOption that specifies under which name Event
// Data is registered with gob.RegisterName. The default name under which Events
// are registered is "goes.event(name)" where name is the name of the
// Event.

// GobNameFunc returns a GobOption that specifies the name under which types are
// registered in the encoding/gob package. If no custom GobNameFunc is provided,
// the format for gob names is
//	fmt.Sprintf("goes(%s)", name)
func GobNameFunc(fn func(string) string) GobOption {
	return func(r *GobRegistry) {
		r.gobNameFunc = fn
	}
}

// Gob wraps the given Registry in a GobRegistry. The GobRegistry provides a
// GobRegister function to register data using a factory function.
//
// If reg is nil, a new underlying Registry is created with New().
func Gob(reg *Registry, opts ...GobOption) *GobRegistry {
	if reg == nil {
		reg = New()
	}

	r := &GobRegistry{Registry: reg}
	for _, opt := range opts {
		opt(r)
	}

	if r.gobNameFunc == nil {
		r.gobNameFunc = defaultGobNameFunc
	}

	return r
}

func GobRegister[T any](r *GobRegistry, name string) {
	Register[T](
		r.Registry,
		name,
		gobEncoder[T]{name},
		gobDecoder[T]{name: name, makeFunc: func() (v T) { return }},
	)

	var val T
	r.gobRegister(name, val)
}

// GobRegister registers data with the given name into the underlying registry.
// makeFunc is used create instances of the data and encoding/gob will be used
// to encode and decode the data returned by makeFunc.
func (reg *GobRegistry) GobRegister(name string, makeFunc func() any) {
	gobRegisterAny(reg, name, makeFunc)
}

func gobRegisterAny(r *GobRegistry, name string, makeFunc func() any) {
	registerWithFactoryFunc[any](
		r.Registry,
		name,
		gobEncoder[any]{name},
		gobDecoder[any]{name: name, makeFunc: makeFunc},
		makeFunc,
	)

	r.gobRegister(name, makeFunc())
}

func (r *GobRegistry) gobRegister(name string, val any) {
	rv := reflect.ValueOf(val)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}

	if gobName := r.gobNameFunc(name); gobName != "" {
		gob.RegisterName(gobName, val)
		return
	}

	gob.Register(val)
}

// gobEncoder is the gob encoder for the given named data.
type gobEncoder[T any] struct{ name string }

func (enc gobEncoder[T]) Encode(w io.Writer, data T) error {
	return gob.NewEncoder(w).Encode(&data)
}

// gobDecoder is the gob decoder for the given named data.
type gobDecoder[T any] struct {
	name     string
	makeFunc func() T
}

func (dec gobDecoder[T]) Decode(r io.Reader) (T, error) {
	data := dec.makeFunc()
	return data, gob.NewDecoder(r).Decode(&data)
}

func defaultGobNameFunc(name string) string {
	return fmt.Sprintf("goes(%s)", name)
}

func deref(p any) any {
	return reflect.ValueOf(p).Elem().Interface()
}

func newPtr(data any) any {
	rval := reflect.ValueOf(data)
	nval := reflect.New(rval.Type())
	nval.Elem().Set(rval)
	return nval.Interface()
}
