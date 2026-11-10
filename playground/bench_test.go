package playground_test

import (
	"context"
	"testing"

	"github.com/getrapid-app/go-validator"
	"github.com/getrapid-app/go-validator/playground"
)

func BenchmarkPlaygroundRules(b *testing.B) {
	reg := validator.NewRegistry()
	playground.Register(reg)
	v := validator.New(validator.WithRegistry(reg))
	in := map[string]any{"phone": "+201001234567", "country": "EG", "code": "123456"}
	rules := validator.Rules{
		"phone":   "required|pg.e164",
		"country": "required|pg.iso3166_1_alpha2",
		"code":    "required|pg.len:6",
	}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if err := v.Validate(ctx, in, rules); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLaravelRules checks the same input with the closest built-in
// rules, for comparison.
func BenchmarkLaravelRules(b *testing.B) {
	in := map[string]any{"phone": "+201001234567", "country": "EG", "code": "123456"}
	rules := validator.Rules{
		"phone":   "required|regex:/^\\+[1-9]\\d{1,14}$/",
		"country": "required|in:EG,US,GB,DE",
		"code":    "required|size:6",
	}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if err := validator.Validate(ctx, in, rules); err != nil {
			b.Fatal(err)
		}
	}
}
