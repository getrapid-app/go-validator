package validator

import (
	"errors"
	"reflect"
	"sync"
)

// TagName is the struct tag holding rules, written in the same syntax as
// Parse: `validate:"required|email|max:255"`.
const TagName = "validate"

type tagEntry struct {
	pattern string
	rules   string
}

var tagCache sync.Map // reflect.Type -> []tagEntry

// tagSpecs compiles the `validate` tags of s. Nested structs become dot
// paths, and slices or maps of structs become wildcard paths, so a tag on
// Item.Price inside Items []Item applies to "items.*.price".
func (v *Validator) tagSpecs(s any) ([]fieldSpec, error) {
	t := reflect.TypeOf(s)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil, errors.New("validator: Struct needs a struct or a pointer to one")
	}
	var entries []tagEntry
	if cached, ok := tagCache.Load(t); ok {
		entries = cached.([]tagEntry)
	} else {
		collectTags(t, "", map[reflect.Type]bool{}, &entries)
		tagCache.Store(t, entries)
	}
	key := compiledKey{t: t, reg: v.registry, version: v.registry.currentVersion()}
	if cached, ok := compiledCache.Load(key); ok {
		return cached.([]fieldSpec), nil
	}
	specs := make([]fieldSpec, 0, len(entries))
	for _, e := range entries {
		rules, err := Parse(e.rules).Build(v.registry)
		if err != nil {
			return nil, err
		}
		specs = append(specs, fieldSpec{pattern: e.pattern, rules: rules})
	}
	compiledCache.Store(key, specs)
	return specs, nil
}

// compiledKey identifies a struct type's tag rules built against one
// version of one registry. Rules are immutable once built, so the compiled
// specs are shared between calls and goroutines.
type compiledKey struct {
	t       reflect.Type
	reg     *Registry
	version uint64
}

var compiledCache sync.Map // compiledKey -> []fieldSpec

func collectTags(t reflect.Type, prefix string, seen map[reflect.Type]bool, out *[]tagEntry) {
	if seen[t] {
		return // recursive types: stop rather than loop forever
	}
	seen[t] = true
	defer delete(seen, t)
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		name, ok := fieldName(sf)
		if !ok {
			continue
		}
		ft := sf.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if sf.Anonymous && name == "" {
			collectTags(ft, prefix, seen, out)
			continue
		}
		path := prefix + name
		if tag, ok := sf.Tag.Lookup(TagName); ok && tag != "" && tag != "-" {
			*out = append(*out, tagEntry{pattern: path, rules: tag})
		}
		if sub, wildcard, ok := nestedStruct(ft); ok {
			p := path + "."
			if wildcard {
				p = path + ".*."
			}
			collectTags(sub, p, seen, out)
		}
	}
}

// nestedStruct reports whether t is a struct to descend into, either
// directly or as the element type of a slice, array or map.
func nestedStruct(t reflect.Type) (sub reflect.Type, wildcard, ok bool) {
	if t.Kind() == reflect.Struct && !isLeafType(t) {
		return t, false, true
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		e := t.Elem()
		for e.Kind() == reflect.Pointer {
			e = e.Elem()
		}
		if e.Kind() == reflect.Struct && !isLeafType(e) {
			return e, true, true
		}
	}
	return nil, false, false
}
