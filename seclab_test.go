//go:build seclab

// The security lab's attack probes. They log time and memory for hostile
// inputs rather than assert, so they only build with the seclab tag:
//
//	go test -tags seclab -run Probe -v . ./nethttp
//
// See docs/SECURITY_LAB.md.

package validator_test

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"

	validator "github.com/getrapid-app/go-validator"
)

func measure(t *testing.T, name string, fn func()) {
	t.Helper()
	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	start := time.Now()
	fn()
	d := time.Since(start)
	runtime.ReadMemStats(&b)
	t.Logf("%-40s %10v  alloc=%6.1f MB", name, d.Round(time.Millisecond), float64(b.TotalAlloc-a.TotalAlloc)/1e6)
}

func items(n int) map[string]any {
	l := make([]any, n)
	for i := range l {
		l[i] = map[string]any{"id": int64(i)}
	}
	return map[string]any{"items": l}
}

func TestProbeDistinct(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{1000, 5000, 20000} {
		data := items(n)
		measure(t, fmt.Sprintf("distinct n=%d", n), func() {
			_ = validator.Validate(ctx, data, validator.Rules{"items.*.id": "distinct"})
		})
	}
}

func TestProbeErrorAmplification(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{10000, 100000} {
		l := make([]any, n)
		for i := range l {
			l[i] = map[string]any{}
		}
		data := map[string]any{"items": l}
		var count int
		measure(t, fmt.Sprintf("required errors n=%d", n), func() {
			err := validator.Validate(ctx, data, validator.Rules{"items.*.id": "required|integer", "items.*.name": "required|string"})
			if bag, ok := validator.ErrorsOf(err); ok {
				count = len(bag.All())
			}
		})
		t.Logf("  error entries: %d", count)
	}
}

func TestProbeMultipleOf(t *testing.T) {
	ctx := context.Background()
	for _, exp := range []string{"1e1000", "1e100000", "1e10000000"} {
		measure(t, "multiple_of:3 value="+exp, func() {
			_ = validator.Validate(ctx, map[string]any{"n": exp}, validator.Rules{"n": "multiple_of:3"})
		})
	}
	measure(t, "multiple_of:0.7 value=1/huge", func() {
		_ = validator.Validate(ctx, map[string]any{"n": "1e-10000000"}, validator.Rules{"n": "multiple_of:0.7"})
	})
}

func TestProbeUint64(t *testing.T) {
	ctx := context.Background()
	for _, v := range []any{uint64(math.MaxUint64), uint(math.MaxUint64), float64(1e300), math.Inf(1), math.NaN()} {
		err := validator.Validate(ctx, map[string]any{"n": v}, validator.Rules{"n": "integer|max:100"})
		t.Logf("integer|max:100 %T(%v) -> err=%v", v, v, err != nil)
		err = validator.Validate(ctx, map[string]any{"n": v}, validator.Rules{"n": "numeric|between:1,100"})
		t.Logf("numeric|between:1,100 %T(%v) -> err=%v", v, v, err != nil)
	}
}

func TestProbeMassAssignment(t *testing.T) {
	type Signup struct {
		Email   string `json:"email" validate:"required|email"`
		IsAdmin bool   `json:"is_admin"`
	}
	req, err := validator.Bind[Signup](context.Background(), map[string]any{"email": "a@b.co", "is_admin": true})
	t.Logf("Bind unvalidated field: is_admin=%v err=%v", req.IsAdmin, err)
}

func TestProbeHugeStrings(t *testing.T) {
	ctx := context.Background()
	s := strings.Repeat("a", 10<<20)
	for _, r := range []string{"email", "url", "active_url", "uuid", "regex:/^(a+)+$/", "alpha_num", "date", "json", "ip", "mac_address", "starts_with:b", "lowercase", "ascii", "ulid", "hex_color", "timezone"} {
		measure(t, "10MB string "+r, func() {
			_ = validator.Validate(ctx, map[string]any{"s": s}, validator.Rules{"s": r})
		})
	}
}

func TestProbeLocale(t *testing.T) {
	h := strings.Repeat("xx-YY;q=0.5,", 1<<20/12)
	measure(t, "MatchLocale 1MB header", func() { _ = validator.MatchLocale(h) })
}

func TestProbeWildcardNesting(t *testing.T) {
	ctx := context.Background()
	n := 100
	outer := make([]any, n)
	for i := range outer {
		mid := make([]any, n)
		for j := range mid {
			inner := make([]any, n)
			for k := range inner {
				inner[k] = "x"
			}
			mid[j] = inner
		}
		outer[i] = mid
	}
	measure(t, "a.*.*.* 1M leaves integer", func() {
		_ = validator.Validate(ctx, map[string]any{"a": outer}, validator.Rules{"a.*.*.*": "integer"})
	})
}

func TestProbeNumericStrings(t *testing.T) {
	ctx := context.Background()
	for _, s := range []string{"NaN", "nan", "Inf", "-Infinity", "0x10", "0x1p4", "1_000", "1e400", " 5 ", "5\n", "+5", ".5", "5."} {
		r := map[string]bool{}
		for _, rule := range []string{"numeric", "numeric|between:1,100", "integer", "integer|max:100", "decimal:0,2", "digits_between:1,3", "gt:0"} {
			r[rule] = validator.Validate(ctx, map[string]any{"n": s}, validator.Rules{"n": rule}) == nil
		}
		t.Logf("%-12q passes: %v", s, r)
	}
}

func TestProbeDateAlloc(t *testing.T) {
	ctx := context.Background()
	s := strings.Repeat("a", 1<<20)
	for _, r := range []string{"date", "date_format:Y-m-d", "after:2020-01-01", "timezone", "date_equals:2020-01-01"} {
		measure(t, "1MB string "+r, func() {
			_ = validator.Validate(ctx, map[string]any{"s": s}, validator.Rules{"s": r})
		})
	}
}
