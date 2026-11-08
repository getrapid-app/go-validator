package validator

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Built-in rules follow one pattern: an exported constructor for the fluent
// API (Required(), Max(255), ...), a value implementing Rule, and a factory
// registered under the Laravel name so the string syntax and struct tags
// resolve to the very same rule. The rules are split by kind:
//
//	rules_presence.go  required*, present*, missing*, prohibited*, exclude*, accepted, declined, filled
//	rules_types.go     string, integer, numeric, boolean, array, list, json, enum, ...
//	rules_strings.go   alpha*, email, url, uuid, ip, regex, starts_with, case, ...
//	rules_numbers.go   size, min, max, between, gt*, lt*, digits*, decimal, multiple_of
//	rules_dates.go     date, date_format, after*, before*, date_equals, timezone
//	rules_compare.go   same, different, confirmed, in, not_in, in_array, distinct, contains, ...
//	rules_files.go     file, image, mimes, mimetypes, extensions, dimensions
//	rules_db.go        exists, unique
//	rules_password.go  password, current_password

const (
	flagBail       flagRule = "bail"
	flagNullable   flagRule = "nullable"
	flagSometimes  flagRule = "sometimes"
	flagAcceptZero flagRule = "accept_zero"
)

// Bail stops validating a field after its first failure.
func Bail() Rule { return flagBail }

// Nullable lets a field be nil; the other rules are then skipped.
func Nullable() Rule { return flagNullable }

// Sometimes only validates a field when it is present in the input.
func Sometimes() Rule { return flagSometimes }

// AcceptZero makes the number 0 count as a value, so that required and the
// other presence rules accept it. Without it, 0 counts as empty, which is
// what a Go struct's zero value usually means.
func AcceptZero() Rule { return flagAcceptZero }

func registerBuiltins(r *Registry) {
	for _, f := range []flagRule{flagBail, flagNullable, flagSometimes, flagAcceptZero} {
		r.Register(string(f), fixed(f))
	}
	registerPresence(r)
	registerTypes(r)
	registerStrings(r)
	registerNumbers(r)
	registerDates(r)
	registerCompare(r)
	registerFiles(r)
	registerDB(r)
	registerPassword(r)
	for key, msg := range englishMessages {
		r.SetMessage(key, msg)
	}
}

// --- factory helpers ------------------------------------------------------

// fixed is a factory for rules that take no parameters.
func fixed(rule Rule) Factory {
	return func([]string) (Rule, error) { return rule, nil }
}

// atLeast wraps a factory with a minimum parameter count check.
func atLeast(n int, what string, build func(p []string) (Rule, error)) Factory {
	return func(p []string) (Rule, error) {
		if len(p) < n {
			return nil, fmt.Errorf("needs %s", what)
		}
		return build(p)
	}
}

// fieldAndValues is the factory shape of rules like required_if:field,v1,v2.
func fieldAndValues(build func(other string, values ...string) Rule) Factory {
	return atLeast(2, "a field and at least one value", func(p []string) (Rule, error) {
		return build(p[0], p[1:]...), nil
	})
}

// oneField is the factory shape of rules like same:field.
func oneField(build func(other string) Rule) Factory {
	return atLeast(1, "a field name", func(p []string) (Rule, error) { return build(p[0]), nil })
}

// fields is the factory shape of rules like required_with:a,b.
func fields(build func(others ...string) Rule) Factory {
	return atLeast(1, "at least one field name", func(p []string) (Rule, error) { return build(p...), nil })
}

// values is the factory shape of rules like in:a,b.
func values(build func(values ...string) Rule) Factory {
	return func(p []string) (Rule, error) { return build(p...), nil }
}

func oneNumber(build func(float64) Rule) Factory {
	return atLeast(1, "a number", func(p []string) (Rule, error) {
		n, err := strconv.ParseFloat(p[0], 64)
		if err != nil {
			return nil, fmt.Errorf("needs a number, got %q", p[0])
		}
		return build(n), nil
	})
}

func twoNumbers(build func(a, b float64) Rule) Factory {
	return atLeast(2, "two numbers", func(p []string) (Rule, error) {
		a, err1 := strconv.ParseFloat(p[0], 64)
		b, err2 := strconv.ParseFloat(p[1], 64)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("needs two numbers, got %q", strings.Join(p, ","))
		}
		return build(a, b), nil
	})
}

func oneInt(build func(int) Rule) Factory {
	return atLeast(1, "a whole number", func(p []string) (Rule, error) {
		n, err := strconv.Atoi(p[0])
		if err != nil {
			return nil, fmt.Errorf("needs a whole number, got %q", p[0])
		}
		return build(n), nil
	})
}

func twoInts(build func(a, b int) Rule) Factory {
	return atLeast(2, "two whole numbers", func(p []string) (Rule, error) {
		a, err1 := strconv.Atoi(p[0])
		b, err2 := strconv.Atoi(p[1])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("needs two whole numbers, got %q", strings.Join(p, ","))
		}
		return build(a, b), nil
	})
}

// check builds a non-implicit rule from a predicate on the field. When ok
// is false the rule fails with its own name and params.
func check(name string, params map[string]any, ok func(f Field) bool) Rule {
	return &funcRule{name: name, fn: func(_ context.Context, f Field) error {
		if !ok(f) {
			return Fail(name, params)
		}
		return nil
	}}
}

// implicitCheck is like check for implicit rules.
func implicitCheck(name string, params func(f Field) map[string]any, ok func(f Field) bool) Rule {
	return &funcRule{name: name, implicit: true, fn: func(_ context.Context, f Field) error {
		if !ok(f) {
			var p map[string]any
			if params != nil {
				p = params(f)
			}
			return Fail(name, p)
		}
		return nil
	}}
}

// --- value helpers --------------------------------------------------------

// attrNames is a placeholder value holding field names; each one is shown
// with its display name (WithAttributes) in messages.
type attrNames []string

func isBlankValue(v any, acceptZero bool) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	case File:
		return t.FileName() == "" && t.FileSize() == 0
	case bool:
		return false
	}
	if !acceptZero {
		rv := reflect.ValueOf(v)
		if isNumberKind(rv.Kind()) && rv.IsZero() {
			return true
		}
	}
	return false
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// matchesAny reports whether v, compared as text the way Laravel does,
// equals one of values. "null" matches nil, and booleans match "true",
// "false", "1" and "0".
func matchesAny(v any, values []string) bool {
	if v == nil {
		return containsString(values, "null")
	}
	if b, ok := v.(bool); ok {
		if b {
			return containsString(values, "true") || containsString(values, "1")
		}
		return containsString(values, "false") || containsString(values, "0")
	}
	return containsString(values, toString(v))
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
	case fmt.Stringer:
		return t.String()
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.String {
		return rv.String()
	}
	return fmt.Sprint(v)
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return n, err == nil && strings.TrimSpace(t) != ""
	case bool, nil:
		return 0, false
	}
	return As[float64](v)
}

func toInt(v any) (int64, bool) {
	if s, ok := v.(string); ok {
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		return n, err == nil
	}
	if _, ok := v.(bool); ok {
		return 0, false
	}
	return As[int64](v)
}

// asStrings turns a list value into strings, for rules like contains.
func asStrings(v any) ([]string, bool) {
	switch t := v.(type) {
	case []any:
		out := make([]string, len(t))
		for i, x := range t {
			out[i] = toString(x)
		}
		return out, true
	case map[string]any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, toString(x))
		}
		return out, true
	}
	return nil, false
}
