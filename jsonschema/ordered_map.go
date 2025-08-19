package jsonschema

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// OrderedMap is a simple map that preserves insertion order of keys.
// It provides JSON and YAML marshaling that respects the insertion
// order, which is useful for predictable output in tests and APIs.
type OrderedMap struct {
	keys []string
	data map[string]any
}

// NewOrderedMap creates an empty OrderedMap.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{
		keys: make([]string, 0),
		data: make(map[string]any),
	}
}

// Set inserts or updates a key while preserving insertion order for new keys.
func (m *OrderedMap) Set(key string, value any) {
	if _, exists := m.data[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.data[key] = value
}

// MarshalJSON marshals the OrderedMap to JSON using the insertion order.
func (m *OrderedMap) MarshalJSON() ([]byte, error) {
	obj := make(map[string]any, len(m.keys))
	for _, k := range m.keys {
		obj[k] = m.data[k]
	}
	return json.Marshal(obj)
}

// MarshalYAML marshals the OrderedMap to a YAML node preserving order.
func (m *OrderedMap) MarshalYAML() (any, error) {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, k := range m.keys {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: k,
		}
		valNode := &yaml.Node{}
		if err := valNode.Encode(m.data[k]); err != nil {
			return nil, err
		}
		node.Content = append(node.Content, keyNode, valNode)
	}
	return node, nil
}

// ToMap returns the underlying map storage. Note: the returned map is
// the internal storage and mutating it will affect the OrderedMap.
func (m *OrderedMap) ToMap() map[string]any {
	return m.data
}
