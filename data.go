package validator

import (
	"encoding"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The data layer turns any input (structs, maps, slices, pointers) into one
// tree of map[string]any, []any and leaf values. Every rule and every path
// lookup works on that tree, so structs and maps behave identically.

var (
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	timeType          = reflect.TypeOf(time.Time{})
)

func normalize(v any) any { return normalizeValue(reflect.ValueOf(v)) }

func normalizeValue(rv reflect.Value) any {
	for rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil
	}
	if isLeafType(rv.Type()) {
		return rv.Interface()
	}
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return rv.Interface()
		}
		if rv.IsNil() {
			return nil
		}
		m := make(map[string]any, rv.Len())
		it := rv.MapRange()
		for it.Next() {
			m[it.Key().String()] = normalizeValue(it.Value())
		}
		return m
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			return nil
		}
		s := make([]any, rv.Len())
		for i := range s {
			s[i] = normalizeValue(rv.Index(i))
		}
		return s
	case reflect.Struct:
		m := make(map[string]any, rv.NumField())
		normalizeStruct(rv, m)
		return m
	}
	return rv.Interface()
}

func normalizeStruct(rv reflect.Value, m map[string]any) {
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		name, ok := fieldName(sf)
		if !ok {
			continue
		}
		fv := rv.Field(i)
		if sf.Anonymous && name == "" {
			for fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					break
				}
				fv = fv.Elem()
			}
			if fv.Kind() == reflect.Struct {
				normalizeStruct(fv, m)
			}
			continue
		}
		if name == "" {
			name = sf.Name
		}
		m[name] = normalizeValue(fv)
	}
}

// fieldName returns the input name of a struct field, following the json
// tag. It returns "" for embedded structs that should be flattened, and
// ok=false for fields that are unexported or tagged json:"-".
func fieldName(sf reflect.StructField) (name string, ok bool) {
	tag := sf.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	name, _, _ = strings.Cut(tag, ",")
	if sf.Anonymous && name == "" {
		t := sf.Type
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		return "", t.Kind() == reflect.Struct && sf.IsExported()
	}
	if !sf.IsExported() {
		return "", false
	}
	if name == "" {
		name = sf.Name
	}
	return name, true
}

// isLeafType reports whether values of t are kept as-is instead of being
// walked: []byte, time.Time and anything that marshals itself to text.
func isLeafType(t reflect.Type) bool {
	if t == timeType {
		return true
	}
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		return true
	}
	if t.Kind() == reflect.Struct && (t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType)) {
		return true
	}
	return false
}

// lookup returns the value at a concrete dot path.
func lookup(root any, path string) (any, bool) {
	if path == "" {
		return root, true
	}
	cur := root
	for _, seg := range strings.Split(path, ".") {
		switch n := cur.(type) {
		case map[string]any:
			v, ok := n[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(n) {
				return nil, false
			}
			cur = n[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// expand turns a path pattern with "*" segments into the concrete paths that
// exist in the data. Literal segments are kept even when missing, so that
// "items.*.name" still yields "items.0.name" for an item without a name.
func expand(root any, pattern string) []string {
	if !strings.Contains(pattern, "*") {
		return []string{pattern}
	}
	var out []string
	var walk func(cur any, segs []string, prefix string)
	walk = func(cur any, segs []string, prefix string) {
		if len(segs) == 0 {
			out = append(out, prefix)
			return
		}
		seg, rest := segs[0], segs[1:]
		join := func(k string) string {
			if prefix == "" {
				return k
			}
			return prefix + "." + k
		}
		if seg != "*" {
			next, _ := lookup(cur, seg)
			walk(next, rest, join(seg))
			return
		}
		switch n := cur.(type) {
		case []any:
			for i, v := range n {
				walk(v, rest, join(strconv.Itoa(i)))
			}
		case map[string]any:
			keys := make([]string, 0, len(n))
			for k := range n {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(n[k], rest, join(k))
			}
		}
	}
	walk(root, strings.Split(pattern, "."), "")
	return out
}

// resolveRelative replaces the "*" segments of other with the indexes that
// path matched in pattern. For a field "items.2.qty" from "items.*.qty",
// "items.*.type" becomes "items.2.type".
func resolveRelative(pattern, path, other string) string {
	if !strings.Contains(other, "*") || !strings.Contains(pattern, "*") {
		return other
	}
	pSegs, cSegs := strings.Split(pattern, "."), strings.Split(path, ".")
	var stars []string
	for i, s := range pSegs {
		if s == "*" && i < len(cSegs) {
			stars = append(stars, cSegs[i])
		}
	}
	oSegs := strings.Split(other, ".")
	for i, s := range oSegs {
		if s == "*" && len(stars) > 0 {
			oSegs[i], stars = stars[0], stars[1:]
		}
	}
	return strings.Join(oSegs, ".")
}
