package validator

import (
	"encoding"
	"mime/multipart"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The data layer turns any input (structs, maps, slices, pointers) into one
// tree of map[string]any, []any and leaf values. Every rule and every path
// lookup works on that tree, so structs and maps behave identically.

var (
	textMarshalerType      = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	timeType               = reflect.TypeOf(time.Time{})
	multipartHeaderType    = reflect.TypeOf(multipart.FileHeader{})
	multipartHeaderPtrType = reflect.TypeOf(&multipart.FileHeader{})
)

func normalize(v any) any {
	// Input that is already a tree of maps, lists and plain values, such as
	// decoded JSON, is used as is instead of being copied. Rules never
	// change the data, so sharing it is safe.
	if isNormal(v, 0) {
		return v
	}
	return normalizeValue(reflect.ValueOf(v))
}

// isNormal reports whether v is already in the normalised form: nil, a
// built-in scalar, or map[string]any and []any holding only such values.
// Very deep trees are normalised instead, to bound the check.
func isNormal(v any, depth int) bool {
	switch t := v.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	case map[string]any:
		if depth > 32 {
			return false
		}
		for _, x := range t {
			if !isNormal(x, depth+1) {
				return false
			}
		}
		return t != nil
	case []any:
		if depth > 32 {
			return false
		}
		for _, x := range t {
			if !isNormal(x, depth+1) {
				return false
			}
		}
		return t != nil
	}
	return false
}

// typeInfo is what normalisation needs to know about a type, worked out
// once per type with reflection and then cached.
type typeInfo struct {
	file   bool // the value is a File (or *multipart.FileHeader)
	leaf   bool // kept as is instead of walked
	fields []fieldInfo
}

type fieldInfo struct {
	index    int
	name     string
	embedded bool // an embedded struct whose fields are flattened
}

var typeInfos sync.Map // reflect.Type -> *typeInfo

func infoOf(t reflect.Type) *typeInfo {
	if ti, ok := typeInfos.Load(t); ok {
		return ti.(*typeInfo)
	}
	ti := &typeInfo{
		file: t == multipartHeaderPtrType || t.Implements(fileType),
		leaf: isLeafType(t),
	}
	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			sf := t.Field(i)
			name, ok := fieldName(sf)
			if !ok {
				continue
			}
			if sf.Anonymous && name == "" {
				ti.fields = append(ti.fields, fieldInfo{index: i, embedded: true})
				continue
			}
			if name == "" {
				name = sf.Name
			}
			ti.fields = append(ti.fields, fieldInfo{index: i, name: name})
		}
	}
	actual, _ := typeInfos.LoadOrStore(t, ti)
	return actual.(*typeInfo)
}

// fileOf returns rv as a File when its type is one.
func fileOf(rv reflect.Value, ti *typeInfo) (File, bool) {
	if !ti.file || !rv.CanInterface() {
		return nil, false
	}
	return asFile(rv)
}

func normalizeValue(rv reflect.Value) any {
	for rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) {
		if rv.IsNil() {
			return nil
		}
		if rv.Kind() == reflect.Pointer {
			if f, ok := fileOf(rv, infoOf(rv.Type())); ok {
				return f
			}
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil
	}
	// Plain values need no type information.
	switch rv.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		if rv.Type().PkgPath() == "" { // built-in types only; named ones may implement File
			return rv.Interface()
		}
	}
	ti := infoOf(rv.Type())
	if f, ok := fileOf(rv, ti); ok {
		return f
	}
	if ti.leaf {
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
		m := make(map[string]any, len(ti.fields))
		normalizeStruct(rv, ti, m)
		return m
	}
	return rv.Interface()
}

func normalizeStruct(rv reflect.Value, ti *typeInfo, m map[string]any) {
	for _, fi := range ti.fields {
		fv := rv.Field(fi.index)
		if fi.embedded {
			for fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					break
				}
				fv = fv.Elem()
			}
			if fv.Kind() == reflect.Struct {
				normalizeStruct(fv, infoOf(fv.Type()), m)
			}
			continue
		}
		// A nil pointer, slice, map or interface in a struct means the value
		// was not provided, so the field is treated as missing.
		switch fv.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface:
			if fv.IsNil() {
				continue
			}
		}
		m[fi.name] = normalizeValue(fv)
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
	if t == timeType || t == multipartHeaderType || t.Implements(fileType) || reflect.PointerTo(t).Implements(fileType) {
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

// lookup returns the value at a concrete dot path. It walks the path
// segment by segment without splitting it, so it does not allocate.
func lookup(root any, path string) (any, bool) {
	if path == "" {
		return root, true
	}
	cur := root
	for {
		seg, rest, more := strings.Cut(path, ".")
		next, ok := child(cur, seg)
		if !ok {
			return nil, false
		}
		cur = next
		if !more {
			return cur, true
		}
		path = rest
	}
}

// child returns the entry seg of a map or list node.
func child(cur any, seg string) (any, bool) {
	switch n := cur.(type) {
	case map[string]any:
		v, ok := n[seg]
		return v, ok
	case []any:
		i, ok := index(seg, len(n))
		if !ok {
			return nil, false
		}
		return n[i], true
	}
	return nil, false
}

// index parses a list index in [0, n) without allocating. Like
// strconv.Atoi it accepts only plain decimal digits.
func index(seg string, n int) (int, bool) {
	if seg == "" || len(seg) > 9 {
		i, err := strconv.Atoi(seg)
		return i, err == nil && i >= 0 && i < n
	}
	i := 0
	for j := 0; j < len(seg); j++ {
		c := seg[j]
		if c < '0' || c > '9' {
			if j == 0 && (c == '+' || c == '-') && len(seg) > 1 {
				v, err := strconv.Atoi(seg)
				return v, err == nil && v >= 0 && v < n
			}
			return 0, false
		}
		i = i*10 + int(c-'0')
	}
	return i, i < n
}

// match is one concrete path produced by expanding a pattern, with the
// value found there.
type match struct {
	path    string
	value   any
	present bool
}

// expand turns a path pattern with "*" segments into the concrete paths that
// exist in the data. Literal segments are kept even when missing, so that
// "items.*.name" still yields "items.0.name" for an item without a name.
func expand(root any, pattern string) []string {
	ms := expandMatches(root, pattern, splitPath(pattern), nil)
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.path
	}
	return out
}

// expandMatches is expand that also returns each value, appending to buf.
// segs is pattern split on dots.
func expandMatches(root any, pattern string, segs []string, buf []match) []match {
	e := expander{buf: buf}
	e.expand(root, pattern, segs)
	return e.buf
}

// expander walks the data for wildcard patterns. One expander is reused
// for every pattern of a validation, so its buffers are allocated once.
type expander struct {
	buf  []match
	path []byte
}

// expand replaces e.buf with the matches of pattern.
func (e *expander) expand(root any, pattern string, segs []string) {
	e.buf = e.buf[:0]
	if !hasWildcard(segs) {
		v, ok := lookup(root, pattern)
		e.buf = append(e.buf, match{pattern, v, ok})
		return
	}
	e.path = e.path[:0]
	e.walk(root, true, segs)
}

// walk builds the path in one reusable byte buffer and turns it into a
// string once per match.
func (e *expander) walk(cur any, present bool, segs []string) {
	if len(segs) == 0 {
		e.buf = append(e.buf, match{string(e.path), cur, present})
		return
	}
	seg, rest := segs[0], segs[1:]
	mark := len(e.path)
	sep := mark > 0
	if seg != "*" {
		if sep {
			e.path = append(e.path, '.')
		}
		e.path = append(e.path, seg...)
		next, ok := child(cur, seg)
		e.walk(next, present && ok, rest)
		e.path = e.path[:mark]
		return
	}
	switch n := cur.(type) {
	case []any:
		for i, v := range n {
			if sep {
				e.path = append(e.path, '.')
			}
			e.path = strconv.AppendInt(e.path, int64(i), 10)
			e.walk(v, true, rest)
			e.path = e.path[:mark]
		}
	case map[string]any:
		keys := make([]string, 0, len(n))
		for k := range n {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if sep {
				e.path = append(e.path, '.')
			}
			e.path = append(e.path, k...)
			e.walk(n[k], true, rest)
			e.path = e.path[:mark]
		}
	}
}

// listPattern speeds up the most common wildcard pattern, one "*" such as
// "items.*.price". Its concrete paths ("items.0.price", ...) are built once
// and shared by every validation that uses the compiled rules.
type listPattern struct {
	prefix, suffix string // "items" and "price"; either may be empty
	paths          atomic.Pointer[[]string]
}

// maxCachedIndex bounds the paths a listPattern keeps.
const maxCachedIndex = 1 << 12

func newListPattern(pattern string) *listPattern {
	if strings.Count(pattern, "*") != 1 {
		return nil
	}
	segs := splitPath(pattern)
	for i, s := range segs {
		if s == "*" {
			return &listPattern{prefix: strings.Join(segs[:i], "."), suffix: strings.Join(segs[i+1:], ".")}
		}
	}
	return nil // "*" inside a segment, such as "a*b"
}

// path returns the concrete path for index i.
func (l *listPattern) path(i int) string {
	if p := l.paths.Load(); p != nil && i < len(*p) {
		return (*p)[i]
	}
	if i >= maxCachedIndex {
		return l.build(i)
	}
	var old []string
	if p := l.paths.Load(); p != nil {
		old = *p
	}
	n := 2 * len(old)
	if n <= i {
		n = i + 1
	}
	if n < 8 {
		n = 8
	}
	if n > maxCachedIndex {
		n = maxCachedIndex
	}
	paths := make([]string, n)
	copy(paths, old)
	for j := len(old); j < n; j++ {
		paths[j] = l.build(j)
	}
	l.paths.Store(&paths) // concurrent growers build the same strings
	return paths[i]
}

func (l *listPattern) build(i int) string {
	b := make([]byte, 0, len(l.prefix)+len(l.suffix)+8)
	if l.prefix != "" {
		b = append(b, l.prefix...)
		b = append(b, '.')
	}
	b = strconv.AppendInt(b, int64(i), 10)
	if l.suffix != "" {
		b = append(b, '.')
		b = append(b, l.suffix...)
	}
	return string(b)
}

// expandSpec is expand for a compiled spec, using its list fast path when
// the wildcard is over a list.
func (e *expander) expandSpec(root any, spec *fieldSpec) {
	l := spec.list
	if l == nil {
		e.expand(root, spec.pattern, spec.segs)
		return
	}
	base, ok := lookup(root, l.prefix)
	list, isList := base.([]any)
	if !ok || !isList {
		e.expand(root, spec.pattern, spec.segs) // maps, or nothing to expand
		return
	}
	e.buf = e.buf[:0]
	for i, item := range list {
		v, present := item, true
		if l.suffix != "" {
			v, present = lookup(item, l.suffix)
		}
		e.buf = append(e.buf, match{l.path(i), v, present})
	}
}

func splitPath(p string) []string { return strings.Split(p, ".") }

func hasWildcard(segs []string) bool {
	for _, s := range segs {
		if s == "*" {
			return true
		}
	}
	return false
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

// overlay copies into dst every key of src that dst lacks, recursing into
// maps present in both. It lets a request's rules see input keys that have
// no struct field, such as password_confirmation.
func overlay(dst, src any) any {
	d, ok1 := dst.(map[string]any)
	s, ok2 := src.(map[string]any)
	if !ok1 || !ok2 {
		if dst == nil {
			return src
		}
		return dst
	}
	for k, sv := range s {
		if dv, ok := d[k]; ok {
			d[k] = overlay(dv, sv)
		} else {
			d[k] = normalize(sv)
		}
	}
	return d
}
