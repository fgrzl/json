package jsonschema

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

type OrderedMap struct {
	keys []string
	data map[string]any
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{
		keys: make([]string, 0),
		data: make(map[string]any),
	}
}

func (m *OrderedMap) Set(key string, value any) {
	if _, exists := m.data[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.data[key] = value
}

func (m *OrderedMap) MarshalJSON() ([]byte, error) {
	obj := make(map[string]any, len(m.keys))
	for _, k := range m.keys {
		obj[k] = m.data[k]
	}
	return json.Marshal(obj)
}

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

func (m *OrderedMap) ToMap() map[string]any {
	return m.data
}
