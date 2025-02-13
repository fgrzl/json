// Package polymorphic provides functionality for registering and handling
// polymorphic types in JSON serialization and deserialization. It allows
// types to be registered with a discriminator string and provides methods
// for marshaling and unmarshaling JSON data with type information.
//
// TypeFactory is a function type that creates instances of registered types.
//
// Register registers a type with a given discriminator string and factory function.
//
// LoadFactory retrieves a factory function for a given discriminator string.
//
// NewEnvelope creates a new Envelope with the given discriminator and content.
//
// MarshalPolymorphicJSON marshals an object with a discriminator into JSON.
//
// Envelope is a struct that holds type information and raw JSON data.
// It implements the json.Marshaler and json.Unmarshaler interfaces.
//
// MarshalJSON marshals the Envelope into JSON, ensuring the type is registered
// and the content is properly serialized.
//
// UnmarshalJSON unmarshals JSON data into the Envelope, extracting the
// discriminator and content, and deserializing the content into the appropriate
// type using the registered factory function.
package polymorphic

import (
	"encoding/json"
	"fmt"
	"sync"
)

// TypeFactory creates instances of registered types
type TypeFactory func() any

var types sync.Map

// Register a type with a discriminator
func Register(discriminator string, factory TypeFactory) {
	types.Store(discriminator, factory)
}

// Retrieve a factory function
func LoadFactory(discriminator string) (TypeFactory, error) {
	if value, ok := types.Load(discriminator); ok {
		if factory, ok := value.(TypeFactory); ok {
			return factory, nil
		}
	}
	return nil, fmt.Errorf("type %q is not registered", discriminator)
}

// Create a new Envelope with a discriminator and content
func NewEnvelope(discriminator string, obj any) *Envelope {
	return &Envelope{
		Discriminator: discriminator,
		Content:       obj,
	}
}

// Marshal an object with a discriminator
func MarshalPolymorphicJSON(discriminator string, obj any) ([]byte, error) {
	// Wrap the object in Envelope and let MarshalJSON handle validation
	wrapper := NewEnvelope(discriminator, obj)
	return json.Marshal(wrapper)
}

// Envelope holds type info and raw JSON data
type Envelope struct {
	Discriminator string `json:"$type"`
	Content       any    `json:"-"`
}

// Implements json.Marshaler
func (e *Envelope) MarshalJSON() ([]byte, error) {

	// Ensure type is registered
	_, err := LoadFactory(e.Discriminator)
	if err != nil {
		return nil, err
	}

	// Marshal the content
	contentBytes, err := json.Marshal(e.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}

	// Use a map to avoid an extra struct allocation
	return json.Marshal(map[string]any{
		"$type":   e.Discriminator,
		"content": json.RawMessage(contentBytes),
	})
}

// Implements json.Unmarshaler
// UnmarshalJSON is a custom JSON unmarshaler for the Envelope type.
// It extracts the raw JSON data into a map and looks for a discriminator
// field named "$type" to determine the concrete type of the content.
// The content is then unmarshaled into the appropriate type using a factory
// function registered for the discriminator. If any required fields are missing
// or if unmarshaling fails, an error is returned.
//
// Parameters:
// - data: The JSON-encoded data to be unmarshaled.
//
// Returns:
// - error: An error if unmarshaling fails or if required fields are missing.
func (e *Envelope) UnmarshalJSON(data []byte) error {
	// Extract raw data without assuming structure
	aux := make(map[string]json.RawMessage)

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal envelope: %w", err)
	}

	// Extract discriminator
	rawType, found := aux["$type"]
	if !found {
		return fmt.Errorf("missing $type field in envelope")
	}
	if err := json.Unmarshal(rawType, &e.Discriminator); err != nil {
		return fmt.Errorf("invalid $type format: %w", err)
	}

	// Ensure type is registered
	factory, err := LoadFactory(e.Discriminator)
	if err != nil {
		return err
	}

	// Extract content
	rawContent, found := aux["content"]
	if !found || len(rawContent) == 0 {
		return fmt.Errorf("missing content for type: %q", e.Discriminator)
	}

	// Deserialize into the correct type
	instance := factory()
	if err := json.Unmarshal(rawContent, instance); err != nil {
		return fmt.Errorf("failed to unmarshal content for %q: %w", e.Discriminator, err)
	}

	e.Content = instance
	return nil
}
