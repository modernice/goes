package codec

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

var _ Encoding = &Registry{}

// Encoding can be used to encode registered data types to and from bytes.
type Encoding interface {
	Marshal(any) ([]byte, error)
	Unmarshal([]byte, string) (any, error)
}

// Registerer is implemented by Registry to allow for registering of data types.
type Registerer interface {
	Register(string, func() any)
}

// Registry is a registry of data types. A Registry marshals and unmarshals
// event data and command payloads.
type Registry struct {
	mux               sync.RWMutex
	factories         map[string]func() any
	fallbackMarshal   func(any) ([]byte, error)
	fallbackUnmarshal func([]byte, any) error
}

// Marshaler can be implemented by data types to override the default marshaler.
type Marshaler interface {
	Marshal() ([]byte, error)
}

// Unmarshaler can be implemented by data types to override the default unmarshaler.
type Unmarshaler interface {
	Unmarshal([]byte) error
}

// Option is an option for the Registry.
type Option func(*Registry)

// Default returns an Option that configures the default marshaler and
// unmarshaler functions to be used when data types do not override the
// default marshaler and unmarshaler.
func Default(marshal func(any) ([]byte, error), unmarshal func([]byte, any) error) Option {
	if marshal == nil || unmarshal == nil {
		panic("default marshal and unmarshal functions must not be nil")
	}

	return func(r *Registry) {
		r.fallbackMarshal = marshal
		r.fallbackUnmarshal = unmarshal
	}
}

// New returns a new Registry for encoding and decoding of event data or command payloads.
func New(opts ...Option) *Registry {
	r := &Registry{
		factories:         make(map[string]func() any),
		fallbackMarshal:   json.Marshal,
		fallbackUnmarshal: json.Unmarshal,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Register registers the data type with the given name. The provided factory
// function is used to initialize the data type when needed. Call the
// package-level Register function instead to register using a generic type:
//	var r *codec.Registry
//	codec.Register[FooData](r, "foo")
func (r *Registry) Register(name string, factory func() any) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.factories[name] = factory
}

// New initializes the data type that is registered under the given name and
// returns a pointer to the data.
func (r *Registry) New(name string) (any, error) {
	r.mux.RLock()
	defer r.mux.RUnlock()
	f, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("no data type registered for name %q", name)
	}
	return f(), nil
}

// Marshal marshals the provided data to a byte slice.
func (r *Registry) Marshal(data any) ([]byte, error) {
	if m, ok := data.(Marshaler); ok {
		return m.Marshal()
	}
	return r.fallbackMarshal(data)
}

// Unmarshal unmarshals the provided bytes to the data type that is registered
// under the given name.
func (r *Registry) Unmarshal(b []byte, name string) (any, error) {
	f, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("no data type registered for name %q", name)
	}

	ptr := f()

	if m, ok := ptr.(Unmarshaler); ok {
		if err := m.Unmarshal(b); err != nil {
			return nil, err
		}
		return resolve(ptr), nil
	}

	if err := r.fallbackUnmarshal(b, ptr); err != nil {
		return resolve(ptr), err
	}

	return resolve(ptr), nil
}

// resolves a pointer to the underlying data type.
func resolve(p any) any {
	return reflect.ValueOf(p).Elem().Interface()
}

// Register registers the generic data type under the given name.
func Register[D any](r Registerer, name string) {
	r.Register(name, func() any {
		var out D
		return &out
	})
}

// Make initializes the data that is registered under the given name.
// If the data type is not the provided generic type, an error is returned.
func Make[D any](r *Registry, name string) (D, error) {
	d, err := r.New(name)
	if err != nil {
		var zero D
		return zero, err
	}

	resolved := resolve(d)
	out, ok := resolved.(D)
	if !ok {
		return out, fmt.Errorf("data is not of type %T", out)
	}

	return out, nil
}
