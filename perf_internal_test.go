package validator

import (
	"math"
	"reflect"
	"testing"
)

// asReflect is the reflection path of As, used to check the fast paths
// give the same answers.
func asReflect[T any](v any) (T, bool) {
	var zero T
	if t, ok := v.(T); ok {
		return t, true
	}
	if v == nil {
		return zero, false
	}
	target := reflect.TypeFor[T]()
	rv := reflect.ValueOf(v)
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

func TestAsFastPathsMatchReflection(t *testing.T) {
	values := []any{
		0, 1, -1, 42, math.MaxInt64, math.MinInt64,
		int8(-5), int16(300), int32(-70000), int64(1 << 53), int64(1<<53 + 1),
		uint(7), uint8(255), uint16(65535), uint32(1 << 31), uint64(math.MaxUint64), uint64(1 << 63),
		float32(1.5), float32(3), float32(math.Inf(1)), float32(math.NaN()),
		0.0, 2.0, 2.5, -3.0, 1e20, math.Inf(-1), math.NaN(), float64(math.MaxInt64),
	}
	for _, v := range values {
		if a, ok := As[int64](v); true {
			b, ok2 := asReflect[int64](v)
			if ok != ok2 || (ok && a != b) {
				t.Errorf("int64(%T %v): fast %v %v, reflect %v %v", v, v, a, ok, b, ok2)
			}
		}
		if a, ok := As[int](v); true {
			b, ok2 := asReflect[int](v)
			if ok != ok2 || (ok && a != b) {
				t.Errorf("int(%T %v): fast %v %v, reflect %v %v", v, v, a, ok, b, ok2)
			}
		}
		if a, ok := As[float64](v); true {
			b, ok2 := asReflect[float64](v)
			if ok != ok2 || (ok && a != b && !(math.IsNaN(a) && math.IsNaN(b))) {
				t.Errorf("float64(%T %v): fast %v %v, reflect %v %v", v, v, a, ok, b, ok2)
			}
		}
	}
}

func TestLookupIndexes(t *testing.T) {
	root := map[string]any{"a": []any{"x", "y", map[string]any{"b": 1}}}
	cases := map[string]bool{"a.0": true, "a.2.b": true, "a.3": false, "a.-1": false, "a.+1": true, "a.01": true, "a.x": false, "a.": false, "a.2.c": false}
	for p, want := range cases {
		if _, ok := lookup(root, p); ok != want {
			t.Errorf("%s: got %v", p, ok)
		}
	}
}
