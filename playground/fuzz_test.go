package playground_test

import (
	"context"
	"testing"

	validator "github.com/getrapid-app/go-validator"
	"github.com/getrapid-app/go-validator/playground"
)

// FuzzTags runs every go-playground tag, with a fuzzed parameter, over a
// fuzzed value. go-playground panics on some inputs; the adapter must turn
// every panic into a failed rule.
func FuzzTags(f *testing.F) {
	f.Add("+14155552671", "6")
	f.Add("#fff", "a b,'c d'")
	f.Add("\xff", "|")
	f.Add("", "0x2C")
	reg := validator.NewRegistry()
	playground.Register(reg)
	v := validator.New(validator.WithRegistry(reg))
	tags := playground.TagNames()
	ctx := context.Background()
	f.Fuzz(func(t *testing.T, value, param string) {
		for _, tag := range tags {
			rule := playground.Prefix + tag
			if param != "" {
				rule += ":" + param
			}
			for _, data := range []any{value, len(value), []any{value}, map[string]any{"k": value}} {
				_ = v.Validate(ctx, map[string]any{"f": data}, validator.Rules{"f": []string{rule}})
			}
		}
	})
}
