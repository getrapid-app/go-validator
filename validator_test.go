package validator_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/getrapid-app/go-validator"
)

var ctx = context.Background()

func mustErrors(t *testing.T, err error) *validator.Errors {
	t.Helper()
	bag, ok := validator.ErrorsOf(err)
	if !ok {
		t.Fatalf("want a validation error, got %v", err)
	}
	return bag
}

func TestThreeSyntaxesAgree(t *testing.T) {
	type form struct {
		Email string `json:"email" validate:"required|email|max:20"`
	}
	in := form{Email: "not-an-email-and-far-too-long"}
	want := map[string][]string{"email": {
		"The email field must be a valid email address.",
		"The email field must not be greater than 20 characters.",
	}}

	fromString := validator.Validate(ctx, in, validator.Rules{"email": []string{"required", "email", "max:20"}})
	fromChain := validator.Validate(ctx, in, validator.Rules{"email": validator.Chain().Required().Email().Max(20)})
	fromTags := validator.Struct(ctx, in)

	for name, err := range map[string]error{"string": fromString, "chain": fromChain, "tags": fromTags} {
		if got := mustErrors(t, err).All(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestPasses(t *testing.T) {
	err := validator.Validate(ctx, map[string]any{"email": "jane@example.com", "age": 30.0},
		validator.Rules{
			"email": "required|email",
			"age":   "required|integer|between:18,99",
		})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestNestedAndWildcards(t *testing.T) {
	data := map[string]any{
		"address": map[string]any{"city": ""},
		"items": []any{
			map[string]any{"type": "physical", "qty": 0.0},
			map[string]any{"type": "digital"},
			map[string]any{"type": "physical"},
		},
	}
	err := validator.Validate(ctx, data, validator.Rules{
		"address.city": "required",
		"items.*.qty":  "required_if:items.*.type,physical|integer|min:1",
	})
	bag := mustErrors(t, err)
	want := []string{"address.city", "items.0.qty", "items.2.qty"}
	if got := bag.Fields(); !reflect.DeepEqual(got, want) {
		t.Fatalf("fields: got %v, want %v", got, want)
	}
	if got := bag.First("items.2.qty"); got != "The items.2.qty field is required when items.*.type is physical." {
		t.Errorf("message: %q", got)
	}
}

type item struct {
	SKU   string  `json:"sku" validate:"required"`
	Price float64 `json:"price" validate:"numeric|min:0"`
}

type order struct {
	Customer struct {
		Name string `json:"name" validate:"required|string"`
	} `json:"customer"`
	Items []item  `json:"items" validate:"required|array"`
	Note  *string `json:"note" validate:"nullable|string|max:5"`
}

func TestStructTagsNested(t *testing.T) {
	o := order{Items: []item{{SKU: "a", Price: 1}, {Price: -2}}}
	bag := mustErrors(t, validator.Struct(ctx, &o))
	want := map[string][]string{
		// "string" is skipped: like Laravel, non-implicit rules ignore blank values.
		"customer.name": {"The customer.name field is required."},
		"items.1.sku":   {"The items.1.sku field is required."},
		"items.1.price": {"The items.1.price field must be at least 0."},
	}
	if got := bag.All(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestFlags(t *testing.T) {
	rules := validator.Rules{
		"a": "bail|integer|min:10",
		"b": "nullable|integer",
		"c": "sometimes|required",
	}
	bag := mustErrors(t, validator.Validate(ctx, map[string]any{"a": "x", "b": nil}, rules))
	if got := bag.Get("a"); len(got) != 1 {
		t.Errorf("bail: want 1 message, got %v", got)
	}
	if bag.Has("b") || bag.Has("c") {
		t.Errorf("nullable/sometimes should pass: %v", bag.All())
	}
}

func TestCustomRuleDefine(t *testing.T) {
	reg := validator.NewRegistry()
	even := reg.Define("even").
		Message("The :attribute must be even.").
		Check(func(_ context.Context, f validator.Field, _ []string) error {
			n, ok := validator.As[int](f.Value)
			if !ok || n%2 != 0 {
				return validator.Fail("even", nil)
			}
			return nil
		})
	v := validator.New(validator.WithRegistry(reg))
	data := map[string]any{"a": 3.0, "b": 3, "c": 5}
	err := v.Validate(ctx, data, validator.Rules{
		"a": "required|even",
		"b": validator.Chain().Rule("even"),
		"c": validator.List(even()),
	})
	bag := mustErrors(t, err)
	for _, f := range []string{"a", "b", "c"} {
		if got := bag.First(f); got != "The "+f+" must be even." {
			t.Errorf("%s: %q", f, got)
		}
	}
}

func TestRuleForGeneric(t *testing.T) {
	short := validator.RuleFor("short", func(_ context.Context, s string, _ validator.Field) error {
		if len(s) > 3 {
			return errors.New("The :attribute is too long.")
		}
		return nil
	})
	bag := mustErrors(t, validator.Validate(ctx, map[string]any{"code": "abcd", "num": 5},
		validator.Rules{"code": validator.List(short), "num": validator.List(short)}))
	if got := bag.First("code"); got != "The code is too long." {
		t.Errorf("code: %q", got)
	}
	if !bag.Has("num") {
		t.Errorf("num: want a type failure")
	}
}

func TestMessagesAttributesTranslator(t *testing.T) {
	tr := validator.MapTranslator{"ar": {"required": "حقل :attribute مطلوب."}}
	data := map[string]any{}
	rules := validator.Rules{"first_name": "required", "email": "required"}

	bag := mustErrors(t, validator.Validate(ctx, data, rules,
		validator.WithTranslator(tr, "ar"),
		validator.WithMessages(map[string]string{"email.required": "We need your email."}),
		validator.WithAttributes(map[string]string{"first_name": "الاسم"}),
	))
	if got := bag.First("first_name"); got != "حقل الاسم مطلوب." {
		t.Errorf("translated: %q", got)
	}
	if got := bag.First("email"); got != "We need your email." {
		t.Errorf("custom: %q", got)
	}
	bag = mustErrors(t, validator.Validate(ctx, data, rules))
	if got := bag.First("first_name"); got != "The first name field is required." {
		t.Errorf("default: %q", got)
	}
}

func TestAbort(t *testing.T) {
	boom := errors.New("db down")
	r := validator.NewRule("unique_email", func(context.Context, validator.Field) error {
		return validator.Abort(boom)
	})
	err := validator.Validate(ctx, map[string]any{"email": "x"}, validator.Rules{"email": validator.List(r)})
	if !errors.Is(err, boom) {
		t.Fatalf("want abort error, got %v", err)
	}
	if _, ok := validator.ErrorsOf(err); ok {
		t.Fatal("abort must not be a validation error")
	}
}

func TestUnknownRule(t *testing.T) {
	err := validator.Validate(ctx, map[string]any{}, validator.Rules{"a": "required|nope"})
	if err == nil || err.Error() != `validator: unknown rule "nope" (field "a")` {
		t.Fatalf("got %v", err)
	}
}

func TestErrorsJSONOrder(t *testing.T) {
	bag := validator.NewErrors()
	bag.Add("z", "first")
	bag.Add("a", "second")
	b, _ := json.Marshal(bag)
	if string(b) != `{"z":["first"],"a":["second"]}` {
		t.Fatalf("got %s", b)
	}
}

type signUp struct {
	Email    string `json:"email" validate:"required|email"`
	Password string `json:"password" validate:"required|min:8"`
	Confirm  string `json:"password_confirmation"`
	allowed  bool
}

func (s *signUp) Authorize(context.Context) error {
	if !s.allowed {
		return errors.New("sign-ups are closed")
	}
	return nil
}

func (s *signUp) Prepare(context.Context) error {
	if s.Email == " jane@example.com " {
		s.Email = "jane@example.com"
	}
	return nil
}

func (s *signUp) Rules() validator.Rules {
	return validator.Rules{"password_confirmation": validator.Chain().Required().Same("password")}
}

func (s *signUp) Attributes() map[string]string {
	return map[string]string{"password_confirmation": "password confirmation"}
}

func (s *signUp) After(_ context.Context, errs *validator.Errors) error {
	if s.Email == "taken@example.com" {
		errs.Add("email", "That email is already registered.")
	}
	return nil
}

func TestRequestPattern(t *testing.T) {
	err := validator.ValidateRequest(ctx, &signUp{})
	if !errors.Is(err, validator.ErrUnauthorized) {
		t.Fatalf("want unauthorized, got %v", err)
	}

	ok := &signUp{Email: " jane@example.com ", Password: "secret123", Confirm: "secret123", allowed: true}
	if err := validator.ValidateRequest(ctx, ok); err != nil {
		t.Fatalf("want pass, got %v", err)
	}

	bad := &signUp{Email: "taken@example.com", Password: "secret123", Confirm: "other", allowed: true}
	bag := mustErrors(t, validator.ValidateRequest(ctx, bad))
	if got := bag.First("password_confirmation"); got != "The password confirmation field must match password." {
		t.Errorf("same: %q", got)
	}
	if got := bag.First("email"); got != "That email is already registered." {
		t.Errorf("after: %q", got)
	}
}

func TestStructNilFieldsAreMissing(t *testing.T) {
	type form struct {
		Age  *int     `json:"age" validate:"sometimes|integer|min:18"`
		Tags []string `json:"tags" validate:"array|max:2"`
		Bio  *string  `json:"bio" validate:"required"`
	}
	bag := mustErrors(t, validator.Struct(ctx, form{}))
	if got := bag.Fields(); len(got) != 1 || got[0] != "bio" {
		t.Fatalf("got %v", bag.All())
	}
	age := 12
	bag = mustErrors(t, validator.Struct(ctx, form{Age: &age}))
	if !bag.Has("age") {
		t.Fatalf("age 12 should fail: %v", bag.All())
	}
}

func TestBindSeesExtraInputKeys(t *testing.T) {
	type form struct {
		Password string `json:"password" validate:"required|confirmed"`
	}
	_, err := validator.Bind[form](ctx, map[string]any{"password": "secret", "password_confirmation": "secret"})
	if err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestTagCacheSeesLaterRules(t *testing.T) {
	type form struct {
		Code string `json:"code" validate:"required|upper_code"`
	}
	reg := validator.NewRegistry()
	reg.Define("upper_code").Check(func(context.Context, validator.Field, []string) error { return nil })
	v := validator.New(validator.WithRegistry(reg))
	if err := v.Struct(ctx, form{Code: "abc"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	reg.Define("upper_code").Message("Must be upper case.").Check(func(context.Context, validator.Field, []string) error {
		return validator.Fail("upper_code", nil)
	})
	bag := mustErrors(t, v.Struct(ctx, form{Code: "abc"}))
	if bag.First("code") != "Must be upper case." {
		t.Fatalf("got %v", bag.All())
	}
}
