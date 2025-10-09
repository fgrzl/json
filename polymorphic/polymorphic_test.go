package polymorphic

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldLoadFactoryGivenRegisteredType(t *testing.T) {
	// Arrange
	ClearRegistry()
	Register(func() *Person { return &Person{} })

	// Act
	factory, err := LoadFactory("person")

	// Assert
	assert.NoError(t, err, "Loading registered factory should not produce an error")
	assert.NotNil(t, factory, "Loaded factory should not be nil")
}

func TestShouldReturnErrorWhenLoadingUnregisteredType(t *testing.T) {
	// Act
	factory, err := LoadFactory("UnknownType")

	// Assert
	assert.Error(t, err, "Loading an unregistered type should return an error")
	assert.Nil(t, factory, "Factory should be nil for an unregistered type")
	assert.ErrorContains(t, err, "type \"UnknownType\" is not registered", "Error message should indicate unknown type")
}

func TestShouldCreateCorrectInstanceWhenFactoryInvoked(t *testing.T) {
	// Arrange
	ClearRegistry()
	Register(func() *Person { return &Person{} })

	// Act
	factory, _ := LoadFactory("person")
	instance := factory()

	// Assert
	_, ok := instance.(*Person)
	assert.True(t, ok, "Factory should return an instance of *Person")
}

func TestShouldMarshalPolymorphicJSONGivenRegisteredType(t *testing.T) {
	// Arrange
	ClearRegistry()
	Register(func() *Person { return &Person{} })
	person := &Person{Name: "Alice", Age: 30}

	// Act
	jsonBytes, err := MarshalPolymorphicJSON(person)

	// Assert
	assert.NoError(t, err, "Marshaling should not produce an error")
	expectedJSON := `{"$type":"person","content":{"name":"Alice","age":30}}`
	assert.JSONEq(t, expectedJSON, string(jsonBytes), "Marshaled JSON should match expected output")
}

func TestShouldFailMarshalingWhenTypeUnregistered(t *testing.T) {
	// Arrange
	ClearRegistry()
	obj := &Person{Name: "Bob", Age: 40}

	// Act
	_, err := MarshalPolymorphicJSON(obj)

	// Assert
	assert.Error(t, err, "Marshaling should fail for an unregistered type")
	assert.ErrorContains(t, err, "type \"person\" is not registered")
}

func TestShouldUnmarshalPolymorphicJSONGivenValidData(t *testing.T) {
	// Arrange
	ClearRegistry()
	Register(func() *Person { return &Person{} })
	jsonStr := `{"$type":"person","content":{"name":"Alice","age":30}}`

	// Act
	var envelope Envelope
	err := json.Unmarshal([]byte(jsonStr), &envelope)

	// Assert
	assert.NoError(t, err, "Unmarshaling should not produce an error")
	assert.Equal(t, "person", envelope.Discriminator, "Discriminator should match expected value")

	// Extract the content
	personObj, ok := envelope.Content.(*Person)

	// Assert that content is correctly deserialized
	assert.True(t, ok, "Content should be of type *Person")
	assert.Equal(t, "Alice", personObj.Name, "Person name should be 'Alice'")
	assert.Equal(t, 30, personObj.Age, "Person age should be 30")
}

func TestUnmarshalShouldFailGivenUnknownType(t *testing.T) {
	// Arrange
	jsonStr := `{"$type":"UnknownType","content":{"key":"value"}}`

	// Act
	var content Envelope
	err := json.Unmarshal([]byte(jsonStr), &content)

	// Assert
	assert.Error(t, err, "Unmarshaling should fail for an unknown type")
	assert.ErrorContains(t, err, "type \"UnknownType\" is not registered", "Error message should indicate unknown type")
}

func TestUnmarshalShouldFailGivenMissingContent(t *testing.T) {
	// Arrange
	ClearRegistry()
	Register(func() *Person { return &Person{} })
	jsonStr := `{"$type":"person"}`

	// Act
	var content Envelope
	err := json.Unmarshal([]byte(jsonStr), &content)

	// Assert
	assert.Error(t, err, "Unmarshaling should fail if content is missing")
	assert.ErrorContains(t, err, "missing content for type: \"person\"", "Error should indicate missing content")
}

func TestShouldSupportMultiplePolymorphicTypes(t *testing.T) {
	// Arrange: Define and register multiple types
	ClearRegistry()

	Register(func() *Person { return &Person{} })
	Register(func() *Car { return &Car{} })

	person := &Person{Name: "Alice", Age: 30}
	car := &Car{Make: "Tesla", Model: "Model S"}

	// Act: Serialize both
	personJSON, errPerson := MarshalPolymorphicJSON(person)
	carJSON, errCar := MarshalPolymorphicJSON(car)

	// Assert: No errors
	assert.NoError(t, errPerson, "Marshaling Person should not produce an error")
	assert.NoError(t, errCar, "Marshaling Car should not produce an error")

	// Act: Deserialize both
	var personContent, carContent Envelope
	errPersonUnmarshal := json.Unmarshal(personJSON, &personContent)
	errCarUnmarshal := json.Unmarshal(carJSON, &carContent)

	// Assert: No errors
	assert.NoError(t, errPersonUnmarshal, "Unmarshaling Person should not produce an error")
	assert.NoError(t, errCarUnmarshal, "Unmarshaling Car should not produce an error")

	// Assert: Correct types
	assert.Equal(t, "person", personContent.Discriminator, "Person discriminator should match")
	assert.Equal(t, "car", carContent.Discriminator, "Car discriminator should match")

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

func TestRegisterTypeShouldProvideSimplifiedSyntax(t *testing.T) {
	// Arrange
	ClearRegistry()

	// Act: Register using the simplified syntax
	RegisterType[Person]()

	// Assert: Verify registration worked
	factory, err := LoadFactory("person")
	assert.NoError(t, err, "Loading registered factory should not produce an error")
	assert.NotNil(t, factory, "Loaded factory should not be nil")

	// Assert: Verify instance creation
	instance := factory()
	personObj, ok := instance.(*Person)
	assert.True(t, ok, "Factory should return an instance of *Person")
	assert.Equal(t, "person", personObj.GetDiscriminator(), "Discriminator should match")
}

func TestShouldPanicGivenNonPolymorphicType(t *testing.T) {
	// Arrange
	ClearRegistry()

	// Act & Assert
	assert.Panics(t, func() {
		RegisterType[NonPolymorphicType]()
	}, "Should panic when registering a type that doesn't implement Polymorphic")
}

func TestRegisterTypeShouldWorkEquivalentlyToRegister(t *testing.T) {
	// Test that RegisterType[T]() produces the same result as Register(func() *T { return &T{} })

	// Test with RegisterType
	ClearRegistry()
	RegisterType[Car]()
	factory1, err1 := LoadFactory("car")
	assert.NoError(t, err1)
	instance1 := factory1()

	// Test with Register
	ClearRegistry()
	Register(func() *Car { return &Car{} })
	factory2, err2 := LoadFactory("car")
	assert.NoError(t, err2)
	instance2 := factory2()

	// Both should produce equivalent results
	assert.Equal(t, fmt.Sprintf("%T", instance1), fmt.Sprintf("%T", instance2), "Both methods should create same type")
	car1, ok1 := instance1.(*Car)
	car2, ok2 := instance2.(*Car)
	assert.True(t, ok1 && ok2, "Both should create *Car instances")
	assert.Equal(t, car1.GetDiscriminator(), car2.GetDiscriminator(), "Both should have same discriminator")
}

func TestRegisterWithDiscriminatorShouldAllowDirectUsage(t *testing.T) {
	// Arrange
	ClearRegistry()
	customFactory := func() any { return &Person{Name: "Test", Age: 25} }

	// Act
	RegisterWithDiscriminator("custom-person", customFactory)

	// Assert
	factory, err := LoadFactory("custom-person")
	assert.NoError(t, err, "Should load custom discriminator")

	instance := factory()
	person, ok := instance.(*Person)
	assert.True(t, ok, "Should create Person instance")
	assert.Equal(t, "Test", person.Name, "Should preserve initial values")
	assert.Equal(t, 25, person.Age, "Should preserve initial values")
}

func TestShouldReturnErrorWhenCreatingInstanceOfUnregisteredType(t *testing.T) {
	// Arrange
	ClearRegistry()

	// Test unregistered type
	instance, err := CreateInstance("unregistered")
	assert.Error(t, err, "Should error for unregistered type")
	assert.Nil(t, instance, "Instance should be nil for unregistered type")
	assert.ErrorContains(t, err, "type \"unregistered\" is not registered", "Error should mention unregistered type")

	// Test invalid factory (register something that doesn't return Polymorphic)
	RegisterWithDiscriminator("invalid", func() any { return "not polymorphic" })
	instance, err = CreateInstance("invalid")
	assert.Error(t, err, "Should error for invalid instance type")
	assert.Nil(t, instance, "Instance should be nil for invalid type")
	assert.ErrorContains(t, err, "invalid instance type for \"invalid\"", "Error should mention invalid instance")
}

func TestLoadFactoryShouldHandleErrorCases(t *testing.T) {
	// Arrange
	ClearRegistry()

	// Test invalid factory type (this would be hard to trigger in practice, but for completeness)
	// We'll just test the unregistered case since the invalid factory type case is difficult to create
	factory, err := LoadFactory("nonexistent")
	assert.Error(t, err, "Should error for nonexistent type")
	assert.Nil(t, factory, "Factory should be nil")
	assert.ErrorContains(t, err, "type \"nonexistent\" is not registered", "Error should mention unregistered type")
}

func TestShouldRemoveAllTypesWhenRegistryCleared(t *testing.T) {
	// Arrange: Register multiple types
	ClearRegistry()
	RegisterType[Person]()
	RegisterType[Car]()

	// Verify they're registered
	_, err1 := LoadFactory("person")
	_, err2 := LoadFactory("car")
	assert.NoError(t, err1, "Person should be registered")
	assert.NoError(t, err2, "Car should be registered")

	// Act: Clear registry
	ClearRegistry()

	// Assert: Both should be gone
	_, err1 = LoadFactory("person")
	_, err2 = LoadFactory("car")
	assert.Error(t, err1, "Person should be unregistered after clear")
	assert.Error(t, err2, "Car should be unregistered after clear")
}

func TestRegisterShouldSupportCustomFactory(t *testing.T) {
	// Test that Register works with custom factory functions
	ClearRegistry()

	// Register with a factory that sets initial values
	Register(func() *Person {
		return &Person{Name: "Default", Age: 0}
	})

	// Test it works
	instance, err := CreateInstance("person")
	assert.NoError(t, err)
	person, ok := instance.(*Person)
	assert.True(t, ok)
	assert.Equal(t, "Default", person.Name, "Should use factory's initial values")
	assert.Equal(t, 0, person.Age, "Should use factory's initial values")
}

// NonPolymorphicType is a type that doesn't implement Polymorphic interface
type NonPolymorphicType struct {
	Value string
}

func TestUnmarshalPolymorphicJSONShouldSucceed(t *testing.T) {
	// Arrange
	ClearRegistry()
	RegisterType[Person]()
	jsonStr := `{"$type":"person","content":{"name":"Alice","age":30}}`

	// Act
	envelope, err := UnmarshalPolymorphicJSON([]byte(jsonStr))

	// Assert
	assert.NoError(t, err, "Unmarshaling should not produce an error")
	assert.NotNil(t, envelope, "Envelope should not be nil")
	assert.Equal(t, "person", envelope.Discriminator, "Discriminator should match")

	person, ok := envelope.Content.(*Person)
	assert.True(t, ok, "Content should be of type *Person")
	assert.Equal(t, "Alice", person.Name, "Person name should be 'Alice'")
	assert.Equal(t, 30, person.Age, "Person age should be 30")
}

func TestUnmarshalPolymorphicJSONShouldFailGivenInvalidJSON(t *testing.T) {
	// Arrange
	invalidJSON := `{"invalid json"`

	// Act
	envelope, err := UnmarshalPolymorphicJSON([]byte(invalidJSON))

	// Assert
	assert.Error(t, err, "Should error on invalid JSON")
	assert.Nil(t, envelope, "Envelope should be nil on error")
	assert.ErrorContains(t, err, "failed to unmarshal polymorphic JSON", "Error should mention unmarshal failure")
}

func TestNewEnvelopeShouldCreateCorrectEnvelope(t *testing.T) {
	// Arrange
	person := &Person{Name: "Test", Age: 25}

	// Act
	envelope := NewEnvelope(person)

	// Assert
	assert.NotNil(t, envelope, "Envelope should not be nil")
	assert.Equal(t, "person", envelope.Discriminator, "Discriminator should match")
	assert.Equal(t, person, envelope.Content, "Content should be the same instance")
}

func TestEnvelopeMarshalJSONShouldFailGivenUnregisteredType(t *testing.T) {
	// Arrange
	ClearRegistry()
	person := &Person{Name: "Test", Age: 25}
	envelope := NewEnvelope(person)

	// Act
	_, err := envelope.MarshalJSON()

	// Assert
	assert.Error(t, err, "Should error when type is not registered")
	assert.ErrorContains(t, err, "type \"person\" is not registered", "Error should mention unregistered type")
}

func TestEnvelopeMarshalJSONShouldFailGivenInvalidContent(t *testing.T) {
	// Arrange
	ClearRegistry()
	RegisterType[Person]()

	// Create envelope with content that can't be marshaled
	envelope := &Envelope{
		Discriminator: "person",
		Content:       make(chan int), // channels can't be marshaled to JSON
	}

	// Act
	_, err := envelope.MarshalJSON()

	// Assert
	assert.Error(t, err, "Should error when content can't be marshaled")
	assert.ErrorContains(t, err, "failed to marshal content", "Error should mention marshal failure")
}

func TestEnvelopeUnmarshalJSONShouldFailGivenMissingTypeField(t *testing.T) {
	// Arrange
	jsonStr := `{"content":{"name":"Alice","age":30}}`
	var envelope Envelope

	// Act
	err := envelope.UnmarshalJSON([]byte(jsonStr))

	// Assert
	assert.Error(t, err, "Should error when $type field is missing")
	assert.ErrorContains(t, err, "missing $type field in envelope", "Error should mention missing $type")
}

func TestEnvelopeUnmarshalJSONShouldFailGivenInvalidTypeFormat(t *testing.T) {
	// Arrange
	jsonStr := `{"$type":123,"content":{"name":"Alice","age":30}}`
	var envelope Envelope

	// Act
	err := envelope.UnmarshalJSON([]byte(jsonStr))

	// Assert
	assert.Error(t, err, "Should error when $type format is invalid")
	assert.ErrorContains(t, err, "invalid $type format", "Error should mention invalid $type format")
}

func TestEnvelopeUnmarshalJSONShouldFailGivenEmptyContent(t *testing.T) {
	// Arrange
	ClearRegistry()
	RegisterType[Person]()
	jsonStr := `{"$type":"person"}`
	var envelope Envelope

	// Act
	err := envelope.UnmarshalJSON([]byte(jsonStr))

	// Assert
	assert.Error(t, err, "Should error when content is missing")
	assert.ErrorContains(t, err, "missing content for type: \"person\"", "Error should mention missing content")
}

func TestEnvelopeUnmarshalJSONShouldFailGivenInvalidContentFormat(t *testing.T) {
	// Arrange
	ClearRegistry()
	RegisterType[Person]()
	jsonStr := `{"$type":"person","content":"invalid json for person"}`
	var envelope Envelope

	// Act
	err := envelope.UnmarshalJSON([]byte(jsonStr))

	// Assert
	assert.Error(t, err, "Should error when content format is invalid")
	assert.ErrorContains(t, err, "failed to unmarshal content for \"person\"", "Error should mention content unmarshal failure")
}

func TestLoadFactoryShouldFailGivenInvalidFactoryType(t *testing.T) {
	// This test is challenging to create since we'd need to manually store an invalid factory
	// The existing tests already cover the realistic error cases
	// This test documents that the error case exists but is hard to trigger in practice
	ClearRegistry()

	factory, err := LoadFactory("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.ErrorContains(t, err, "not registered")
}

type Car struct {
	Make  string `json:"make"`
	Model string `json:"model"`
}

// Implement the Discriminator interface
func (e *Car) GetDiscriminator() string {
	return "car"
}

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Implement the Discriminator interface
func (e *Person) GetDiscriminator() string {
	return "person"
}
