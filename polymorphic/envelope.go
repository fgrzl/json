package polymorphic

import (
	"encoding/json"
	"fmt"
)

// NewEnvelope creates an Envelope wrapping a polymorphic object. The
// Envelope contains the discriminator value and the content to be
// marshaled. The discriminator is obtained by calling obj.GetDiscriminator().
// It panics if obj is nil.
func NewEnvelope(obj Polymorphic) *Envelope {
	return &Envelope{
		Discriminator: obj.GetDiscriminator(),
		Content:       obj,
	}
}

// MarshalPolymorphicJSON is a helper that marshals a Polymorphic object
// into the envelope format used by this package.
func MarshalPolymorphicJSON(obj Polymorphic) ([]byte, error) {
	wrapper := NewEnvelope(obj)
	return json.Marshal(wrapper)
}

// UnmarshalPolymorphicJSON unmarshals data into an Envelope and resolves
// the contained polymorphic value using the registered factory for the
// discriminator value.
func UnmarshalPolymorphicJSON(data []byte) (*Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal polymorphic JSON: %w", err)
	}
	return &envelope, nil
}

// Envelope represents a marshaled polymorphic value. The `$type` field
// contains the discriminator and `Content` holds the concrete value
// after unmarshaling.
type Envelope struct {
	Discriminator string `json:"$type"`
	Content       any    `json:"-"`
}

// MarshalJSON implements json.Marshaler for Envelope. It validates that
// the discriminator is registered and marshals the content into a small
// envelope object containing `$type` and `content`.
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

// UnmarshalJSON implements json.Unmarshaler for Envelope. It expects a
// JSON object with a non-empty `$type` discriminator and a `content` field.
// The content must be present and non-null; null or missing content
// returns an error. The content is unmarshaled into a concrete instance
// returned by the registered factory for that discriminator.
func (e *Envelope) UnmarshalJSON(data []byte) error {
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
	if e.Discriminator == "" {
		return fmt.Errorf("empty $type discriminator")
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
	if string(rawContent) == "null" {
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
