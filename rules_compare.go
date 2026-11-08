package validator

import (
	"context"
	"reflect"
	"strings"
)

func registerCompare(r *Registry) {
	r.Register("same", oneField(Same)).
		Register("different", oneField(Different)).
		Register("confirmed", func(p []string) (Rule, error) { return Confirmed(p...), nil }).
		Register("in", values(In)).
		Register("not_in", values(NotIn)).
		Register("in_array", oneField(InArray)).
		Register("in_array_keys", atLeast(1, "at least one key", func(p []string) (Rule, error) { return InArrayKeys(p...), nil })).
		Register("distinct", func(p []string) (Rule, error) {
			return distinctRule(containsString(p, "strict"), containsString(p, "ignore_case")), nil
		}).
		Register("contains", atLeast(1, "at least one value", func(p []string) (Rule, error) { return Contains(p...), nil })).
		Register("doesnt_contain", atLeast(1, "at least one value", func(p []string) (Rule, error) { return DoesntContain(p...), nil }))
}

// Same fails unless the value equals the value of another field.
func Same(other string) Rule {
	return check("same", map[string]any{"other": other}, func(f Field) bool {
		o, _ := f.Lookup(other)
		return reflect.DeepEqual(o, f.Value)
	})
}

// Different fails when the value equals the value of another field, or
// when the other field is missing, as in Laravel.
func Different(other string) Rule {
	return check("different", map[string]any{"other": other}, func(f Field) bool {
		o, ok := f.Lookup(other)
		return ok && !reflect.DeepEqual(o, f.Value)
	})
}

// Confirmed fails unless a matching "<field>_confirmation" field exists, as
// for password confirmation. Pass a field name to use a different one.
func Confirmed(field ...string) Rule {
	return &funcRule{name: "confirmed", fn: func(_ context.Context, f Field) error {
		other := f.Pattern + "_confirmation"
		if len(field) > 0 && field[0] != "" {
			other = field[0]
		}
		o, ok := f.Lookup(other)
		if !ok || !reflect.DeepEqual(o, f.Value) {
			return Fail("confirmed", nil)
		}
		return nil
	}}
}

// In fails unless the value, as text, is one of values. For a list, every
// item must be one of values.
func In(values ...string) Rule {
	vals := append([]string(nil), values...)
	return check("in", map[string]any{"values": vals}, func(f Field) bool {
		if items, ok := f.Value.([]any); ok {
			for _, it := range items {
				if !matchesAny(it, vals) {
					return false
				}
			}
			return true
		}
		return matchesAny(f.Value, vals)
	})
}

// NotIn fails when the value, as text, is one of values.
func NotIn(values ...string) Rule {
	vals := append([]string(nil), values...)
	return check("not_in", map[string]any{"values": vals}, func(f Field) bool {
		if items, ok := f.Value.([]any); ok {
			for _, it := range items {
				if matchesAny(it, vals) {
					return false
				}
			}
			return true
		}
		return !matchesAny(f.Value, vals)
	})
}

// InArray passes values found among the values of another field, written
// with a wildcard: in_array:allowed.*.
func InArray(other string) Rule {
	return check("in_array", map[string]any{"other": other}, func(f Field) bool {
		pattern := resolveRelative(f.Pattern, f.Path, other)
		if !strings.Contains(pattern, "*") {
			pattern += ".*"
		}
		for _, p := range expand(f.root, pattern) {
			if v, ok := lookup(f.root, p); ok && reflect.DeepEqual(v, f.Value) {
				return true
			}
		}
		return false
	})
}

// InArrayKeys passes maps that hold at least one of keys.
func InArrayKeys(keys ...string) Rule {
	return check("in_array_keys", map[string]any{"values": keys}, func(f Field) bool {
		m, ok := f.Value.(map[string]any)
		if !ok {
			return false
		}
		for _, k := range keys {
			if _, ok := m[k]; ok {
				return true
			}
		}
		return false
	})
}

// Distinct fails when another item matched by the same wildcard pattern has
// the same value, as in "items.*.id": "distinct".
func Distinct() Rule { return distinctRule(false, false) }

// DistinctIgnoreCase is like Distinct, comparing strings case-insensitively.
func DistinctIgnoreCase() Rule { return distinctRule(false, true) }

// DistinctStrict is like Distinct, but 1 and "1" count as different.
func DistinctStrict() Rule { return distinctRule(true, false) }

func distinctRule(strict, ignoreCase bool) Rule {
	key := func(v any) any {
		if !strict {
			v = toString(v)
		}
		if s, ok := v.(string); ok && ignoreCase {
			return strings.ToLower(s)
		}
		return v
	}
	return check("distinct", nil, func(f Field) bool {
		if !strings.Contains(f.Pattern, "*") {
			return true
		}
		// Siblings share this field's concrete path except at the last
		// wildcard, so items.0.id is compared with items.*.id.
		mine := key(f.Value)
		for _, p := range expand(f.root, siblingPattern(f.Pattern, f.Path)) {
			if p == f.Path {
				continue
			}
			if v, ok := lookup(f.root, p); ok && reflect.DeepEqual(key(v), mine) {
				return false
			}
		}
		return true
	})
}

// siblingPattern keeps every wildcard of pattern resolved from path except
// the last one.
func siblingPattern(pattern, path string) string {
	pSegs, cSegs := strings.Split(pattern, "."), strings.Split(path, ".")
	last := -1
	for i, s := range pSegs {
		if s == "*" {
			last = i
		}
	}
	out := make([]string, len(pSegs))
	for i, s := range pSegs {
		if s == "*" && i != last && i < len(cSegs) {
			out[i] = cSegs[i]
		} else {
			out[i] = s
		}
	}
	return strings.Join(out, ".")
}

// Contains passes lists that contain every one of values.
func Contains(values ...string) Rule {
	return check("contains", map[string]any{"values": values}, func(f Field) bool {
		items, ok := asStrings(f.Value)
		if !ok {
			return false
		}
		for _, v := range values {
			if !containsString(items, v) {
				return false
			}
		}
		return true
	})
}

// DoesntContain passes lists that contain none of values.
func DoesntContain(values ...string) Rule {
	return check("doesnt_contain", map[string]any{"values": values}, func(f Field) bool {
		items, ok := asStrings(f.Value)
		if !ok {
			return false
		}
		for _, v := range values {
			if containsString(items, v) {
				return false
			}
		}
		return true
	})
}

// AnyOf passes when the value passes at least one of the rule sets, like
// Laravel's Rule::anyOf. Each set can be anything a Rules value accepts.
//
//	validator.AnyOf([]string{"string", "email"}, []string{"integer", "min:1"})
func AnyOf(sets ...any) Rule {
	return &funcRule{name: "any_of", fn: func(ctx context.Context, f Field) error {
		reg := f.validator().registry
		if reg == nil {
			reg = DefaultRegistry
		}
		for _, set := range sets {
			rules, err := toRules(set, reg)
			if err != nil {
				return Abort(err)
			}
			err = f.runRules(ctx, rules)
			if err == nil {
				return nil
			}
			if _, ok := isAbort(err); ok {
				return err
			}
		}
		return Fail("any_of", nil)
	}}
}

// When adds rules only when cond is true, like Laravel's Rule::when. The
// optional otherwise rules apply when it is false.
//
//	"coupon": validator.When(isCheckout, []string{"required", "alpha_num"})
func When(cond bool, rules any, otherwise ...any) RuleSet {
	return whenSet{cond: cond, rules: rules, otherwise: otherwise}
}

type whenSet struct {
	cond      bool
	rules     any
	otherwise []any
}

func (w whenSet) Build(r *Registry) ([]Rule, error) {
	if w.cond {
		return toRules(w.rules, r)
	}
	return toRules(w.otherwise, r)
}
