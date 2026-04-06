// Package polymorphic provides JSON serialization for values of different
// concrete types using a discriminator-based envelope format.
//
// # Wire format
//
// The wire format is a JSON object with two fields:
//   - "$type" (string): the discriminator; must be non-empty and must have
//     been registered via Register, RegisterType, or RegisterWithDiscriminator.
//   - "content" (object): the JSON value decoded into the type registered
//     for that discriminator. It must be present and non-null.
//
// Unknown top-level keys are ignored when unmarshaling.
//
// # Global state
//
// Types register themselves under a discriminator string. Some types (for
// example PolymorphicPage) register in init() when the package is imported.
// Tests that require a clean registry should call ClearRegistry(), which
// removes custom registrations and restores the package defaults.
package polymorphic
