// Package testkit provides helpers for polymorphic registration tests.
package testkit

import (
	"testing"

	"github.com/fgrzl/json/polymorphic"
	"github.com/stretchr/testify/require"
)

// TestPolymorphicRegistrations tests that all provided types are properly registered
// with the polymorphic system and can be created from their discriminators.
func TestPolymorphicRegistrations(t *testing.T, types map[string]any) {
	t.Helper()

	for discriminator, instance := range types {
		t.Run(discriminator, func(t *testing.T) {
			// Polymorphic check
			created, err := polymorphic.CreateInstance(discriminator)
			require.NoError(t, err, "could not create instance for %s", discriminator)
			require.NotNil(t, created, "nil returned for %s", discriminator)
			require.IsType(t, instance, created, "instance type mismatch for %s", discriminator)
		})
	}
}
