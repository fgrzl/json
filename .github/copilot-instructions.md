# GitHub Copilot Instructions

This document provides guidelines for GitHub Copilot when working on this Go project. These instructions ensure consistent, idiomatic Go code and behavioral testing practices.

## Go Code Style and Idioms

### General Go Principles
- Follow the [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` and `goimports` for consistent formatting
- Write clear, self-documenting code with meaningful variable and function names
- Prefer composition over inheritance
- Use interfaces to define behavior, not implementation
- Keep functions small and focused on a single responsibility

### Naming Conventions
- Use `camelCase` for variable and function names
- Use `PascalCase` for exported types, functions, and constants
- Use descriptive names: `userCount` not `cnt`, `validateInput` not `validate`
- Interface names should end with `-er` when possible: `Reader`, `Writer`, `Marshaler`
- Package names should be short, lowercase, and descriptive

### Error Handling
- Always handle errors explicitly - never ignore them with `_`
- Use `fmt.Errorf` for error wrapping: `fmt.Errorf("failed to process %s: %w", name, err)`
- Return errors as the last return value
- Use sentinel errors for well-known error conditions
- Provide context in error messages to aid debugging

### Concurrency
- Use channels to communicate between goroutines
- Prefer `sync.Mutex` over channel-based synchronization for protecting shared state
- Use `context.Context` for cancellation and timeouts
- Always check for context cancellation in long-running operations

### Memory Management
- Prefer value types over pointers when possible
- Use `sync.Pool` for expensive-to-allocate objects that are frequently created
- Avoid memory leaks by properly closing resources (files, connections, etc.)
- Use `defer` for cleanup operations

### Generic Types (Go 1.18+)
- Use meaningful type parameter names: `[T Comparable]` not `[T any]`
- Constrain type parameters appropriately to ensure type safety
- Prefer specific interfaces over `any` when possible
- Use type parameters to eliminate code duplication while maintaining type safety

## Testing Standards

### Test Structure and Naming
- Test files should end with `_test.go`
- Test functions should start with `Test` followed by the function/behavior being tested
- Use descriptive test names that describe the scenario: `TestRegisterType_PanicOnNonPolymorphic`
- Group related tests using subtests with `t.Run()`

### Behavioral Testing Approach
Tests should be **behavioral** in nature, focusing on what the system should do rather than how it does it.

#### Test Function Naming Pattern
Use behavioral naming patterns that focus on what the system **should** do:

**Preferred patterns:**
- `TestShould<Behavior>Given<Condition>`
- `TestShould<Behavior>When<Action>`
- `Test<Function>Should<Behavior>Given<Condition>`

Examples:
```go
func TestShouldPanicGivenNonPolymorphicType(t *testing.T)
func TestShouldReturnErrorWhenTypeUnregistered(t *testing.T)
func TestRegisterTypeShouldSucceedGivenValidType(t *testing.T)
func TestCreateInstanceShouldReturnErrorGivenUnregisteredType(t *testing.T)
func TestMarshalJSONShouldFailWhenContentInvalid(t *testing.T)
```

#### Test Comments Structure
When test names are descriptive and behavioral, avoid redundant Given/When/Then comments. The test name should clearly communicate the behavior being tested.

**Good example:**
```go
func TestShouldPanicGivenNonPolymorphicType(t *testing.T) {
    // Arrange
    polymorphic.ClearRegistry()
    
    // Act & Assert
    assert.Panics(t, func() {
        polymorphic.RegisterType[NonPolymorphicType]()
    }, "Should panic when registering a type that doesn't implement Polymorphic")
}
```

**Avoid redundant comments:**
```go
func TestShouldPanicGivenNonPolymorphicType(t *testing.T) {
    // Given: A type that does not implement Polymorphic interface  // ❌ Redundant
    // When: RegisterType is called with that type                  // ❌ Redundant  
    // Then: It should panic with a descriptive error message       // ❌ Redundant
    
    // Arrange
    // ...
}
```

### Test Structure: Arrange/Act/Assert (AAA)
Every test should follow the AAA pattern with clear section comments:

```go
func TestExample(t *testing.T) {
    // Arrange
    polymorphic.ClearRegistry()
    expectedValue := "test"
    
    // Act
    result, err := someFunction(expectedValue)
    
    // Assert
    assert.NoError(t, err, "Function should not return an error")
    assert.Equal(t, expectedValue, result, "Result should match expected value")
}
```

### Test Organization
- **Arrange**: Set up test data, mocks, and preconditions
- **Act**: Execute the code being tested (usually one function call)
- **Assert**: Verify the results match expectations

### Assertion Guidelines
- Use the `testify/assert` library for assertions
- Include descriptive messages in assertions to aid debugging
- Test both success and failure scenarios
- Assert on both the result and any side effects
- Use `assert.ErrorContains()` for error message validation

### Test Coverage Expectations
- Aim for 90%+ code coverage
- Focus on testing behavior, not implementation details
- Test error conditions and edge cases
- Ensure all public APIs have comprehensive test coverage
- Test concurrent code with race condition detection

### Example Test Implementation
```go
func TestShouldPanicGivenNonPolymorphicType(t *testing.T) {
    // Given: A type that does not implement Polymorphic interface
    // When: RegisterType is called with that type
    // Then: It should panic with a descriptive error message
    
    // Arrange
    polymorphic.ClearRegistry()
    
    // Act & Assert
    assert.Panics(t, func() {
        polymorphic.RegisterType[NonPolymorphicType]()
    }, "Should panic when registering a type that doesn't implement Polymorphic")
}

func TestCreateInstanceShouldReturnValidInstanceGivenRegisteredType(t *testing.T) {
    // Given: A registered polymorphic type
    // When: CreateInstance is called with the type's discriminator
    // Then: It should return a valid instance of that type
    
    // Arrange
    polymorphic.ClearRegistry()
    polymorphic.RegisterType[Person]()
    expectedDiscriminator := "person"
    
    // Act
    instance, err := polymorphic.CreateInstance(expectedDiscriminator)
    
    // Assert
    assert.NoError(t, err, "CreateInstance should not return an error for registered type")
    assert.NotNil(t, instance, "Instance should not be nil")
    
    person, ok := instance.(*Person)
    assert.True(t, ok, "Instance should be of type *Person")
    assert.Equal(t, expectedDiscriminator, person.GetDiscriminator(), "Instance discriminator should match expected")
}
```

## Package Organization

### File Structure
- Keep related functionality in the same package
- Use internal packages for implementation details not meant for external use
- Separate test files with `_test.go` suffix
- Use `doc.go` files for package-level documentation

### Import Organization
Group imports in this order:
1. Standard library imports
2. Third-party imports  
3. Local project imports

```go
import (
    "encoding/json"
    "fmt"
    "sync"

    "github.com/stretchr/testify/assert"

    "github.com/fgrzl/json/polymorphic"
)
```

## Documentation

### Code Comments
- Write package comments for all packages
- Document all exported functions, types, and constants
- Use complete sentences starting with the name being documented
- Include examples in doc comments when helpful

### GoDoc Requirement for Exported APIs
- Ensure every exported type, function, variable, and constant has a GoDoc comment.
- Comments should follow the standard GoDoc format and start with the item name (for example: "Builder builds..." or "GenerateSchema returns...").
- Prefer short, focused examples and show expected inputs/outputs for non-trivial APIs.
- This repository will use GoDoc comments as part of code review criteria and automated documentation generation.

### Function Documentation
```go
// RegisterType registers a polymorphic type T with the global registry.
// The type T must implement the Polymorphic interface with a pointer receiver.
// It panics if T does not implement Polymorphic interface.
//
// Example:
//   polymorphic.RegisterType[MyType]()
func RegisterType[T any]() {
    // implementation
}
```

These guidelines ensure consistent, maintainable, and well-tested Go code that follows established idioms and best practices.

### Docs folder and file naming

- Use a top-level `docs/` directory for project documentation.
- Documentation files should follow the naming convention `docs/my-doc.md` (all lower-case, hyphen-separated if needed). Use `my-doc.md` as the canonical naming pattern when creating new docs.
- When adding new documentation, include a short frontmatter summary (one paragraph) and a small "Try it" section when the doc demonstrates usage or commands.
- Copilot should create docs under `docs/` and respect the `docs/my-doc.md` naming convention when suggesting or generating new documentation files.

