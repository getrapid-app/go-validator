package validator

import (
	"context"
	"encoding/json"
)

func registerTypes(r *Registry) {
	r.Register("string", fixed(String())).
		Register("integer", func(p []string) (Rule, error) { return integerRule(containsString(p, "strict")), nil }).
		Register("numeric", func(p []string) (Rule, error) { return numericRule(containsString(p, "strict")), nil }).
		Register("boolean", func(p []string) (Rule, error) { return booleanRule(containsString(p, "strict")), nil }).
		Register("array", values(Array)).
		Register("list", fixed(ListRule())).
		Register("json", fixed(JSON()))
}

// String fails unless the value is a string.
func String() Rule {
	return check("string", nil, func(f Field) bool { _, ok := f.Value.(string); return ok })
}

// Integer passes Go integers, whole floats (as JSON numbers decode) and
// strings holding an integer.
func Integer() Rule { return integerRule(false) }

// IntegerStrict passes Go integers and whole floats, but not strings.
func IntegerStrict() Rule { return integerRule(true) }

func integerRule(strict bool) Rule {
	return check("integer", nil, func(f Field) bool {
		if _, isStr := f.Value.(string); isStr && strict {
			return false
		}
		_, ok := toInt(f.Value)
		return ok
	})
}

// Numeric passes Go numbers and numeric strings.
func Numeric() Rule { return numericRule(false) }

// NumericStrict passes Go numbers only.
func NumericStrict() Rule { return numericRule(true) }

func numericRule(strict bool) Rule {
	return check("numeric", nil, func(f Field) bool {
		if _, isStr := f.Value.(string); isStr && strict {
			return false
		}
		_, ok := toFloat(f.Value)
		return ok
	})
}

// Boolean passes true, false, 1, 0, "1", "0", "true" and "false".
func Boolean() Rule { return booleanRule(false) }

// BooleanStrict passes only true and false.
func BooleanStrict() Rule { return booleanRule(true) }

func booleanRule(strict bool) Rule {
	return check("boolean", nil, func(f Field) bool {
		switch v := f.Value.(type) {
		case bool:
			return true
		case string:
			return !strict && containsString([]string{"1", "0", "true", "false"}, v)
		}
		n, ok := toFloat(f.Value)
		return !strict && ok && (n == 0 || n == 1)
	})
}

// Array passes lists and maps. With keys, a map may only hold those keys,
// as in Laravel's array:name,email.
func Array(keys ...string) Rule {
	return check("array", nil, func(f Field) bool {
		switch v := f.Value.(type) {
		case []any:
			return true
		case map[string]any:
			for k := range v {
				if len(keys) > 0 && !containsString(keys, k) {
					return false
				}
			}
			return true
		}
		return false
	})
}

// ListRule passes lists ([]any) only, not maps. It is named ListRule
// because List builds a RuleSet.
func ListRule() Rule {
	return check("list", nil, func(f Field) bool { _, ok := f.Value.([]any); return ok })
}

// JSON passes strings that hold valid JSON.
func JSON() Rule {
	return check("json", nil, func(f Field) bool {
		s, ok := f.Value.(string)
		return ok && json.Valid([]byte(s))
	})
}

// Enum passes values equal to one of the given values, after converting the
// input to T (so "active" matches a named type Status string). It is the
// counterpart of Laravel's Rule::enum.
//
//	validator.Enum(StatusActive, StatusArchived)
func Enum[T comparable](allowed ...T) Rule {
	return &funcRule{name: "enum", fn: func(_ context.Context, f Field) error {
		v, ok := As[T](f.Value)
		if ok {
			for _, a := range allowed {
				if a == v {
					return nil
				}
			}
		}
		return Fail("enum", nil)
	}}
}

// ValidEnum passes values that convert to T and report IsValid() == true,
// for enum types that know their own valid values.
func ValidEnum[T interface{ IsValid() bool }]() Rule {
	return &funcRule{name: "enum", fn: func(_ context.Context, f Field) error {
		if v, ok := As[T](f.Value); ok && v.IsValid() {
			return nil
		}
		return Fail("enum", nil)
	}}
}
