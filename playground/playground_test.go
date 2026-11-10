package playground_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getrapid-app/go-validator"
	"github.com/getrapid-app/go-validator/playground"
	pgv "github.com/go-playground/validator/v10"
)

// newValidator returns a validator with its own registry and translations,
// so tests do not depend on the package-level defaults.
func newValidator(t *testing.T, opts ...playground.Option) (*validator.Validator, *validator.Registry, *validator.Translations) {
	t.Helper()
	reg := validator.NewRegistry()
	playground.Register(reg, opts...)
	tr := validator.NewTranslations()
	playground.AddTranslations(tr)
	return validator.New(validator.WithRegistry(reg), validator.WithTranslator(tr, "en")), reg, tr
}

func firstError(t *testing.T, err error, field string) string {
	t.Helper()
	bag, ok := validator.ErrorsOf(err)
	if !ok {
		t.Fatalf("want a validation error, got %v", err)
	}
	return bag.First(field)
}

func TestRules(t *testing.T) {
	v, _, _ := newValidator(t)
	ctx := context.Background()
	cases := []struct {
		rules string
		value any
		ok    bool
	}{
		{"pg.e164", "+201001234567", true},
		{"pg.e164", "01001234567", false},
		{"pg.iso3166_1_alpha2", "EG", true},
		{"pg.iso3166_1_alpha2", "XX", false},
		{"pg.len:6", "123456", true},
		{"pg.len:6", "12345", false},
		{"pg.len:2", []any{"a", "b"}, true},
		{"pg.min:3", "abc", true},
		{"pg.min:3", 2.0, false},
		{"pg.max:2", []any{1.0, 2.0, 3.0}, false},
		{"pg.gt:10", 11.0, true},
		{"pg.gt:10", 10.0, false},
		{"pg.oneof:red,green,blue", "green", true},
		{"pg.oneof:red,green,blue", "pink", false},
		{"pg.oneof:1,2,3", 2.0, true}, // decoded JSON number
		{"pg.oneof:1,2,3", 4.0, false},
		{"pg.oneof:new york,cairo", "new york", true},
		{"pg.oneofci:red,green", "RED", true},
		{"pg.contains:,", "a,b", true}, // a comma parameter
		{"pg.contains:,", "ab", false},
		{"pg.uuid4", "3f1e3a2c-1b9d-4c47-9a57-0d2f6b6d9a10", true},
		{"pg.uuid4", "nope", false},
		{"pg.email", 42.0, false}, // a type the tag does not support
		{"pg.hexcolor", "#fff", true},
		{"pg.semver", "1.2.3", true},
		{"pg.datetime:2006-01-02", "2026-11-10", true},
		{"pg.datetime:2006-01-02", "10/11/2026", false},
		{"pg.unique", []any{"a", "b", "a"}, false},
		{"pg.country_code", "EGY", true},
		{"required|pg.e164", "+201001234567", true},
	}
	for _, c := range cases {
		err := v.Validate(ctx, map[string]any{"f": c.value}, validator.Rules{"f": c.rules})
		if (err == nil) != c.ok {
			t.Errorf("%s on %#v: got %v, want ok=%v", c.rules, c.value, err, c.ok)
		}
	}
}

func TestSkippedWhenMissingOrEmpty(t *testing.T) {
	v, _, _ := newValidator(t)
	ctx := context.Background()
	for _, in := range []map[string]any{{}, {"phone": ""}} {
		if err := v.Validate(ctx, in, validator.Rules{"phone": "pg.e164"}); err != nil {
			t.Errorf("%v: %v", in, err)
		}
	}
	// As in Laravel, null is checked unless the field is nullable.
	if err := v.Validate(ctx, map[string]any{"phone": nil}, validator.Rules{"phone": "nullable|pg.e164"}); err != nil {
		t.Errorf("nullable: %v", err)
	}
	if err := v.Validate(ctx, map[string]any{}, validator.Rules{"phone": "required|pg.e164"}); err == nil {
		t.Error("required should still fail")
	}
}

type signup struct {
	Phone   string   `json:"phone" validate:"required|pg.e164"`
	Country string   `json:"country" validate:"required|pg.iso3166_1_alpha2"`
	Code    string   `json:"code" validate:"required|pg.len:6"`
	Tags    []string `json:"tags" validate:"pg.unique"`
}

func TestThreeSyntaxes(t *testing.T) {
	v, _, _ := newValidator(t)
	ctx := context.Background()

	good := signup{Phone: "+201001234567", Country: "EG", Code: "123456", Tags: []string{"a", "b"}}
	if err := v.Struct(ctx, &good); err != nil {
		t.Fatalf("struct tags: %v", err)
	}
	bad := signup{Phone: "123", Country: "EG", Code: "12", Tags: []string{"a", "a"}}
	bag, _ := validator.ErrorsOf(v.Struct(ctx, &bad))
	if got := strings.Join(bag.Fields(), ","); got != "phone,code,tags" {
		t.Fatalf("struct tags: failed fields %q", got)
	}

	in := map[string]any{"code": "12"}
	strRules := validator.Rules{"code": "required|pg.len:6"}
	listRules := validator.Rules{"code": []string{"required", "pg.len:6"}}
	fluent := validator.Rules{"code": validator.Chain().Required().Rule("pg.len", "6")}
	for name, rules := range map[string]validator.Rules{"string": strRules, "list": listRules, "fluent": fluent} {
		if got := firstError(t, v.Validate(ctx, in, rules), "code"); got != "The code field must be exactly 6 characters." {
			t.Errorf("%s: %q", name, got)
		}
	}
}

func TestMessages(t *testing.T) {
	v, _, tr := newValidator(t)
	ctx := context.Background()
	check := func(v *validator.Validator, rules string, value any, want string) {
		t.Helper()
		got := firstError(t, v.Validate(ctx, map[string]any{"phone": value}, validator.Rules{"phone": rules}), "phone")
		if got != want {
			t.Errorf("%s on %#v: got %q, want %q", rules, value, got, want)
		}
	}
	check(v, "pg.e164", "1", "The phone field must be a phone number in E.164 format.")
	check(v, "pg.min:5", "abc", "The phone field must be at least 5 characters.")
	check(v, "pg.min:5", 3.0, "The phone field must be at least 5.")
	check(v, "pg.min:5", []any{1.0}, "The phone field must have at least 5 items.")
	check(v, "pg.oneof:a,b", "c", "The phone field must be one of: a, b.")
	check(v, "pg.gt", time.Now().Add(-time.Hour), "The phone field must be a date after now.")

	ar := v.With(validator.WithLocale("ar"))
	check(ar, "pg.e164", "1", "يجب أن يكون حقل phone رقم هاتف بصيغة E.164.")
	check(ar, "pg.min:5", "abc", "يجب أن يكون طول حقل phone 5 حرفًا على الأقل.")

	// Overrides work as for any rule: slang for a locale, and in code.
	tr.Add("ar-EG", map[string]string{"pg.e164": "اكتب رقم :attribute صح."})
	check(v.With(validator.WithLocale("ar-EG")), "pg.e164", "1", "اكتب رقم phone صح.")
	check(v.With(validator.WithMessages(map[string]string{"phone.pg.e164": "Use +20..."})), "pg.e164", "1", "Use +20...")
}

func TestEveryLanguageHasEveryMessage(t *testing.T) {
	_, _, tr := newValidator(t)
	for _, key := range playground.MessageKeys() {
		if _, ok := tr.Translate("ar", key); !ok {
			t.Errorf("no Arabic message for %s", key)
		}
	}
}

func TestBadParameters(t *testing.T) {
	_, reg, _ := newValidator(t)
	if _, err := reg.Make("pg.len"); err == nil {
		t.Error("pg.len without a parameter should not compile")
	}
	if _, err := reg.Make("pg.email", "x"); err == nil {
		t.Error("pg.email with a parameter should not compile")
	}
	if _, err := reg.Make("pg.eqfield", "Other"); err == nil {
		t.Error("cross-field tags should not be registered")
	}
}

// Every registered tag must exist in go-playground: validate recovers
// from panics, so a misspelt tag would otherwise fail silently.
func TestEveryTagExistsInPlayground(t *testing.T) {
	pv := pgv.New()
	func() { // the check below relies on this panic message
		defer func() {
			if r := recover(); !strings.Contains(fmt.Sprint(r), "Undefined validation function") {
				t.Fatalf("unexpected panic for an unknown tag: %v", r)
			}
		}()
		_ = pv.Var("x", "no_such_tag")
	}()
	for _, name := range playground.TagNames() {
		tag := name
		if p := sampleParam(name); p != "" {
			tag += "=" + p
		}
		func() {
			defer func() {
				if r := recover(); r != nil && strings.Contains(fmt.Sprint(r), "Undefined validation function") {
					t.Errorf("go-playground has no %q tag", name)
				}
			}()
			_ = pv.Var("x", tag)
		}()
	}
}

func sampleParam(tag string) string {
	switch tag {
	case "datetime":
		return "2006-01-02"
	case "postcode_iso3166_alpha2":
		return "US"
	case "len", "min", "max", "contains", "containsany", "containsrune", "excludes", "excludesall",
		"excludesrune", "startswith", "endswith", "startsnotwith", "endsnotwith", "oneof", "oneofci",
		"noneof", "noneofci", "eq_ignore_case", "ne_ignore_case":
		return "1"
	}
	return ""
}

func TestCustomPlaygroundValidation(t *testing.T) {
	pv := pgv.New()
	if err := pv.RegisterValidation("even_len", func(fl pgv.FieldLevel) bool { return len(fl.Field().String())%2 == 0 }); err != nil {
		t.Fatal(err)
	}
	reg := validator.NewRegistry()
	playground.Register(reg, playground.WithValidate(pv), playground.Tags("even_len"))
	reg.SetMessage("pg.even_len", "The :attribute must have an even length.")
	v := validator.New(validator.WithRegistry(reg))
	ctx := context.Background()
	if err := v.Validate(ctx, map[string]any{"s": "ab"}, validator.Rules{"s": "pg.even_len"}); err != nil {
		t.Fatal(err)
	}
	got := firstError(t, v.Validate(ctx, map[string]any{"s": "abc"}, validator.Rules{"s": "pg.even_len"}), "s")
	if got != "The s must have an even length." {
		t.Fatalf("got %q", got)
	}
}

func TestConcurrentUse(t *testing.T) {
	v, _, _ := newValidator(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 200 {
				phone := "+20100123456" + fmt.Sprint(j%10)
				if i%2 == 1 {
					phone = "bad"
				}
				err := v.Validate(ctx, map[string]any{"phone": phone}, validator.Rules{"phone": "required|pg.e164|pg.max:20"})
				if (err == nil) == (i%2 == 1) {
					t.Errorf("worker %d: %v", i, err)
					return
				}
			}
		})
	}
	wg.Wait()
}
