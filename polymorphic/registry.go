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

var (
	registryMu   sync.RWMutex
	types        = make(map[string]TypeFactory)
	defaultTypes = make(map[string]TypeFactory)
)

func registerWithDiscriminator(discriminator string, factory TypeFactory, isDefault bool) {
	if discriminator == "" {
		panic("discriminator must be non-empty")
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	types[discriminator] = factory
	if isDefault {
		defaultTypes[discriminator] = factory
	}
}

// RegisterWithDiscriminator stores a factory function under the given
// discriminator. The factory should return a pointer to a zero-value
// instance of the concrete type. It panics if discriminator is empty.
func RegisterWithDiscriminator(discriminator string, factory TypeFactory) {
	registerWithDiscriminator(discriminator, factory, false)
}

// Register registers a factory for a Polymorphic type using a factory
// function that returns the concrete instance. The registration uses
// the discriminator value returned by the instance produced by the
// factory.
func Register[T Polymorphic](factory func() T) {
	instance := factory()
	discriminator := instance.GetDiscriminator()
	registerWithDiscriminator(discriminator, func() any { return factory() }, false)
}

func registerDefault[T Polymorphic](factory func() T) {
	instance := factory()
	discriminator := instance.GetDiscriminator()
	registerWithDiscriminator(discriminator, func() any { return factory() }, true)
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

func registerDefaultType[T any]() {
	factory := ctor[T]()

	instance := factory()
	if _, ok := any(instance).(Polymorphic); !ok {
		panic(fmt.Sprintf("type %T does not implement Polymorphic interface", instance))
	}

	registerDefault(func() Polymorphic {
		return any(factory()).(Polymorphic)
	})
}

// CreateInstance creates a new instance for the given discriminator.
// It returns the instance as a Polymorphic interface or an error if the
// discriminator is not registered.
func CreateInstance(discriminator string) (Polymorphic, error) {
	factory, err := LoadFactory(discriminator)
	if err != nil {
		return nil, err
	}

	instance := factory()
	typedInstance, ok := instance.(Polymorphic)
	if !ok {
		return nil, fmt.Errorf("invalid instance type for %q", discriminator)
	}
	return typedInstance, nil
}

// LoadFactory returns the factory function registered for the
// discriminator, or an error if there is none.
func LoadFactory(discriminator string) (TypeFactory, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	if factory, ok := types[discriminator]; ok {
		return factory, nil
	}
	return nil, fmt.Errorf("type %q is not registered", discriminator)
}

func cloneFactories(source map[string]TypeFactory) map[string]TypeFactory {
	cloned := make(map[string]TypeFactory, len(source))
	for discriminator, factory := range source {
		cloned[discriminator] = factory
	}
	return cloned
}

// ClearRegistry resets the registry to the package default factories.
// Useful in tests to remove custom registrations without leaving the
// package in a partially uninitialized state.
func ClearRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()

	types = cloneFactories(defaultTypes)
}
