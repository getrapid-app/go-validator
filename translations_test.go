package validator_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/getrapid-app/go-validator"
)

func TestArabicBuiltIn(t *testing.T) {
	bag := mustErrors(t, validator.Validate(ctx, map[string]any{"name": "abcdef"},
		validator.Rules{"email": "required", "name": "max:3"}, validator.WithLocale("ar")))
	if got := bag.First("email"); got != "حقل email مطلوب." {
		t.Errorf("required: %q", got)
	}
	if got := bag.First("name"); got != "يجب ألا يزيد طول حقل name عن 3 حرفًا." {
		t.Errorf("max.string: %q", got)
	}
}

func TestAddLanguageAndSlang(t *testing.T) {
	tr := validator.NewTranslations().
		Add("fr", map[string]string{"required": "Le champ :attribute est obligatoire."}).
		Add("ar_EG", map[string]string{"required": "لازم تكتب :attribute."})
	v := validator.New(validator.WithTranslator(tr, "en"))
	rules := validator.Rules{"email": "required", "age": "integer"}
	data := map[string]any{"age": "x"}

	cases := []struct{ locale, email, age string }{
		{"fr", "Le champ email est obligatoire.", "The age field must be an integer."}, // falls back to English
		{"ar-EG", "لازم تكتب email.", "يجب أن يكون حقل age عددًا صحيحًا."},             // slang, then Arabic
		{"AR-eg", "لازم تكتب email.", "يجب أن يكون حقل age عددًا صحيحًا."},
		{"ar-SA", "حقل email مطلوب.", "يجب أن يكون حقل age عددًا صحيحًا."}, // dialect not added: Arabic
		{"de", "The email field is required.", "The age field must be an integer."},
	}
	for _, c := range cases {
		bag := mustErrors(t, v.Validate(ctx, data, rules, validator.WithLocale(c.locale)))
		if got := bag.First("email"); got != c.email {
			t.Errorf("%s email: %q", c.locale, got)
		}
		if got := bag.First("age"); got != c.age {
			t.Errorf("%s age: %q", c.locale, got)
		}
	}
	if got := tr.Locales(); len(got) != 4 || got[1] != "ar-eg" {
		t.Errorf("locales: %v", got)
	}
	if !tr.Has("fr-CA") || tr.Has("de") {
		t.Errorf("Has")
	}
}

func TestOverrideEnglishAndFallback(t *testing.T) {
	tr := validator.NewTranslations().Add("en", map[string]string{"required": "Yo, :attribute is needed."})
	bag := mustErrors(t, validator.Validate(ctx, map[string]any{}, validator.Rules{"email": "required"},
		validator.WithTranslator(tr, "en")))
	if got := bag.First("email"); got != "Yo, email is needed." {
		t.Errorf("override: %q", got)
	}
	tr.SetFallback("ar")
	bag = mustErrors(t, validator.Validate(ctx, map[string]any{}, validator.Rules{"email": "required"},
		validator.WithTranslator(tr, "de")))
	if got := bag.First("email"); got != "حقل email مطلوب." {
		t.Errorf("fallback ar: %q", got)
	}
}

func TestTranslationsJSONFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"lang/ar-EG.json": {Data: []byte(`{
			"required": "لازم :attribute.",
			"max": {"string": ":attribute طويل أوي، أقصى حاجة :max."},
			"attributes": {"email": "الإيميل", "items.*.name": "اسم المنتج"},
			"custom": {"email": {"email": "الإيميل ده مش مظبوط."}}
		}`)},
		"lang/notes.txt": {Data: []byte("ignored")},
	}
	tr := validator.NewTranslations()
	if err := tr.LoadFS(fsys, "lang/*.json"); err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"email": "nope", "name": "abcdef", "items": []any{map[string]any{}}}
	bag := mustErrors(t, validator.Validate(ctx, data, validator.Rules{
		"email":        "required|email",
		"name":         "max:3",
		"items.*.name": "required",
		"phone":        "required|string",
	}, validator.WithTranslator(tr, "ar-EG")))
	want := map[string]string{
		"email":        "الإيميل ده مش مظبوط.",
		"name":         "name طويل أوي، أقصى حاجة 3.",
		"items.0.name": "لازم اسم المنتج.",
		"phone":        "لازم phone.",
	}
	for f, w := range want {
		if got := bag.First(f); got != w {
			t.Errorf("%s: %q, want %q", f, got, w)
		}
	}

	if err := tr.AddJSON("x", []byte(`{"required": 1}`)); err == nil {
		t.Error("want error for a non-string message")
	}
}

func TestContextLocaleAndCustomRule(t *testing.T) {
	reg := validator.NewRegistry()
	reg.Define("even").Message("The :attribute must be even.").Check(func(_ context.Context, f validator.Field, _ []string) error {
		if n, ok := validator.As[int](f.Value); ok && n%2 == 0 {
			return nil
		}
		return validator.Fail("even", nil)
	})
	tr := validator.NewTranslations().Add("ar", map[string]string{"even": "يجب أن يكون :attribute عددًا زوجيًا."})
	v := validator.New(validator.WithRegistry(reg), validator.WithTranslator(tr, "en"))
	data := map[string]any{"n": 3}
	rules := validator.Rules{"n": "even"}

	bag := mustErrors(t, v.Validate(validator.ContextWithLocale(ctx, "ar"), data, rules))
	if got := bag.First("n"); got != "يجب أن يكون n عددًا زوجيًا." {
		t.Errorf("ar: %q", got)
	}
	bag = mustErrors(t, v.Validate(ctx, data, rules))
	if got := bag.First("n"); got != "The n must be even." {
		t.Errorf("en: %q", got)
	}
	// Code-level messages still win over translations.
	bag = mustErrors(t, v.Validate(validator.ContextWithLocale(ctx, "ar"), data, rules,
		validator.WithMessages(map[string]string{"n.even": "custom"})))
	if got := bag.First("n"); got != "custom" {
		t.Errorf("custom: %q", got)
	}
}

func TestRegistrySetMessageStillApplies(t *testing.T) {
	reg := validator.NewRegistry()
	reg.SetMessage("required", "Need :attribute.")
	bag := mustErrors(t, validator.Validate(ctx, map[string]any{}, validator.Rules{"a": "required"}, validator.WithRegistry(reg)))
	if got := bag.First("a"); got != "Need a." {
		t.Errorf("got %q", got)
	}
}

func TestTranslateKey(t *testing.T) {
	got := validator.Translate(validator.ContextWithLocale(ctx, "ar"), "http.more_errors", map[string]any{"message": "x", "count": 3})
	if got != "x (و 3 أخطاء أخرى)" {
		t.Errorf("ar: %q", got)
	}
	if got := validator.Translate(ctx, "http.unauthorized", nil); got != "This action is unauthorized." {
		t.Errorf("en: %q", got)
	}
	if got := validator.Translate(ctx, "no.such.key", nil); got != "no.such.key" {
		t.Errorf("missing: %q", got)
	}
}

func TestMatchLocale(t *testing.T) {
	if got := validator.MatchLocale("fr, ar-EG;q=0.8", "en", "ar"); got != "ar" {
		t.Errorf("got %q", got)
	}
	if got := validator.MatchLocale("de"); got != "" {
		t.Errorf("got %q", got)
	}
}
