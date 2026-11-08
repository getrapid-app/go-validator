package validator

import (
	"context"
)

func registerPresence(r *Registry) {
	r.Register("required", fixed(Required())).
		Register("required_if", fieldAndValues(RequiredIf)).
		Register("required_unless", fieldAndValues(RequiredUnless)).
		Register("required_with", fields(RequiredWith)).
		Register("required_with_all", fields(RequiredWithAll)).
		Register("required_without", fields(RequiredWithout)).
		Register("required_without_all", fields(RequiredWithoutAll)).
		Register("required_if_accepted", oneField(RequiredIfAccepted)).
		Register("required_if_declined", oneField(RequiredIfDeclined)).
		Register("required_array_keys", atLeast(1, "at least one key", func(p []string) (Rule, error) { return RequiredArrayKeys(p...), nil })).
		Register("filled", fixed(Filled())).
		Register("present", fixed(Present())).
		Register("present_if", fieldAndValues(PresentIf)).
		Register("present_unless", fieldAndValues(PresentUnless)).
		Register("present_with", fields(PresentWith)).
		Register("present_with_all", fields(PresentWithAll)).
		Register("missing", fixed(Missing())).
		Register("missing_if", fieldAndValues(MissingIf)).
		Register("missing_unless", fieldAndValues(MissingUnless)).
		Register("missing_with", fields(MissingWith)).
		Register("missing_with_all", fields(MissingWithAll)).
		Register("prohibited", fixed(Prohibited())).
		Register("prohibited_if", fieldAndValues(ProhibitedIf)).
		Register("prohibited_unless", fieldAndValues(ProhibitedUnless)).
		Register("prohibited_if_accepted", oneField(ProhibitedIfAccepted)).
		Register("prohibited_if_declined", oneField(ProhibitedIfDeclined)).
		Register("prohibits", fields(Prohibits)).
		Register("accepted", fixed(Accepted())).
		Register("accepted_if", fieldAndValues(AcceptedIf)).
		Register("declined", fixed(Declined())).
		Register("declined_if", fieldAndValues(DeclinedIf)).
		Register("exclude", fixed(Exclude())).
		Register("exclude_if", fieldAndValues(ExcludeIf)).
		Register("exclude_unless", fieldAndValues(ExcludeUnless)).
		Register("exclude_with", oneField(ExcludeWith)).
		Register("exclude_without", oneField(ExcludeWithout))
}

// --- conditions shared by the presence rules -------------------------------

// otherIs reports whether another field's value matches one of values.
func otherIs(f Field, other string, values []string) (any, bool) {
	v, _ := f.Lookup(other)
	return v, matchesAny(v, values)
}

// otherFilled reports whether another field has a value. The other field's
// own rules are unknown here, so 0 counts as a value, as in Laravel.
func otherFilled(f Field, other string) bool {
	o := f.other(other)
	return o.Present && !isBlankValue(o.Value, true)
}

func otherPresent(f Field, other string) bool { _, ok := f.Lookup(other); return ok }

func anyFilled(f Field, others []string) bool {
	for _, o := range others {
		if otherFilled(f, o) {
			return true
		}
	}
	return false
}

func allFilled(f Field, others []string) bool {
	for _, o := range others {
		if !otherFilled(f, o) {
			return false
		}
	}
	return true
}

func anyPresent(f Field, others []string) bool {
	for _, o := range others {
		if otherPresent(f, o) {
			return true
		}
	}
	return false
}

func allPresent(f Field, others []string) bool {
	for _, o := range others {
		if !otherPresent(f, o) {
			return false
		}
	}
	return true
}

var (
	acceptedValues = []string{"yes", "on", "1", "true"}
	declinedValues = []string{"no", "off", "0", "false"}
)

func isAccepted(v any) bool { return v != nil && matchesAny(v, acceptedValues) }
func isDeclined(v any) bool { return v != nil && matchesAny(v, declinedValues) }

// conditional builds an implicit rule that applies test only when cond
// holds. params supplies the message placeholders.
func conditional(name string, cond func(f Field) bool, test func(f Field) bool, params func(f Field) map[string]any) Rule {
	return implicitCheck(name, params, func(f Field) bool { return !cond(f) || test(f) })
}

func otherValueParams(other string) func(f Field) map[string]any {
	return func(f Field) map[string]any {
		v, _ := f.Lookup(other)
		return map[string]any{"other": other, "value": toString(v)}
	}
}

func otherValuesParams(other string, values []string) func(Field) map[string]any {
	return func(Field) map[string]any { return map[string]any{"other": other, "values": values} }
}

func namesParams(others []string) func(Field) map[string]any {
	return func(Field) map[string]any { return map[string]any{"values": attrNames(others)} }
}

func notBlank(f Field) bool  { return !f.Blank() }
func isPresent(f Field) bool { return f.Present }
func isMissing(f Field) bool { return !f.Present }
func blank(f Field) bool     { return f.Blank() }

// --- required --------------------------------------------------------------

// Required fails when the field is missing, nil, a blank string, an empty
// array or map, or the number 0 (unless the field has AcceptZero).
func Required() Rule { return implicitCheck("required", nil, notBlank) }

// RequiredIf makes the field required when another field equals one of
// values. Wildcards in other refer to the same position as this field.
func RequiredIf(other string, values ...string) Rule {
	return conditional("required_if", func(f Field) bool { _, ok := otherIs(f, other, values); return ok }, notBlank, otherValueParams(other))
}

// RequiredIfFunc makes the field required when cond returns true, like
// Laravel's Rule::requiredIf(callback).
func RequiredIfFunc(cond func(ctx context.Context, f Field) bool) Rule {
	return &funcRule{name: "required", implicit: true, fn: func(ctx context.Context, f Field) error {
		if cond(ctx, f) && f.Blank() {
			return Fail("required", nil)
		}
		return nil
	}}
}

// RequiredUnless makes the field required unless another field equals one
// of values.
func RequiredUnless(other string, values ...string) Rule {
	return conditional("required_unless", func(f Field) bool { _, ok := otherIs(f, other, values); return !ok }, notBlank, otherValuesParams(other, values))
}

// RequiredWith makes the field required when any of the other fields is
// filled.
func RequiredWith(others ...string) Rule {
	return conditional("required_with", func(f Field) bool { return anyFilled(f, others) }, notBlank, namesParams(others))
}

// RequiredWithAll makes the field required when all of the other fields
// are filled.
func RequiredWithAll(others ...string) Rule {
	return conditional("required_with_all", func(f Field) bool { return allFilled(f, others) }, notBlank, namesParams(others))
}

// RequiredWithout makes the field required when any of the other fields is
// empty or missing.
func RequiredWithout(others ...string) Rule {
	return conditional("required_without", func(f Field) bool { return !allFilled(f, others) }, notBlank, namesParams(others))
}

// RequiredWithoutAll makes the field required when all of the other fields
// are empty or missing.
func RequiredWithoutAll(others ...string) Rule {
	return conditional("required_without_all", func(f Field) bool { return !anyFilled(f, others) }, notBlank, namesParams(others))
}

// RequiredIfAccepted makes the field required when another field is
// accepted ("yes", "on", 1, true).
func RequiredIfAccepted(other string) Rule {
	return conditional("required_if_accepted", func(f Field) bool { v, _ := f.Lookup(other); return isAccepted(v) }, notBlank,
		func(Field) map[string]any { return map[string]any{"other": other} })
}

// RequiredIfDeclined makes the field required when another field is
// declined ("no", "off", 0, false).
func RequiredIfDeclined(other string) Rule {
	return conditional("required_if_declined", func(f Field) bool { v, _ := f.Lookup(other); return isDeclined(v) }, notBlank,
		func(Field) map[string]any { return map[string]any{"other": other} })
}

// RequiredArrayKeys fails unless the value is a map holding every key.
func RequiredArrayKeys(keys ...string) Rule {
	return check("required_array_keys", map[string]any{"values": keys}, func(f Field) bool {
		m, ok := f.Value.(map[string]any)
		if !ok {
			return false
		}
		for _, k := range keys {
			if _, ok := m[k]; !ok {
				return false
			}
		}
		return true
	})
}

// Filled fails when the field is present but empty.
func Filled() Rule {
	return implicitCheck("filled", nil, func(f Field) bool { return !f.Present || !f.Blank() })
}

// --- present / missing -----------------------------------------------------

// Present fails unless the field exists in the input; it may be empty.
func Present() Rule { return implicitCheck("present", nil, isPresent) }

// PresentIf requires the field to exist when another field equals one of
// values.
func PresentIf(other string, values ...string) Rule {
	return conditional("present_if", func(f Field) bool { _, ok := otherIs(f, other, values); return ok }, isPresent, otherValueParams(other))
}

// PresentUnless requires the field to exist unless another field equals
// one of values.
func PresentUnless(other string, values ...string) Rule {
	return conditional("present_unless", func(f Field) bool { _, ok := otherIs(f, other, values); return !ok }, isPresent, otherValueParams(other))
}

// PresentWith requires the field to exist when any of the others exists.
func PresentWith(others ...string) Rule {
	return conditional("present_with", func(f Field) bool { return anyPresent(f, others) }, isPresent, namesParams(others))
}

// PresentWithAll requires the field to exist when all of the others exist.
func PresentWithAll(others ...string) Rule {
	return conditional("present_with_all", func(f Field) bool { return allPresent(f, others) }, isPresent, namesParams(others))
}

// Missing fails when the field exists in the input.
func Missing() Rule { return implicitCheck("missing", nil, isMissing) }

// MissingIf requires the field to be absent when another field equals one
// of values.
func MissingIf(other string, values ...string) Rule {
	return conditional("missing_if", func(f Field) bool { _, ok := otherIs(f, other, values); return ok }, isMissing, otherValueParams(other))
}

// MissingUnless requires the field to be absent unless another field
// equals one of values.
func MissingUnless(other string, values ...string) Rule {
	return conditional("missing_unless", func(f Field) bool { _, ok := otherIs(f, other, values); return !ok }, isMissing,
		func(Field) map[string]any { return map[string]any{"other": other, "value": values} })
}

// MissingWith requires the field to be absent when any of the others exists.
func MissingWith(others ...string) Rule {
	return conditional("missing_with", func(f Field) bool { return anyPresent(f, others) }, isMissing, namesParams(others))
}

// MissingWithAll requires the field to be absent when all of the others
// exist.
func MissingWithAll(others ...string) Rule {
	return conditional("missing_with_all", func(f Field) bool { return allPresent(f, others) }, isMissing, namesParams(others))
}

// --- prohibited ------------------------------------------------------------

// Prohibited fails unless the field is missing or empty.
func Prohibited() Rule { return implicitCheck("prohibited", nil, blank) }

// ProhibitedIf prohibits the field when another field equals one of values.
func ProhibitedIf(other string, values ...string) Rule {
	return conditional("prohibited_if", func(f Field) bool { _, ok := otherIs(f, other, values); return ok }, blank, otherValueParams(other))
}

// ProhibitedIfFunc prohibits the field when cond returns true.
func ProhibitedIfFunc(cond func(ctx context.Context, f Field) bool) Rule {
	return &funcRule{name: "prohibited", implicit: true, fn: func(ctx context.Context, f Field) error {
		if cond(ctx, f) && !f.Blank() {
			return Fail("prohibited", nil)
		}
		return nil
	}}
}

// ProhibitedUnless prohibits the field unless another field equals one of
// values.
func ProhibitedUnless(other string, values ...string) Rule {
	return conditional("prohibited_unless", func(f Field) bool { _, ok := otherIs(f, other, values); return !ok }, blank, otherValuesParams(other, values))
}

// ProhibitedIfAccepted prohibits the field when another field is accepted.
func ProhibitedIfAccepted(other string) Rule {
	return conditional("prohibited_if_accepted", func(f Field) bool { v, _ := f.Lookup(other); return isAccepted(v) }, blank,
		func(Field) map[string]any { return map[string]any{"other": other} })
}

// ProhibitedIfDeclined prohibits the field when another field is declined.
func ProhibitedIfDeclined(other string) Rule {
	return conditional("prohibited_if_declined", func(f Field) bool { v, _ := f.Lookup(other); return isDeclined(v) }, blank,
		func(Field) map[string]any { return map[string]any{"other": other} })
}

// Prohibits fails when this field is filled and any of the others is too.
func Prohibits(others ...string) Rule {
	return conditional("prohibits", notBlank, func(f Field) bool { return !anyFilled(f, others) },
		func(Field) map[string]any { return map[string]any{"other": attrNames(others)} })
}

// --- accepted / declined ---------------------------------------------------

// Accepted fails unless the value is "yes", "on", 1, "1", true or "true".
func Accepted() Rule {
	return implicitCheck("accepted", nil, func(f Field) bool { return f.Present && isAccepted(f.Value) })
}

// AcceptedIf requires acceptance when another field equals one of values.
func AcceptedIf(other string, values ...string) Rule {
	return conditional("accepted_if", func(f Field) bool { _, ok := otherIs(f, other, values); return ok },
		func(f Field) bool { return f.Present && isAccepted(f.Value) }, otherValueParams(other))
}

// Declined fails unless the value is "no", "off", 0, "0", false or "false".
func Declined() Rule {
	return implicitCheck("declined", nil, func(f Field) bool { return f.Present && isDeclined(f.Value) })
}

// DeclinedIf requires declining when another field equals one of values.
func DeclinedIf(other string, values ...string) Rule {
	return conditional("declined_if", func(f Field) bool { _, ok := otherIs(f, other, values); return ok },
		func(f Field) bool { return f.Present && isDeclined(f.Value) }, otherValueParams(other))
}

// --- exclude ---------------------------------------------------------------

// excluder is implemented by the exclude rules. The engine asks every
// excluder first, and skips the field entirely when one says so.
type excluder interface {
	excluded(ctx context.Context, f Field) (bool, error)
}

type excludeRule struct {
	name string
	cond func(ctx context.Context, f Field) bool
}

func (e excludeRule) Name() string                        { return e.name }
func (excludeRule) Validate(context.Context, Field) error { return nil }
func (e excludeRule) excluded(ctx context.Context, f Field) (bool, error) {
	return e.cond(ctx, f), nil
}

// Exclude always skips the field.
func Exclude() Rule {
	return excludeRule{"exclude", func(context.Context, Field) bool { return true }}
}

// ExcludeIf skips the field when another field equals one of values.
func ExcludeIf(other string, values ...string) Rule {
	return excludeRule{"exclude_if", func(_ context.Context, f Field) bool { _, ok := otherIs(f, other, values); return ok }}
}

// ExcludeIfFunc skips the field when cond returns true.
func ExcludeIfFunc(cond func(ctx context.Context, f Field) bool) Rule {
	return excludeRule{"exclude_if", cond}
}

// ExcludeUnless skips the field unless another field equals one of values.
func ExcludeUnless(other string, values ...string) Rule {
	return excludeRule{"exclude_unless", func(_ context.Context, f Field) bool { _, ok := otherIs(f, other, values); return !ok }}
}

// ExcludeWith skips the field when another field is present.
func ExcludeWith(other string) Rule {
	return excludeRule{"exclude_with", func(_ context.Context, f Field) bool { return otherPresent(f, other) }}
}

// ExcludeWithout skips the field when another field is missing.
func ExcludeWithout(other string) Rule {
	return excludeRule{"exclude_without", func(_ context.Context, f Field) bool { return !otherPresent(f, other) }}
}
