package validator_test

import (
	"fmt"
	"testing"
	"time"

	validator "github.com/getrapid-app/go-validator"
)

func emptyItems(n int) map[string]any {
	l := make([]any, n)
	for i := range l {
		l[i] = map[string]any{}
	}
	return map[string]any{"items": l}
}

var itemRules = validator.Rules{"items.*.id": "required|integer", "items.*.name": "required|string"}

func TestDefaultMaxErrors(t *testing.T) {
	start := time.Now()
	bag := mustErrors(t, validator.Validate(ctx, emptyItems(100_000), itemRules))
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
	if n := len(bag.Fields()); n != validator.DefaultMaxErrors {
		t.Fatalf("%d failed fields, want %d", n, validator.DefaultMaxErrors)
	}
	// Fields are checked in order, so the first errors are the ones kept.
	if f := bag.Fields(); f[0] != "items.0.id" || f[1] != "items.1.id" {
		t.Fatalf("first fields %v", f[:2])
	}
}

func TestWithMaxErrors(t *testing.T) {
	for _, tc := range []struct {
		opt  validator.Option
		want int
	}{
		{validator.WithMaxErrors(5), 5},
		{validator.WithMaxErrors(-1), 2000},
		{validator.StopOnFirstFailure(), 1},
	} {
		bag := mustErrors(t, validator.Validate(ctx, emptyItems(1000), itemRules, tc.opt))
		if n := len(bag.Fields()); n != tc.want {
			t.Errorf("%d failed fields, want %d", n, tc.want)
		}
	}
}

func TestDefaultMaxErrorsVariable(t *testing.T) {
	old := validator.DefaultMaxErrors
	defer func() { validator.DefaultMaxErrors = old }()
	validator.DefaultMaxErrors = 3
	if n := len(mustErrors(t, validator.Validate(ctx, emptyItems(10), itemRules)).Fields()); n != 3 {
		t.Fatalf("%d failed fields, want 3", n)
	}
	validator.DefaultMaxErrors = 0
	if n := len(mustErrors(t, validator.Validate(ctx, emptyItems(200), itemRules)).Fields()); n != 400 {
		t.Fatalf("%d failed fields, want 400", n)
	}
}

// A field keeps all of its own messages; the limit counts fields.
func TestStopOnFirstFailureKeepsFieldMessages(t *testing.T) {
	err := validator.Validate(ctx, map[string]any{"a": "x", "b": "y"},
		validator.Rules{"a": "integer|min:5", "b": "integer"}, validator.StopOnFirstFailure())
	bag := mustErrors(t, err)
	if f := bag.Fields(); len(f) != 1 || f[0] != "a" || len(bag.Get("a")) != 2 {
		t.Fatalf("fields %v, messages %v", f, bag.Get("a"))
	}
}

type stopRequest struct {
	A string `json:"a" validate:"integer"`
	B string `json:"b" validate:"integer"`
}

func (stopRequest) StopOnFirstFailure() bool { return true }

func TestRequestStopOnFirstFailure(t *testing.T) {
	_, err := validator.Bind[stopRequest](ctx, map[string]any{"a": "x", "b": "y"})
	if f := mustErrors(t, err).Fields(); len(f) != 1 || f[0] != "a" {
		t.Fatalf("fields %v, want [a]", f)
	}
}

// Decode errors from Bind count toward the limit too.
func TestMaxErrorsCapsBindTypeErrors(t *testing.T) {
	type req struct {
		N []int `json:"n"`
	}
	vals := make([]any, 500)
	for i := range vals {
		vals[i] = fmt.Sprint("x", i)
	}
	_, err := validator.Bind[req](ctx, map[string]any{"n": vals}, validator.WithMaxErrors(10))
	if n := len(mustErrors(t, err).Fields()); n != 10 {
		t.Fatalf("%d failed fields, want 10", n)
	}
}
