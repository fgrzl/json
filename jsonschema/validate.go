package jsonschema

import (
	"encoding/base64"
	"fmt"
	"math"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
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
	if schema == nil {
		return
	}

	// $ref: resolve and validate against referenced schema (same-document only)
	if ref, ok := schema[RefKey].(string); ok {
		resolved, err := resolveRef(root, ref)
		if err != nil {
			addErr(errs, path, err.Error())
		} else {
			validateAt(root, path, resolved, data, errs)
		}
	}

	// allOf: must validate against all subschemas
	if allOf, ok := schema[AllOfKey].([]any); ok {
		for _, s := range allOf {
			if sub, ok := s.(map[string]any); ok {
				validateAt(root, path, sub, data, errs)
			}
		}
	}

	// anyOf: at least one must pass
	if anyOf, ok := schema[AnyOfKey].([]any); ok {
		matched := false
		var anyErrs []ValidationError
		for _, s := range anyOf {
			sub, _ := s.(map[string]any)
			if sub == nil {
				continue
			}
			var subErrs []ValidationError
			validateAt(root, path, sub, data, &subErrs)
			if len(subErrs) == 0 {
				matched = true
				break
			}
			anyErrs = append(anyErrs, subErrs...)
		}
		if !matched {
			*errs = append(*errs, anyErrs...)
		}
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
	}

	// not: subschema must fail
	if notSchema, ok := schema[NotKey].(map[string]any); ok {
		var notErrs []ValidationError
		validateAt(root, path, notSchema, data, &notErrs)
		if len(notErrs) == 0 {
			addErr(errs, path, "value must not match schema in not")
		}
	}

	validateIfThenElse(root, path, schema, data, errs)

	// type: string or []any for nullable
	if typeVal, hasType := schema[TypeKey]; hasType {
		if validateExplicitType(root, path, schema, typeVal, data, errs) {
			validateEnumConst(path, schema, data, errs)
		}
		return
	}

	validateInstanceConstraints(root, path, schema, data, errs)
	validateEnumConst(path, schema, data, errs)
}

func validateExplicitType(root map[string]any, path string, schema map[string]any, typeVal any, data any, errs *[]ValidationError) bool {
	switch typed := typeVal.(type) {
	case string:
		if !typeMatches(typed, data) {
			addErr(errs, path, fmt.Sprintf("expected %s, got %s", typed, jsonKind(data)))
			return false
		}
		validateTypeConstraints(root, path, schema, data, errs)
		return true
	case []any:
		matched := false
		for _, t := range typed {
			tstr, _ := t.(string)
			if typeMatches(tstr, data) {
				matched = true
				break
			}
		}
		if !matched {
			addErr(errs, path, fmt.Sprintf("value must be one of types %v", typed))
			return false
		}
		validateTypeConstraints(root, path, schema, data, errs)
		return true
	case []string:
		matched := false
		for _, t := range typed {
			if typeMatches(t, data) {
				matched = true
				break
			}
		}
		if !matched {
			addErr(errs, path, fmt.Sprintf("value must be one of types %v", typed))
			return false
		}
		validateTypeConstraints(root, path, schema, data, errs)
		return true
	}

	typeStr := fmt.Sprint(typeVal)
	if !typeMatches(typeStr, data) {
		addErr(errs, path, fmt.Sprintf("expected %s, got %s", typeStr, jsonKind(data)))
		return false
	}
	validateTypeConstraints(root, path, schema, data, errs)
	return true
}

func validateInstanceConstraints(root map[string]any, path string, schema map[string]any, data any, errs *[]ValidationError) {
	switch value := data.(type) {
	case map[string]any:
		validateObject(root, path, schema, value, errs)
	case []any:
		validateArray(root, path, schema, value, errs)
	case string:
		validateStringConstraints(path, schema, value, errs)
	default:
		if _, ok := toFloat(data); ok {
			validateNumberConstraints(path, schema, data, errs)
		}
	}
}

func validateIfThenElse(root map[string]any, path string, schema map[string]any, data any, errs *[]ValidationError) {
	ifSchema, ok := schema[IfKey].(map[string]any)
	if !ok {
		return
	}

	var ifErrs []ValidationError
	validateAt(root, path, ifSchema, data, &ifErrs)
	if len(ifErrs) == 0 {
		if thenSchema, ok := schema[ThenKey].(map[string]any); ok {
			validateAt(root, path, thenSchema, data, errs)
		}
		return
	}

	if elseSchema, ok := schema[ElseKey].(map[string]any); ok {
		validateAt(root, path, elseSchema, data, errs)
	}
}

func validateTypeConstraints(root map[string]any, path string, schema map[string]any, data any, errs *[]ValidationError) {
	matchedType, ok := matchingSchemaType(schema[TypeKey], data)
	if !ok {
		return
	}

	validateTypeSpecificConstraints(root, path, schema, matchedType, data, errs)
}

func matchingSchemaType(typeSpec any, data any) (string, bool) {
	switch typed := typeSpec.(type) {
	case string:
		if typeMatches(typed, data) {
			return typed, true
		}
	case []any:
		for _, candidate := range typed {
			typeName, _ := candidate.(string)
			if typeName == "" || typeName == "null" {
				continue
			}
			if typeMatches(typeName, data) {
				return typeName, true
			}
		}
	case []string:
		for _, typeName := range typed {
			if typeName == "" || typeName == "null" {
				continue
			}
			if typeMatches(typeName, data) {
				return typeName, true
			}
		}
	}

	return "", false
}

func validateTypeSpecificConstraints(root map[string]any, path string, schema map[string]any, typeVal string, data any, errs *[]ValidationError) {
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
}

type patternProperty struct {
	re     *regexp.Regexp
	schema map[string]any
}

func validateObject(root map[string]any, path string, schema map[string]any, obj map[string]any, errs *[]ValidationError) {
	validateObjectPropertyCounts(path, schema, len(obj), errs)
	patternProps := compilePatternProperties(path, schema, errs)
	matchedByPattern := make(map[string]bool)

	if req, ok := schemaAnySlice(schema[RequiredKey]); ok {
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
		for _, patternProp := range patternProps {
			if patternProp.re.MatchString(key) {
				matchedByPattern[key] = true
				validateAt(root, subPath, patternProp.schema, val, errs)
			}
		}
	}
	validateAdditionalProperties(root, path, schema, obj, matchedByPattern, errs)
}

func validateArray(root map[string]any, path string, schema map[string]any, arr []any, errs *[]ValidationError) {
	itemsSchema, hasItems := schema[ItemsKey].(map[string]any)
	if hasItems {
		for i, item := range arr {
			validateAt(root, fmt.Sprintf("%s/%d", path, i), itemsSchema, item, errs)
		}
	}
	validateArrayLength(path, schema, len(arr), errs)
	validateUniqueItems(path, schema, arr, errs)
	validateContains(root, path, schema, arr, errs)
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
	if format, ok := schema[FormatKey].(string); ok && format != "" {
		validateFormatConstraint(path, format, s, errs)
	}
}

func validateFormatConstraint(path, format, value string, errs *[]ValidationError) {
	switch format {
	case "date-time":
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			addErr(errs, path, "string does not match format date-time")
		}
	case "uuid":
		if _, err := uuid.Parse(value); err != nil {
			addErr(errs, path, "string does not match format uuid")
		}
	case "uri":
		u, err := url.Parse(value)
		if err != nil || u.Scheme == "" {
			addErr(errs, path, "string does not match format uri")
		}
	case "ipv4":
		ip := net.ParseIP(value)
		if ip == nil || ip.To4() == nil {
			addErr(errs, path, "string does not match format ipv4")
		}
	case "byte":
		if _, err := base64.StdEncoding.DecodeString(value); err != nil {
			addErr(errs, path, "string does not match format byte")
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
	if multiple, ok := toFloat(schema[MultipleOfKey]); ok {
		if multiple == 0 {
			addErr(errs, path, "multipleOf must be non-zero")
		} else if !isMultipleOf(n, multiple) {
			addErr(errs, path, fmt.Sprintf("value %v not multiple of %v", data, multiple))
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
	if enum, ok := schemaAnySlice(schema[EnumKey]); ok {
		for _, e := range enum {
			if deepEqualJSON(e, data) {
				return
			}
		}
		addErr(errs, path, fmt.Sprintf("value not in enum %v", enum))
	}
}

func validateObjectPropertyCounts(path string, schema map[string]any, length int, errs *[]ValidationError) {
	if min, ok := toFloat(schema[MinPropertiesKey]); ok {
		if float64(length) < min {
			addErr(errs, path, fmt.Sprintf("object property count %d less than minProperties %d", length, int(min)))
		}
	}
	if max, ok := toFloat(schema[MaxPropertiesKey]); ok {
		if float64(length) > max {
			addErr(errs, path, fmt.Sprintf("object property count %d greater than maxProperties %d", length, int(max)))
		}
	}
}

func compilePatternProperties(path string, schema map[string]any, errs *[]ValidationError) []patternProperty {
	rawPatternProps, _ := schema[PatternPropertiesKey].(map[string]any)
	if rawPatternProps == nil {
		return nil
	}

	compiled := make([]patternProperty, 0, len(rawPatternProps))
	for pattern, value := range rawPatternProps {
		subSchema, ok := value.(map[string]any)
		if !ok {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			addErr(errs, path, fmt.Sprintf("invalid patternProperties regex %s", pattern))
			continue
		}
		compiled = append(compiled, patternProperty{re: re, schema: subSchema})
	}

	return compiled
}

func validateAdditionalProperties(root map[string]any, path string, schema map[string]any, obj map[string]any, matchedByPattern map[string]bool, errs *[]ValidationError) {
	props, _ := schema[PropertiesKey].(map[string]any)
	allowed := map[string]bool{}
	for k := range props {
		allowed[k] = true
	}
	additionalVal := schema[AdditionalPropertiesKey]
	if additionalVal == false {
		for k := range obj {
			if !allowed[k] && !matchedByPattern[k] {
				addErr(errs, path+"/"+escapeJSONPointer(k), "additional property not allowed")
			}
		}
		return
	}
	if subSchema, ok := additionalVal.(map[string]any); ok {
		for k, v := range obj {
			if allowed[k] || matchedByPattern[k] {
				continue
			}
			validateAt(root, path+"/"+escapeJSONPointer(k), subSchema, v, errs)
		}
	}
}

func validateContains(root map[string]any, path string, schema map[string]any, arr []any, errs *[]ValidationError) {
	containsSchema, ok := schema[ContainsKey].(map[string]any)
	if !ok {
		return
	}

	for i, item := range arr {
		var itemErrs []ValidationError
		validateAt(root, fmt.Sprintf("%s/%d", path, i), containsSchema, item, &itemErrs)
		if len(itemErrs) == 0 {
			return
		}
	}

	addErr(errs, path, "array must contain at least one item matching contains")
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
	for i := 1; i < len(arr); i++ {
		for j := 0; j < i; j++ {
			if deepEqualJSON(arr[i], arr[j]) {
				addErr(errs, fmt.Sprintf("%s/%d", path, i), "duplicate array items (uniqueItems)")
				return
			}
		}
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

func isMultipleOf(value, multiple float64) bool {
	quotient := value / multiple
	return math.Abs(quotient-math.Round(quotient)) <= 1e-9
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

func schemaAnySlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		converted := make([]any, len(typed))
		for i, item := range typed {
			converted[i] = item
		}
		return converted, true
	default:
		return nil, false
	}
}

// resolveRef resolves #/$defs/X or #/defs/X from the root (same-document only).
func resolveRef(rootSchema map[string]any, ref string) (map[string]any, error) {
	if ref == "" {
		return nil, fmt.Errorf("unresolved ref %q", ref)
	}
	if ref[0] != '#' {
		return nil, fmt.Errorf("unsupported ref %q", ref)
	}

	originalRef := ref
	ref = ref[1:]
	if ref == "" {
		return rootSchema, nil
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
				return sub, nil
			}
		}
	}
	if len(parts) >= 3 && parts[0] == "components" && parts[1] == "schemas" {
		comp, _ := rootSchema["components"].(map[string]any)
		if comp != nil {
			schemas, _ := comp["schemas"].(map[string]any)
			if schemas != nil {
				if sub, ok := schemas[parts[2]].(map[string]any); ok {
					return sub, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("unresolved ref %q", originalRef)
}
