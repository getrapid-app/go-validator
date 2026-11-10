package validator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// Rule is the single contract every validation rule implements. Built-in
// rules and user-defined rules use exactly the same interface.
//
// Validate returns nil when the field passes. To report a validation failure,
// return a Failure (see Fail) so the message can be translated and customised,
// or any other error whose text becomes the message. To stop validation
// because of an infrastructure problem (a database is down, for example),
// return Abort(err): the whole validation then returns err unchanged.
type Rule interface {
	// Name is the rule's identifier, such as "required" or "max". It is used to
	// look up messages and to answer Field.HasRule.
	Name() string
	Validate(ctx context.Context, f Field) error
}

// ImplicitRule is implemented by rules that must run even when the field is
// missing or an empty string, such as "required" or "required_if". Every
// other rule is skipped for missing and empty values, as in Laravel.
type ImplicitRule interface {
	Rule
	Implicit() bool
}

// RuleSet is anything that produces a list of rules for one field: a parsed
// rule string (Parse), a fluent chain (Chain) or a plain list (List). Rules
// are built against the validator's Registry so custom rules resolve too.
type RuleSet interface {
	Build(r *Registry) ([]Rule, error)
}

// Rules maps a field path to its rules. Paths use dot notation and may
// contain wildcards: "email", "address.city", "items.*.price".
//
// A value can be any of:
//
//	[]string{"required", "email", "max:255"}  // one rule per element
//	"required|email|max:255"                  // Laravel pipe syntax
//	validator.Chain().Required().Email()      // fluent chain (any RuleSet)
//	validator.Required()                      // a single Rule
//	[]validator.Rule{...}                     // several Rule values
//	[]any{"required", myRule, "max:255"}      // a mix of the above
//
// Elements of a []string are not split on "|", so a rule such as
// "regex:/^(a|b)$/" is safe there.
type Rules map[string]any

// toRules builds the rules for one field from any of the forms Rules accepts.
func toRules(spec any, r *Registry) ([]Rule, error) {
	switch s := spec.(type) {
	case nil:
		return nil, nil
	case RuleSet:
		return s.Build(r)
	case Rule:
		return []Rule{s}, nil
	case string:
		return parsed(s).Build(r)
	case []string:
		out := make([]Rule, 0, len(s))
		for _, one := range s {
			rule, err := parseOne(one, r)
			if err != nil {
				return nil, err
			}
			if rule != nil {
				out = append(out, rule)
			}
		}
		return out, nil
	case []Rule:
		return s, nil
	case []RuleSet:
		var out []Rule
		for _, set := range s {
			rules, err := set.Build(r)
			if err != nil {
				return nil, err
			}
			out = append(out, rules...)
		}
		return out, nil
	case []any:
		var out []Rule
		for _, item := range s {
			if str, ok := item.(string); ok {
				item = []string{str}
			}
			rules, err := toRules(item, r)
			if err != nil {
				return nil, err
			}
			out = append(out, rules...)
		}
		return out, nil
	}
	return nil, fmt.Errorf("validator: unsupported rules value of type %T", spec)
}

// Failure is a validation failure that carries a message key and the values
// for its placeholders. The key is usually the rule name, or a variant such
// as "max.string" for rules whose message depends on the value's type.
type Failure struct {
	Key    string
	Params map[string]any
}

func (f *Failure) Error() string { return "validation failed: " + f.Key }

// Fail returns a Failure for key with optional placeholder values.
func Fail(key string, params map[string]any) error {
	return &Failure{Key: key, Params: params}
}

type abortError struct{ err error }

func (a *abortError) Error() string { return a.err.Error() }
func (a *abortError) Unwrap() error { return a.err }

// Abort wraps err so that validation stops and returns err as-is instead of
// recording a field failure.
func Abort(err error) error {
	if err == nil {
		return nil
	}
	return &abortError{err: err}
}

// funcRule adapts a function to the Rule interface.
type funcRule struct {
	name     string
	implicit bool
	fn       func(ctx context.Context, f Field) error
}

func (r *funcRule) Name() string                                { return r.name }
func (r *funcRule) Implicit() bool                              { return r.implicit }
func (r *funcRule) Validate(ctx context.Context, f Field) error { return r.fn(ctx, f) }

// NewRule creates a rule from a function.
func NewRule(name string, fn func(ctx context.Context, f Field) error) Rule {
	return &funcRule{name: name, fn: fn}
}

// RuleFor creates a rule whose function receives the field value already
// converted to T. When the value cannot be converted, the rule fails with
// its own name as the message key.
//
//	even := validator.RuleFor("even", func(ctx context.Context, n int, f validator.Field) error {
//		if n%2 != 0 {
//			return validator.Fail("even", nil)
//		}
//		return nil
//	})
func RuleFor[T any](name string, fn func(ctx context.Context, value T, f Field) error) Rule {
	return &funcRule{name: name, fn: func(ctx context.Context, f Field) error {
		v, ok := As[T](f.Value)
		if !ok {
			return Fail(name, nil)
		}
		return fn(ctx, v, f)
	}}
}

// As converts v to T. It accepts values that already are T, converts
// between string types (so "active" converts to a named type Status), and
// converts between numeric kinds when no information is lost (so a JSON float64 of
// 3 converts to int, while 3.5 does not).
func As[T any](v any) (T, bool) {
	var zero T
	if t, ok := v.(T); ok {
		return t, true
	}
	if v == nil {
		return zero, false
	}
	// Fast paths for the conversions rules use most, without reflection.
	switch any(zero).(type) {
	case int64:
		if n, ok, known := exactInt64(v); known {
			return any(n).(T), ok
		}
	case int:
		if n, ok, known := exactInt64(v); known {
			if !ok || int64(int(n)) != n {
				return zero, false
			}
			return any(int(n)).(T), true
		}
	case float64:
		if n, ok, known := exactFloat64(v); known {
			return any(n).(T), ok
		}
	}
	target := reflect.TypeFor[T]()
	rv := reflect.ValueOf(v)
	// Named string types, such as enums: type Status string.
	if rv.Kind() == reflect.String && target.Kind() == reflect.String {
		return rv.Convert(target).Interface().(T), true
	}
	if !isNumberKind(rv.Kind()) || !isNumberKind(target.Kind()) {
		return zero, false
	}
	converted := rv.Convert(target)
	back := converted.Convert(rv.Type())
	if !reflect.DeepEqual(back.Interface(), v) {
		return zero, false
	}
	return converted.Interface().(T), true
}

func isNumberKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func isImplicit(r Rule) bool {
	ir, ok := r.(ImplicitRule)
	return ok && ir.Implicit()
}

func isAbort(err error) (error, bool) {
	if a, ok := errors.AsType[*abortError](err); ok {
		return a.err, true
	}
	return nil, false
}

// list is the RuleSet returned by List.
type list []Rule

func (l list) Build(*Registry) ([]Rule, error) { return l, nil }

// List returns a RuleSet made of the given rules, in order.
func List(rules ...Rule) RuleSet { return list(rules) }

// flagRule is a rule that changes how the other rules run instead of
// checking anything itself: bail, nullable and sometimes.
type flagRule string

func (f flagRule) Name() string                        { return string(f) }
func (flagRule) Validate(context.Context, Field) error { return nil }
func (f flagRule) String() string                      { return fmt.Sprintf("flag(%s)", string(f)) }

// exactInt64 converts a built-in number to int64 when no information is
// lost, matching what As does through reflection. known is false for types
// it does not handle, which then take the reflection path.
func exactInt64(v any) (n int64, ok, known bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true, true
	case int8:
		return int64(t), true, true
	case int16:
		return int64(t), true, true
	case int32:
		return int64(t), true, true
	case uint:
		return int64(t), uint(int64(t)) == t, true
	case uint8:
		return int64(t), true, true
	case uint16:
		return int64(t), true, true
	case uint32:
		return int64(t), true, true
	case uint64:
		return int64(t), uint64(int64(t)) == t, true
	case float32:
		n := int64(t)
		return n, float32(n) == t, true
	case float64:
		n := int64(t)
		return n, float64(n) == t, true
	}
	return 0, false, false
}

// exactFloat64 is exactInt64 for float64.
func exactFloat64(v any) (n float64, ok, known bool) {
	switch t := v.(type) {
	case int:
		f := float64(t)
		return f, int(f) == t, true
	case int8:
		return float64(t), true, true
	case int16:
		return float64(t), true, true
	case int32:
		return float64(t), true, true
	case int64:
		f := float64(t)
		return f, int64(f) == t, true
	case uint:
		f := float64(t)
		return f, uint(f) == t, true
	case uint8:
		return float64(t), true, true
	case uint16:
		return float64(t), true, true
	case uint32:
		return float64(t), true, true
	case uint64:
		f := float64(t)
		return f, uint64(f) == t, true
	case float32:
		f := float64(t)
		return f, float32(f) == t, true
	}
	return 0, false, false
}
