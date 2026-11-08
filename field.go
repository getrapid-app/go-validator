package validator

// Field is what a rule sees while it validates one concrete value.
type Field struct {
	// Path is the concrete path of the value, such as "items.0.price".
	Path string
	// Pattern is the path as written in the rules, such as "items.*.price".
	Pattern string
	// Value is the normalised value: a leaf value, map[string]any or []any.
	Value any
	// Present reports whether the path exists in the input at all.
	Present bool

	root  any
	rules []Rule
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
	for _, r := range f.rules {
		if r.Name() == name {
			return true
		}
	}
	return false
}
