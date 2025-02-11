package polymorphic_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/fgrzl/json/polymorphic"
	"github.com/stretchr/testify/assert"
)

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func resetRegistry() {
	polymorphicTypes := new(sync.Map)
	polymorphicTypes.Range(func(key, value any) bool {
		polymorphicTypes.Delete(key)
		return true
	})
}

func TestRegisterAndLoadFactory(t *testing.T) {
	// Arrange
	resetRegistry()
	polymorphic.Register("Person", func() any { return &Person{} })

	// Act
	factory, err := polymorphic.LoadFactory("Person")

	// Assert
	assert.NoError(t, err, "Loading registered factory should not produce an error")
	assert.NotNil(t, factory, "Loaded factory should not be nil")
}

func TestLoadFactory_UnregisteredType(t *testing.T) {
	// Act
	factory, err := polymorphic.LoadFactory("UnknownType")

	// Assert
	assert.Error(t, err, "Loading an unregistered type should return an error")
	assert.Nil(t, factory, "Factory should be nil for an unregistered type")
	assert.ErrorContains(t, err, "type \"UnknownType\" is not registered", "Error message should indicate unknown type")
}

func TestFactoryCreatesInstance(t *testing.T) {
	// Arrange
	resetRegistry()
	polymorphic.Register("Person", func() any { return &Person{} })

	// Act
	factory, _ := polymorphic.LoadFactory("Person")
	instance := factory()

	// Assert
	_, ok := instance.(*Person)
	assert.True(t, ok, "Factory should return an instance of *Person")
}

func TestMarshalPolymorphicJSON(t *testing.T) {
	// Arrange
	resetRegistry()
	polymorphic.Register("Person", func() any { return &Person{} })
	person := &Person{Name: "Alice", Age: 30}

	// Act
	jsonBytes, err := polymorphic.MarshalPolymorphicJSON("Person", person)

	// Assert
	assert.NoError(t, err, "Marshaling should not produce an error")
	expectedJSON := `{"$type":"Person","content":{"name":"Alice","age":30}}`
	assert.JSONEq(t, expectedJSON, string(jsonBytes), "Marshaled JSON should match expected output")
}

func TestMarshal_UnregisteredTypeFails(t *testing.T) {
	// Arrange
	resetRegistry()
	obj := &Person{Name: "Bob", Age: 40}

	// Act
	_, err := polymorphic.MarshalPolymorphicJSON("UnregisteredPerson", obj)

	// Assert
	assert.Error(t, err, "Marshaling should fail for an unregistered type")
	assert.ErrorContains(t, err, "type \"UnregisteredPerson\" is not registered")
}

func TestUnmarshalPolymorphicJSON(t *testing.T) {
	// Arrange
	resetRegistry()
	polymorphic.Register("Person", func() any { return &Person{} })
	jsonStr := `{"$type":"Person","content":{"name":"Alice","age":30}}`

	// Act
	var content polymorphic.Envelope
	err := json.Unmarshal([]byte(jsonStr), &content)

	// Assert
	assert.NoError(t, err, "Unmarshaling should not produce an error")
	assert.Equal(t, "Person", content.Discriminator, "Discriminator should match expected value")

	// Extract the content
	personObj, ok := content.Content.(*Person)

	// Assert that content is correctly deserialized
	assert.True(t, ok, "Content should be of type *Person")
	assert.Equal(t, "Alice", personObj.Name, "Person name should be 'Alice'")
	assert.Equal(t, 30, personObj.Age, "Person age should be 30")
}

func TestUnmarshal_UnknownTypeFails(t *testing.T) {
	// Arrange
	jsonStr := `{"$type":"UnknownType","content":{"key":"value"}}`

	// Act
	var content polymorphic.Envelope
	err := json.Unmarshal([]byte(jsonStr), &content)

	// Assert
	assert.Error(t, err, "Unmarshaling should fail for an unknown type")
	assert.ErrorContains(t, err, "type \"UnknownType\" is not registered", "Error message should indicate unknown type")
}

func TestUnmarshal_MissingContentFails(t *testing.T) {
	// Arrange
	resetRegistry()
	polymorphic.Register("Person", func() any { return &Person{} })
	jsonStr := `{"$type":"Person"}`

	// Act
	var content polymorphic.Envelope
	err := json.Unmarshal([]byte(jsonStr), &content)

	// Assert
	assert.Error(t, err, "Unmarshaling should fail if content is missing")
	assert.ErrorContains(t, err, "missing content for type: \"Person\"", "Error should indicate missing content")
}

func TestPolymorphicContent_MultipleTypes(t *testing.T) {
	// Arrange: Define and register multiple types
	resetRegistry()
	type Car struct {
		Make  string `json:"make"`
		Model string `json:"model"`
	}
	polymorphic.Register("Person", func() any { return &Person{} })
	polymorphic.Register("Car", func() any { return &Car{} })

	person := &Person{Name: "Alice", Age: 30}
	car := &Car{Make: "Tesla", Model: "Model S"}

	// Act: Serialize both
	personJSON, errPerson := polymorphic.MarshalPolymorphicJSON("Person", person)
	carJSON, errCar := polymorphic.MarshalPolymorphicJSON("Car", car)

	// Assert: No errors
	assert.NoError(t, errPerson, "Marshaling Person should not produce an error")
	assert.NoError(t, errCar, "Marshaling Car should not produce an error")

	// Act: Deserialize both
	var personContent, carContent polymorphic.Envelope
	errPersonUnmarshal := json.Unmarshal(personJSON, &personContent)
	errCarUnmarshal := json.Unmarshal(carJSON, &carContent)

	// Assert: No errors
	assert.NoError(t, errPersonUnmarshal, "Unmarshaling Person should not produce an error")
	assert.NoError(t, errCarUnmarshal, "Unmarshaling Car should not produce an error")

	// Assert: Correct types
	assert.Equal(t, "Person", personContent.Discriminator, "Person discriminator should match")
	assert.Equal(t, "Car", carContent.Discriminator, "Car discriminator should match")

	// Assert: Correct values
	personObj, okPerson := personContent.Content.(*Person)
	carObj, okCar := carContent.Content.(*Car)

	assert.True(t, okPerson, "Content should be of type *Person")
	assert.Equal(t, "Alice", personObj.Name, "Person name should be 'Alice'")
	assert.Equal(t, 30, personObj.Age, "Person age should be 30")

	assert.True(t, okCar, "Content should be of type *Car")
	assert.Equal(t, "Tesla", carObj.Make, "Car make should be 'Tesla'")
	assert.Equal(t, "Model S", carObj.Model, "Car model should be 'Model S'")
}
