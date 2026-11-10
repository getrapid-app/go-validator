package validator

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

// Fuzz targets for input that may come from untrusted clients. Each one
// only asserts that the package never panics and stays within the
// documented contract; run them longer with, for example:
//
//	go test -run '^$' -fuzz '^FuzzRuleValue$' -fuzztime 5m .

// ruleNames returns every built-in rule name, sorted so runs are stable.
func ruleNames() []string {
	r := NewRegistry()
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for n := range r.factories {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// skipRules need resources the fuzzer cannot provide quickly.
var skipRules = map[string]bool{"active_url": true, "exists": true, "unique": true, "current_password": true}

// FuzzRuleValue runs every built-in rule, with fuzzed parameters, over a
// fuzzed value given as a string, a number and a list.
func FuzzRuleValue(f *testing.F) {
	for _, s := range []string{"", "0", "-1", "NaN", "1e400", "a@b.co", "2024-02-30", "\xff\xfe", "1,2", "items.*", "Y-m-d", "/^a$/"} {
		f.Add(s, "1,10")
	}
	names := ruleNames()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	v := New(WithClock(func() time.Time { return time.Unix(1_700_000_000, 0) }))
	f.Fuzz(func(t *testing.T, value, params string) {
		for _, name := range names {
			if skipRules[name] {
				continue
			}
			rule := name
			if params != "" {
				rule += ":" + params
			}
			for _, data := range []any{value, len(value), []any{value, value}, map[string]any{"k": value}} {
				_ = v.Validate(ctx, map[string]any{"f": data, "other": value}, Rules{"f": []string{rule}})
			}
		}
	})
}

// FuzzRuleString parses arbitrary rule strings: compile errors are fine,
// panics are not.
func FuzzRuleString(f *testing.F) {
	for _, s := range []string{"required|max:255", "between:1", "in:", "regex:/(/", "date_format:", "multiple_of:0", "digits_between:9,1", "size:-1", "required_if:a", "exclude_unless:x,", "mimes", "dimensions:min_width=a", "|||", ":"} {
		f.Add(s, "x")
	}
	ctx := context.Background()
	f.Fuzz(func(t *testing.T, rules, value string) {
		if strings.Contains(rules, "active_url") || strings.Contains(rules, "email:") && strings.Contains(rules, "dns") {
			return
		}
		_ = Validate(ctx, map[string]any{"f": value, "items": []any{value}}, Rules{"f": rules, "items.*": rules})
	})
}

// FuzzPattern fuzzes field patterns (wildcards and relative references)
// against JSON input.
func FuzzPattern(f *testing.F) {
	f.Add("items.*.id", `{"items":[{"id":1},{"id":1}]}`, "distinct")
	f.Add("a.*.*", `{"a":[[1,2],[3]]}`, "required_with:a.*.b")
	f.Add("*", `{"x":1}`, "same:*.y")
	f.Add("a..b", `{"a":{"":{"b":1}}}`, "integer")
	ctx := context.Background()
	f.Fuzz(func(t *testing.T, pattern, input, rule string) {
		var data any
		if json.Unmarshal([]byte(input), &data) != nil {
			return
		}
		if strings.Contains(rule, "active_url") || strings.Contains(rule, "dns") {
			return
		}
		_ = Validate(ctx, data, Rules{pattern: rule})
	})
}

// FuzzBind decodes fuzzed JSON into a struct with every kind of field.
func FuzzBind(f *testing.F) {
	f.Add(`{"name":"x","age":3,"tags":["a"],"when":"2024-01-01T00:00:00Z","inner":{"n":1.5},"m":{"a":1}}`)
	f.Add(`{"age":1e400,"tags":"x","inner":[1],"ptr":{"n":"a"}}`)
	type inner struct {
		N float64 `json:"n" validate:"numeric"`
	}
	type req struct {
		Name  string         `json:"name" validate:"required|string|max:20"`
		Age   int8           `json:"age" validate:"integer|between:0,120"`
		U     uint           `json:"u"`
		Tags  []string       `json:"tags" validate:"array|max:3"`
		When  time.Time      `json:"when"`
		Inner inner          `json:"inner"`
		Ptr   *inner         `json:"ptr"`
		M     map[string]int `json:"m"`
		Arr   [2]bool        `json:"arr"`
		Any   any            `json:"any"`
	}
	ctx := context.Background()
	f.Fuzz(func(t *testing.T, input string) {
		var data any
		if json.Unmarshal([]byte(input), &data) != nil {
			return
		}
		_, _ = Bind[req](ctx, data)
	})
}

// FuzzMatchLocale parses arbitrary Accept-Language headers.
func FuzzMatchLocale(f *testing.F) {
	f.Add("ar-EG,ar;q=0.9,en;q=0.8")
	f.Add("*;q=NaN, en;q=-1, ,;;")
	supported := []string{"en", "ar", "ar-EG"}
	f.Fuzz(func(t *testing.T, header string) {
		got := MatchLocale(header, supported...)
		if got != "" && !slices.Contains(supported, got) {
			t.Fatalf("MatchLocale(%q) = %q, not a supported locale", header, got)
		}
	})
}

// FuzzTranslations loads fuzzed JSON catalogs and renders messages with
// fuzzed placeholders.
func FuzzTranslations(f *testing.F) {
	f.Add(`{"required":"The :attribute is :Attribute :ATTRIBUTE"}`, ":attribute:values")
	f.Add(`{"custom":{"f":{"required":":min"}},"attributes":{"f":":attribute"}}`, ":::")
	ctx := context.Background()
	f.Fuzz(func(t *testing.T, catalog, param string) {
		tr := NewTranslations()
		if tr.AddJSON("xx", []byte(catalog)) != nil {
			return
		}
		v := New(WithTranslator(tr, "xx"))
		_ = v.Translate(ctx, "required", map[string]any{"attribute": param, "values": []string{param}})
		_ = v.Validate(ctx, map[string]any{"f": ""}, Rules{"f": "required"})
	})
}
