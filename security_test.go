package validator_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	validator "github.com/getrapid-app/go-validator"
)

// Regression tests for issues found by the security lab (docs/SECURITY_LAB.md).

// strconv.ParseFloat accepts "NaN", "Inf", hex floats and digit separators.
// They must not count as numbers, or "NaN" passes between:1,100.
func TestNumericRejectsNonDecimalText(t *testing.T) {
	for _, s := range []string{"NaN", "nan", "Inf", "-Infinity", "0x1p4", "0x10", "1_000", "1e400", "e5", ".", "-", "1e", "1.2.3"} {
		for _, rule := range []string{"numeric", "numeric|between:1,100", "decimal:0,2", "integer"} {
			if validator.Validate(ctx, map[string]any{"n": s}, validator.Rules{"n": rule}) == nil {
				t.Errorf("%q passed %s", s, rule)
			}
		}
	}
	for _, s := range []string{"5", " 5 ", "+5", "-5", ".5", "5.", "1e3", "1E-3", "-1.5e+2"} {
		if err := validator.Validate(ctx, map[string]any{"n": s}, validator.Rules{"n": "numeric"}); err != nil {
			t.Errorf("%q failed numeric: %v", s, err)
		}
	}
}

func TestNumericRejectsNaNAndInf(t *testing.T) {
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, rule := range []string{"numeric", "numeric|between:1,100", "numeric|max:100"} {
			if validator.Validate(ctx, map[string]any{"n": f}, validator.Rules{"n": rule}) == nil {
				t.Errorf("%v passed %s", f, rule)
			}
		}
	}
}

// distinct used to compare every item with every other one, so a request
// with 20,000 items took minutes.
func TestDistinctIsLinear(t *testing.T) {
	n := 20_000
	if testing.Short() || validator.RaceEnabled {
		n = 2_000
	}
	list := make([]any, n)
	for i := range list {
		list[i] = map[string]any{"id": int64(i), "tags": []any{"a", "b"}}
	}
	list[n-1].(map[string]any)["id"] = int64(7)
	data := map[string]any{"items": list}
	start := time.Now()
	bag := mustErrors(t, validator.Validate(ctx, data, validator.Rules{"items.*.id": "distinct", "items.*.tags.*": "distinct"}))
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("distinct over %d items took %v", n, d)
	}
	want := map[string]bool{"items.7.id": true, fmt.Sprintf("items.%d.id", n-1): true}
	got := bag.Fields()
	if len(got) != 2 || !want[got[0]] || !want[got[1]] {
		t.Fatalf("failed fields = %v, want %v", got, want)
	}
}

// Siblings are grouped by every wildcard but the last, so equal values in
// different groups are not duplicates.
func TestDistinctGroups(t *testing.T) {
	data := map[string]any{"a": []any{
		map[string]any{"b": []any{"x", "y"}},
		map[string]any{"b": []any{"x", "X"}},
		map[string]any{"b": []any{"z", "z"}},
	}}
	bag := mustErrors(t, validator.Validate(ctx, data, validator.Rules{"a.*.b.*": "distinct"}))
	if got := bag.Fields(); len(got) != 2 || got[0] != "a.2.b.0" || got[1] != "a.2.b.1" {
		t.Fatalf("distinct: %v", got)
	}
	bag = mustErrors(t, validator.Validate(ctx, data, validator.Rules{"a.*.b.*": "distinct:ignore_case"}))
	if got := bag.Fields(); len(got) != 4 {
		t.Fatalf("distinct:ignore_case: %v", got)
	}
	lists := map[string]any{"l": []any{[]any{1, 2}, []any{1, 2}, []any{2, 1}}}
	bag = mustErrors(t, validator.Validate(ctx, lists, validator.Rules{"l.*": "distinct:strict"}))
	if got := bag.Fields(); len(got) != 2 || got[0] != "l.0" || got[1] != "l.1" {
		t.Fatalf("distinct:strict on lists: %v", got)
	}
}
