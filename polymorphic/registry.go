package polymorphic

import (
	"fmt"
	"sync"
)

// Polymorphic is implemented by types that expose a discriminator string
// used to identify the concrete implementation when serializing or
// deserializing polymorphic values.
type Polymorphic interface {
	GetDiscriminator() string
}

// TypeFactory creates instances of registered types.
type TypeFactory = func() any

var types sync.Map

// RegisterWithDiscriminator stores a factory function under the given
// discriminator. The factory should return a pointer to a zero-value
// instance of the concrete type. It panics if discriminator is empty.
func RegisterWithDiscriminator(discriminator string, factory TypeFactory) {
	if discriminator == "" {
		panic("discriminator must be non-empty")
	}
	types.Store(discriminator, factory)
}

// Register registers a factory for a Polymorphic type using a factory
// function that returns the concrete instance. The registration uses
// the discriminator value returned by the instance produced by the
// factory.
func Register[T Polymorphic](factory func() T) {
	instance := factory()
	discriminator := instance.GetDiscriminator()
	RegisterWithDiscriminator(discriminator, func() any { return factory() })
}

func ctor[T any]() func() *T { return func() *T { return new(T) } }

// RegisterType is a convenience helper that registers a type T where
// *T implements Polymorphic. It will panic if *T does not implement
// the Polymorphic interface.
func RegisterType[T any]() {
	factory := ctor[T]()

	// Verify that *T implements Polymorphic by creating an instance
	instance := factory()
	if _, ok := any(instance).(Polymorphic); !ok {
		panic(fmt.Sprintf("type %T does not implement Polymorphic interface", instance))
	}

	// Create a wrapper that satisfies the Register function's type constraint
	Register(func() Polymorphic {
		return any(factory()).(Polymorphic)
	})
}

// CreateInstance creates a new instance for the given discriminator.
// It returns the instance as a Polymorphic interface or an error if the
// discriminator is not registered.
func CreateInstance(discriminator string) (Polymorphic, error) {
	if factory, ok := types.Load(discriminator); ok {
		instance := factory.(TypeFactory)()
		typedInstance, ok := instance.(Polymorphic)
		if !ok {
			return nil, fmt.Errorf("invalid instance type for %q", discriminator)
		}
		return typedInstance, nil
	}
	return nil, fmt.Errorf("type %q is not registered", discriminator)
}

// LoadFactory returns the factory function registered for the
// discriminator, or an error if there is none.
func LoadFactory(discriminator string) (TypeFactory, error) {
	if factory, ok := types.Load(discriminator); ok {
		typedFactory, ok := factory.(TypeFactory)
		if !ok {
			return nil, fmt.Errorf("invalid factory type for %q", discriminator)
		}
		return typedFactory, nil
	}
	return nil, fmt.Errorf("type %q is not registered", discriminator)
}

// ClearRegistry removes all registered factories. Useful in tests to
// reset global state.
func ClearRegistry() {
	types.Range(func(k, v any) bool {
		types.Delete(k)
		return true
	})
}
