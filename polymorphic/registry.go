package polymorphic

import (
	"fmt"
	"sync"
)

// Polymorphic ensures types implement GetDiscriminator().
type Polymorphic interface {
	GetDiscriminator() string
}

// TypeFactory creates instances of registered types.
type TypeFactory = func() any

var types sync.Map

// Register stores a factory function with its discriminator.
func RegisterWithDiscriminator(discriminator string, factory TypeFactory) {
	types.Store(discriminator, factory)
}

func Register[T Polymorphic](factory func() T) {
	// Ensure T is a pointer at compile-time
	var instance T

	// Get discriminator
	discriminator := instance.GetDiscriminator()

	RegisterWithDiscriminator(discriminator, func() any { return factory() })
}

// CreateInstance creates an instance based on the discriminator.
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

func ClearRegistry() {
	types.Range(func(k, v any) bool {
		types.Delete(k)
		return true
	})
}
