package jsonschema

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// ValidationError represents a single validation failure at a path.
type ValidationError struct {
	Path    string
	Message string
}

// ErrValidation holds one or more validation errors and implements error.
type ErrValidation struct {
	Errs []ValidationError
}

func (e *ErrValidation) Error() string {
	if len(e.Errs) == 0 {
		return "validation failed"
	}
	if len(e.Errs) == 1 {
		return e.Errs[0].Path + ": " + e.Errs[0].Message
	}
	return fmt.Sprintf("%d validation error(s): %s: %s", len(e.Errs), e.Errs[0].Path, e.Errs[0].Message)
}

// Errors returns the list of validation errors.
func (e *ErrValidation) Errors() []ValidationError {
	return e.Errs
}

// Validate validates data against the schema. Data should be the decoded JSON
// shape: map[string]any, []any, float64, string, bool, or nil. Returns nil if
// valid; otherwise *ErrValidation with one or more path/message pairs.
func Validate(schema map[string]any, data any) error {
	var errs []ValidationError
	validateAt(schema, "", schema, data, &errs)
	if len(errs) == 0 {
		return nil
	}
	return &ErrValidation{Errs: errs}
}

func validateAt(root map[string]any, path string, schema map[string]any, data any, errs *[]ValidationError) {
	// $ref: resolve and validate against referenced schema (same-document only)
	if ref, ok := schema[RefKey].(string); ok {
		resolved := resolveRef(root, ref)
		if resolved != nil {
			validateAt(root, path, resolved, data, errs)
		}
		return
	}

	// allOf: must validate against all subschemas
	if allOf, ok := schema[AllOfKey].([]any); ok {
		for _, s := range allOf {
			if sub, ok := s.(map[string]any); ok {
				validateAt(root, path, sub, data, errs)
			}
		}
		return
	}

	// anyOf: at least one must pass
	if anyOf, ok := schema[AnyOfKey].([]any); ok {
		var anyErrs []ValidationError
		for _, s := range anyOf {
			sub, _ := s.(map[string]any)
			if sub == nil {
				continue
			}
			var subErrs []ValidationError
			validateAt(root, path, sub, data, &subErrs)
			if len(subErrs) == 0 {
				return
			}
			anyErrs = append(anyErrs, subErrs...)
		}
		*errs = append(*errs, anyErrs...)
		return
	}

	// oneOf: exactly one must pass
	if oneOf, ok := schema[OneOfKey].([]any); ok {
		var matchCount int
		var allErrs []ValidationError
		for _, s := range oneOf {
			sub, _ := s.(map[string]any)
			if sub == nil {
				continue
			}
			var subErrs []ValidationError
			validateAt(root, path, sub, data, &subErrs)
			if len(subErrs) == 0 {
				matchCount++
			} else {
				allErrs = append(allErrs, subErrs...)
			}
		}
		if matchCount != 1 {
			*errs = append(*errs, allErrs...)
			addErr(errs, path, fmt.Sprintf("value must match exactly one schema in oneOf (matched %d)", matchCount))
		}
		return
	}

	// not: subschema must fail
	if notSchema, ok := schema[NotKey].(map[string]any); ok {
		var notErrs []ValidationError
		validateAt(root, path, notSchema, data, &notErrs)
		if len(notErrs) == 0 {
			addErr(errs, path, "value must not match schema in not")
		}
		return
	}

	// type: string or []any for nullable
	if typeVal, hasType := schema[TypeKey]; hasType {
		if typeSlice, ok := typeVal.([]any); ok {
			// nullable: ["string", "null"] etc.
			if data == nil {
				return
			}
			for _, t := range typeSlice {
				tstr, _ := t.(string)
				if tstr == "null" {
					continue
				}
				if typeMatches(tstr, data) {
					validateTypeConstraints(root, path, schema, data, errs)
					return
				}
			}
			addErr(errs, path, fmt.Sprintf("value must be one of types %v", typeSlice))
			return
		}
		typeStr, _ := typeVal.(string)
		if !typeMatches(typeStr, data) {
			addErr(errs, path, fmt.Sprintf("expected %s, got %s", typeStr, jsonKind(data)))
			return
		}
		validateTypeConstraints(root, path, schema, data, errs)
		return
	}

	// no type: allow and recurse into object/array if present
	if obj, ok := data.(map[string]any); ok {
		validateObject(root, path, schema, obj, errs)
	}
}

func validateTypeConstraints(root map[string]any, path string, schema map[string]any, data any, errs *[]ValidationError) {
	typeVal, _ := schema[TypeKey].(string)
	if typeVal == "" {
		if slice, ok := schema[TypeKey].([]any); ok {
			for _, t := range slice {
				if s, _ := t.(string); s != "" && s != "null" {
					typeVal = s
					break
				}
			}
		}
	}
	switch typeVal {
	case TypeObject:
		if obj, ok := data.(map[string]any); ok {
			validateObject(root, path, schema, obj, errs)
		}
	case TypeArray:
		if arr, ok := data.([]any); ok {
			validateArray(root, path, schema, arr, errs)
		}
	case TypeString:
		validateStringConstraints(path, schema, data, errs)
	case TypeNumber, TypeInteger:
		validateNumberConstraints(path, schema, data, errs)
	}
	// enum, const apply to any type
	validateEnumConst(path, schema, data, errs)
}

func validateObject(root map[string]any, path string, schema map[string]any, obj map[string]any, errs *[]ValidationError) {
	if req, ok := schema[RequiredKey].([]any); ok {
		for _, r := range req {
			key, _ := r.(string)
			if key == "" {
				continue
			}
			if _, has := obj[key]; !has {
				addErr(errs, path, fmt.Sprintf("required property missing: %s", key))
			}
		}
	}
	props, _ := schema[PropertiesKey].(map[string]any)
	for key, val := range obj {
		subPath := path + "/" + escapeJSONPointer(key)
		if props != nil {
			if subSchema, ok := props[key].(map[string]any); ok {
				validateAt(root, subPath, subSchema, val, errs)
			}
		}
	}
	// additionalProperties (Phase 2)
	validateAdditionalProperties(root, path, schema, obj, errs)
}

func validateArray(root map[string]any, path string, schema map[string]any, arr []any, errs *[]ValidationError) {
	itemsSchema, hasItems := schema[ItemsKey].(map[string]any)
	if !hasItems {
		return
	}
	for i, item := range arr {
		validateAt(root, fmt.Sprintf("%s/%d", path, i), itemsSchema, item, errs)
	}
	// minItems, maxItems, uniqueItems (Phase 4)
	validateArrayLength(path, schema, len(arr), errs)
	validateUniqueItems(path, schema, arr, errs)
}

func validateStringConstraints(path string, schema map[string]any, data any, errs *[]ValidationError) {
	s, ok := data.(string)
	if !ok {
		return
	}
	if min, ok := toFloat(schema[MinLengthKey]); ok {
		if int(min) > len([]rune(s)) {
			addErr(errs, path, fmt.Sprintf("string length %d less than minLength %d", len([]rune(s)), int(min)))
		}
	}
	if max, ok := toFloat(schema[MaxLengthKey]); ok {
		if int(max) < len([]rune(s)) {
			addErr(errs, path, fmt.Sprintf("string length %d greater than maxLength %d", len([]rune(s)), int(max)))
		}
	}
	if pattern, ok := schema[PatternKey].(string); ok && pattern != "" {
		re, err := regexp.Compile(pattern)
		if err == nil && !re.MatchString(s) {
			addErr(errs, path, fmt.Sprintf("string does not match pattern %s", pattern))
		}
	}
}

func validateNumberConstraints(path string, schema map[string]any, data any, errs *[]ValidationError) {
	n, ok := toFloat(data)
	if !ok {
		return
	}
	if min, ok := toFloat(schema[MinimumKey]); ok {
		if n < min {
			addErr(errs, path, fmt.Sprintf("value %v less than minimum %v", data, min))
		}
	}
	if max, ok := toFloat(schema[MaximumKey]); ok {
		if n > max {
			addErr(errs, path, fmt.Sprintf("value %v greater than maximum %v", data, max))
		}
	}
	if exMin, ok := toFloat(schema[ExclusiveMinimumKey]); ok {
		if n <= exMin {
			addErr(errs, path, fmt.Sprintf("value %v must be > exclusiveMinimum %v", data, exMin))
		}
	}
	if exMax, ok := toFloat(schema[ExclusiveMaximumKey]); ok {
		if n >= exMax {
			addErr(errs, path, fmt.Sprintf("value %v must be < exclusiveMaximum %v", data, exMax))
		}
	}
}

func validateEnumConst(path string, schema map[string]any, data any, errs *[]ValidationError) {
	if c, has := schema[ConstKey]; has {
		if !deepEqualJSON(c, data) {
			addErr(errs, path, fmt.Sprintf("value must be const %v", c))
		}
		return
	}
	if enum, ok := schema[EnumKey].([]any); ok {
		for _, e := range enum {
			if deepEqualJSON(e, data) {
				return
			}
		}
		addErr(errs, path, fmt.Sprintf("value not in enum %v", enum))
	}
}

func validateAdditionalProperties(root map[string]any, path string, schema map[string]any, obj map[string]any, errs *[]ValidationError) {
	props, _ := schema[PropertiesKey].(map[string]any)
	allowed := map[string]bool{}
	for k := range props {
		allowed[k] = true
	}
	additionalVal := schema[AdditionalPropertiesKey]
	if additionalVal == false {
		for k := range obj {
			if !allowed[k] {
				addErr(errs, path+"/"+escapeJSONPointer(k), "additional property not allowed")
			}
		}
		return
	}
	if subSchema, ok := additionalVal.(map[string]any); ok {
		for k, v := range obj {
			if allowed[k] {
				continue
			}
			validateAt(root, path+"/"+escapeJSONPointer(k), subSchema, v, errs)
		}
	}
}

func validateArrayLength(path string, schema map[string]any, length int, errs *[]ValidationError) {
	if min, ok := toFloat(schema[MinItemsKey]); ok {
		if float64(length) < min {
			addErr(errs, path, fmt.Sprintf("array length %d less than minItems %d", length, int(min)))
		}
	}
	if max, ok := toFloat(schema[MaxItemsKey]); ok {
		if float64(length) > max {
			addErr(errs, path, fmt.Sprintf("array length %d greater than maxItems %d", length, int(max)))
		}
	}
}

func validateUniqueItems(path string, schema map[string]any, arr []any, errs *[]ValidationError) {
	if schema[UniqueItemsKey] != true {
		return
	}
	seen := make(map[string]bool)
	for i, item := range arr {
		key := jsonValueKey(item)
		if seen[key] {
			addErr(errs, fmt.Sprintf("%s/%d", path, i), "duplicate array items (uniqueItems)")
			return
		}
		seen[key] = true
	}
}

func typeMatches(typeStr string, data any) bool {
	switch typeStr {
	case TypeString:
		_, ok := data.(string)
		return ok
	case TypeNumber:
		_, ok := toFloat(data)
		return ok
	case TypeInteger:
		n, ok := toFloat(data)
		return ok && n == float64(int64(n))
	case TypeBoolean:
		_, ok := data.(bool)
		return ok
	case "null":
		return data == nil
	case TypeObject:
		_, ok := data.(map[string]any)
		return ok
	case TypeArray:
		_, ok := data.([]any)
		return ok
	default:
		return false
	}
}

func jsonKind(data any) string {
	if data == nil {
		return "null"
	}
	switch data.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return reflect.TypeOf(data).String()
	}
}

func toFloat(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

func addErr(errs *[]ValidationError, path, msg string) {
	*errs = append(*errs, ValidationError{Path: path, Message: msg})
}

// escapeJSONPointer escapes a key for use in a JSON Pointer (RFC 6901).
func escapeJSONPointer(key string) string {
	key = strings.ReplaceAll(key, "~", "~0")
	key = strings.ReplaceAll(key, "/", "~1")
	return key
}

// deepEqualJSON compares two JSON-like values (float64, string, bool, nil, []any, map[string]any).
func deepEqualJSON(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch av := a.(type) {
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqualJSON(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if !deepEqualJSON(v, bv[k]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

// jsonValueKey returns a string key for simple equality in uniqueItems.
func jsonValueKey(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return fmt.Sprintf("%v", x)
	case string:
		return "s:" + x
	default:
		return fmt.Sprintf("%p", v)
	}
}

// resolveRef resolves #/$defs/X or #/defs/X from the root (same-document only).
func resolveRef(rootSchema map[string]any, ref string) map[string]any {
	if ref == "" || ref[0] != '#' {
		return nil
	}
	ref = ref[1:]
	if ref == "" {
		return nil
	}
	if ref[0] == '/' {
		ref = ref[1:]
	}
	parts := strings.Split(ref, "/")
	if len(parts) >= 2 && (parts[0] == "$defs" || parts[0] == "defs") {
		defs, _ := rootSchema[parts[0]].(map[string]any)
		if defs == nil {
			defs, _ = rootSchema[DefsKey].(map[string]any)
		}
		if defs != nil {
			if sub, ok := defs[parts[1]].(map[string]any); ok {
				return sub
			}
		}
	}
	if len(parts) >= 3 && parts[0] == "components" && parts[1] == "schemas" {
		comp, _ := rootSchema["components"].(map[string]any)
		if comp != nil {
			schemas, _ := comp["schemas"].(map[string]any)
			if schemas != nil {
				if sub, ok := schemas[parts[2]].(map[string]any); ok {
					return sub
				}
			}
		}
	}
	return nil
}
