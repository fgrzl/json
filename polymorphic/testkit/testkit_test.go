package testkit

import (
	"testing"

	"github.com/fgrzl/json/polymorphic"
)

type testkitPerson struct {
	Name string `json:"name"`
}

func (p *testkitPerson) GetDiscriminator() string { return "testkit-person" }

func TestShouldValidatePolymorphicRegistrationsGivenRegisteredTypes(t *testing.T) {
	// Arrange
	t.Cleanup(polymorphic.ClearRegistry)
	polymorphic.ClearRegistry()
	polymorphic.Register(func() *testkitPerson { return &testkitPerson{} })

	// Act
	TestPolymorphicRegistrations(t, map[string]any{
		"testkit-person": &testkitPerson{},
	})

	// Assert
	// The helper performs the type assertions internally; reaching this line means the
	// registration map matched the expected discriminator and concrete type.
}
