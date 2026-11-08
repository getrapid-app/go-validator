package validator

import (
	"context"
	"fmt"
	"net/mail"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

// This file holds the first group of built-in rules. Step 2 of the roadmap
// ports the rest of Laravel's rules using the same pattern: a constructor
// for the fluent API, a type implementing Rule, and a factory registered in
// registerBuiltins for the string syntax.

const (
	flagBail      flagRule = "bail"
	flagNullable  flagRule = "nullable"
	flagSometimes flagRule = "sometimes"
)

// Bail stops validating a field after its first failure.
func Bail() Rule { return flagBail }

// Nullable lets a field be nil; the other rules are then skipped.
func Nullable() Rule { return flagNullable }

// Sometimes only validates a field when it is present in the input.
func Sometimes() Rule { return flagSometimes }

func registerBuiltins(r *Registry) {
	noParams := func(rule Rule) Factory {
		return func([]string) (Rule, error) { return rule, nil }
	}
	r.Register("bail", noParams(Bail())).
		Register("nullable", noParams(Nullable())).
		Register("sometimes", noParams(Sometimes())).
		Register("required", noParams(Required())).
		Register("required_if", func(p []string) (Rule, error) {
			if len(p) < 2 {
				return nil, fmt.Errorf("needs a field and at least one value")
			}
			return RequiredIf(p[0], p[1:]...), nil
		}).
		Register("string", noParams(String())).
		Register("integer", noParams(Integer())).
		Register("numeric", noParams(Numeric())).
		Register("boolean", noParams(Boolean())).
		Register("array", noParams(Array())).
		Register("email", noParams(Email())).
		Register("min", numberFactory(Min)).
		Register("max", numberFactory(Max)).
		Register("between", func(p []string) (Rule, error) {
			if len(p) != 2 {
				return nil, fmt.Errorf("needs two numbers")
			}
			lo, err1 := strconv.ParseFloat(p[0], 64)
			hi, err2 := strconv.ParseFloat(p[1], 64)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("needs two numbers")
			}
			return Between(lo, hi), nil
		}).
		Register("in", func(p []string) (Rule, error) { return In(p...), nil }).
		Register("same", func(p []string) (Rule, error) {
			if len(p) != 1 {
				return nil, fmt.Errorf("needs one field")
			}
			return Same(p[0]), nil
		})

	for key, msg := range map[string]string{
		"required":        "The :attribute field is required.",
		"required_if":     "The :attribute field is required when :other is :value.",
		"string":          "The :attribute field must be a string.",
		"integer":         "The :attribute field must be an integer.",
		"numeric":         "The :attribute field must be a number.",
		"boolean":         "The :attribute field must be true or false.",
		"array":           "The :attribute field must be an array.",
		"email":           "The :attribute field must be a valid email address.",
		"min.numeric":     "The :attribute field must be at least :min.",
		"min.string":      "The :attribute field must be at least :min characters.",
		"min.array":       "The :attribute field must have at least :min items.",
		"max.numeric":     "The :attribute field must not be greater than :max.",
		"max.string":      "The :attribute field must not be greater than :max characters.",
		"max.array":       "The :attribute field must not have more than :max items.",
		"between.numeric": "The :attribute field must be between :min and :max.",
		"between.string":  "The :attribute field must be between :min and :max characters.",
		"between.array":   "The :attribute field must have between :min and :max items.",
		"in":              "The selected :attribute is invalid.",
		"same":            "The :attribute field must match :other.",
	} {
		r.SetMessage(key, msg)
	}
}

func numberFactory(build func(float64) Rule) Factory {
	return func(p []string) (Rule, error) {
		if len(p) != 1 {
			return nil, fmt.Errorf("needs one number")
		}
		n, err := strconv.ParseFloat(p[0], 64)
		if err != nil {
			return nil, fmt.Errorf("needs one number: %w", err)
		}
		return build(n), nil
	}
}

// --- presence -------------------------------------------------------------

type requiredRule struct{}

// Required fails when the field is missing, nil, a blank string or an empty
// array or map.
func Required() Rule { return requiredRule{} }

func (requiredRule) Name() string   { return "required" }
func (requiredRule) Implicit() bool { return true }
func (requiredRule) Validate(_ context.Context, f Field) error {
	if !f.Present || isEmpty(f.Value) {
		return Fail("required", nil)
	}
	return nil
}

type requiredIfRule struct {
	other  string
	values []string
}

// RequiredIf makes the field required when another field equals one of
// values. Wildcards in other refer to the same position as this field.
func RequiredIf(other string, values ...string) Rule {
	return requiredIfRule{other: other, values: values}
}

func (requiredIfRule) Name() string   { return "required_if" }
func (requiredIfRule) Implicit() bool { return true }
func (r requiredIfRule) Validate(_ context.Context, f Field) error {
	v, ok := f.Lookup(r.other)
	if !ok || !containsString(r.values, toString(v)) {
		return nil
	}
	if !f.Present || isEmpty(f.Value) {
		return Fail("required_if", map[string]any{"other": r.other, "value": toString(v)})
	}
	return nil
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

// --- types ----------------------------------------------------------------

// String fails unless the value is a string.
func String() Rule {
	return NewRule("string", func(_ context.Context, f Field) error {
		if _, ok := f.Value.(string); !ok {
			return Fail("string", nil)
		}
		return nil
	})
}

// Integer passes Go integers, whole floats (as JSON numbers decode) and
// strings holding an integer.
func Integer() Rule {
	return NewRule("integer", func(_ context.Context, f Field) error {
		if _, ok := toInt(f.Value); !ok {
			return Fail("integer", nil)
		}
		return nil
	})
}

// Numeric passes Go numbers and numeric strings.
func Numeric() Rule {
	return NewRule("numeric", func(_ context.Context, f Field) error {
		if _, ok := toFloat(f.Value); !ok {
			return Fail("numeric", nil)
		}
		return nil
	})
}

// Boolean passes true, false, 1, 0, "1", "0", "true" and "false".
func Boolean() Rule {
	return NewRule("boolean", func(_ context.Context, f Field) error {
		switch v := f.Value.(type) {
		case bool:
			return nil
		case string:
			if containsString([]string{"1", "0", "true", "false"}, v) {
				return nil
			}
		default:
			if n, ok := toFloat(v); ok && (n == 0 || n == 1) {
				return nil
			}
		}
		return Fail("boolean", nil)
	})
}

// Array passes lists and maps.
func Array() Rule {
	return NewRule("array", func(_ context.Context, f Field) error {
		switch f.Value.(type) {
		case []any, map[string]any:
			return nil
		}
		return Fail("array", nil)
	})
}

// --- formats --------------------------------------------------------------

// Email passes a bare email address such as "jane@example.com".
func Email() Rule {
	return NewRule("email", func(_ context.Context, f Field) error {
		s, ok := f.Value.(string)
		if !ok {
			return Fail("email", nil)
		}
		addr, err := mail.ParseAddress(s)
		if err != nil || addr.Address != s {
			return Fail("email", nil)
		}
		return nil
	})
}

// --- sizes ----------------------------------------------------------------

// sizeOf measures a value the way Laravel's size rules do: numbers by value,
// strings by character count (or by value when the field also has the
// numeric or integer rule), arrays by length.
func sizeOf(f Field) (size float64, kind string, ok bool) {
	switch v := f.Value.(type) {
	case string:
		if f.HasRule("numeric") || f.HasRule("integer") {
			n, ok := toFloat(v)
			return n, "numeric", ok
		}
		return float64(utf8.RuneCountInString(v)), "string", true
	case []any:
		return float64(len(v)), "array", true
	case map[string]any:
		return float64(len(v)), "array", true
	}
	n, ok := toFloat(f.Value)
	return n, "numeric", ok
}

type sizeRule struct {
	name   string
	lo, hi float64
	hasLo  bool
	hasHi  bool
}

func (r sizeRule) Name() string { return r.name }
func (r sizeRule) Validate(_ context.Context, f Field) error {
	size, kind, ok := sizeOf(f)
	params := map[string]any{"min": r.lo, "max": r.hi}
	if !ok {
		return Fail(r.name+".numeric", params)
	}
	if (r.hasLo && size < r.lo) || (r.hasHi && size > r.hi) {
		return Fail(r.name+"."+kind, params)
	}
	return nil
}

// Min fails when the value's size is below n.
func Min(n float64) Rule { return sizeRule{name: "min", lo: n, hasLo: true} }

// Max fails when the value's size is above n.
func Max(n float64) Rule { return sizeRule{name: "max", hi: n, hasHi: true} }

// Between fails when the value's size is outside [lo, hi].
func Between(lo, hi float64) Rule {
	return sizeRule{name: "between", lo: lo, hi: hi, hasLo: true, hasHi: true}
}

// --- comparisons ----------------------------------------------------------

// In fails unless the value, as text, is one of values.
func In(values ...string) Rule {
	vals := append([]string(nil), values...)
	return NewRule("in", func(_ context.Context, f Field) error {
		if !containsString(vals, toString(f.Value)) {
			return Fail("in", map[string]any{"values": vals})
		}
		return nil
	})
}

// Same fails unless the value equals the value of another field.
func Same(other string) Rule {
	return NewRule("same", func(_ context.Context, f Field) error {
		o, _ := f.Lookup(other)
		if !reflect.DeepEqual(o, f.Value) {
			return Fail("same", map[string]any{"other": other})
		}
		return nil
	})
}

// --- helpers --------------------------------------------------------------

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return formatFloat(t)
	case float32:
		return formatFloat(float64(t))
	}
	return fmt.Sprint(v)
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func toFloat(v any) (float64, bool) {
	if s, ok := v.(string); ok {
		n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return n, err == nil
	}
	if _, isBool := v.(bool); isBool {
		return 0, false
	}
	return As[float64](v)
}

func toInt(v any) (int64, bool) {
	if s, ok := v.(string); ok {
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		return n, err == nil
	}
	return As[int64](v)
}
