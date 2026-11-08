package validator

import (
	"context"
	"time"
)

// Field is what a rule sees while it validates one concrete value.
type Field struct {
	// Path is the concrete path of the value, such as "items.0.price".
	Path string
	// Pattern is the path as written in the rules, such as "items.*.price".
	Pattern string
	// Value is the normalised value: a leaf value, map[string]any, []any or
	// a File.
	Value any
	// Present reports whether the path exists in the input at all.
	Present bool

	root  any
	rules []Rule
	v     *Validator
}

// Lookup returns the value of another field in the same input. Wildcards in
// path are filled in from this field's own position, so a rule on
// "items.*.qty" that looks up "items.*.type" reads the same item's type.
func (f Field) Lookup(path string) (any, bool) {
	return lookup(f.root, resolveRelative(f.Pattern, f.Path, path))
}

// HasRule reports whether the field's rule list contains a rule named name.
// Size rules use it to treat numeric strings as numbers, as Laravel does.
func (f Field) HasRule(name string) bool {
	return f.rule(name) != nil
}

func (f Field) rule(name string) Rule {
	for _, r := range f.rules {
		if r.Name() == name {
			return r
		}
	}
	return nil
}

// Blank reports whether the field counts as empty for "required" and the
// other presence rules: missing, nil, a blank string, an empty array or
// map, or the number zero. Zero is not blank when the field also has the
// accept_zero rule.
func (f Field) Blank() bool {
	return !f.Present || isBlankValue(f.Value, f.HasRule("accept_zero"))
}

// Presence returns the verifier configured with WithPresenceVerifier, used
// by the exists and unique rules, or nil.
func (f Field) Presence() PresenceVerifier {
	if f.v == nil {
		return nil
	}
	return f.v.presence
}

// Now returns the validator's current time (see WithClock).
func (f Field) Now() time.Time {
	if f.v == nil || f.v.now == nil {
		return time.Now()
	}
	return f.v.now()
}

// other returns a Field for another path, sharing this field's input and
// validator. Wildcards resolve relative to f.
func (f Field) other(path string) Field {
	p := resolveRelative(f.Pattern, f.Path, path)
	v, ok := lookup(f.root, p)
	return Field{Path: p, Pattern: path, Value: v, Present: ok, root: f.root, v: f.v}
}

// validator returns the Validator running this field. A Field built by
// hand has none, so an empty one is returned.
func (f Field) validator() *Validator {
	if f.v == nil {
		return &Validator{}
	}
	return f.v
}

// runRules runs rules against f the way the engine would, returning the
// first failure. AnyOf uses it.
func (f Field) runRules(ctx context.Context, rules []Rule) error {
	g := f
	g.rules = rules
	for _, r := range rules {
		if _, ok := r.(flagRule); ok {
			continue
		}
		if !isImplicit(r) && (!g.Present || isBlankString(g.Value)) {
			continue
		}
		if err := r.Validate(ctx, g); err != nil {
			return err
		}
	}
	return nil
}
