package polymorphic

import (
	"fmt"
	"sync"
)

// Discriminator ensures types implement GetDiscriminator().
type Discriminator interface {
	GetDiscriminator() string
}

// TypeFactory creates instances of registered types.
type TypeFactory func() any

var types sync.Map

// Register stores a factory function with its discriminator.
func Register(discriminator string, factory TypeFactory) {
	types.Store(discriminator, factory)
}

func RegisterType[T Discriminator]() {
	// Ensure T is a pointer at compile-time
	var instance T

	// Get discriminator
	discriminator := instance.GetDiscriminator()

	// Factory function that guarantees a non-nil instance
	factory := func() any {
		return new(T)
	}

	Register(discriminator, factory)
}

// CreateInstance creates an instance based on the discriminator.
func CreateInstance(discriminator string) (any, error) {
	if factory, ok := types.Load(discriminator); ok {
		return factory.(TypeFactory)(), nil
	}
	return nil, fmt.Errorf("type %q is not registered", discriminator)
}

// LoadFactory retrieves the factory function associated with the discriminator.
func LoadFactory(discriminator string) (TypeFactory, error) {
	if factory, ok := types.Load(discriminator); ok {
		return factory.(TypeFactory), nil
	}
	return nil, fmt.Errorf("type %q is not registered", discriminator)
}
