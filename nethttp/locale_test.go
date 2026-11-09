package nethttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getrapid-app/go-validator"
	"github.com/getrapid-app/go-validator/nethttp"
)

func TestPreferredLocale(t *testing.T) {
	cases := []struct {
		header    string
		supported []string
		want      string
	}{
		{"ar-EG,ar;q=0.9,en;q=0.8", nil, "ar"},
		{"ar-EG,ar;q=0.9", []string{"en", "ar", "ar-EG"}, "ar-EG"},
		{"fr;q=0.5, en-US;q=0.8", nil, "en"},
		{"de, *;q=0.1", nil, ""},
		{"", nil, ""},
		{"en;q=0, ar", []string{"en", "ar"}, "ar"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept-Language", c.header)
		if got := nethttp.PreferredLocale(r, c.supported...); got != c.want {
			t.Errorf("%q %v: got %q, want %q", c.header, c.supported, got, c.want)
		}
	}
}

func TestLocalizeMiddleware(t *testing.T) {
	var got *createOrder
	h := nethttp.Localize(handler(&got))
	r := jsonRequest(`{"items":[{"sku":"a","qty":1}]}`)
	r.Header.Set("Accept-Language", "ar")
	code, body := do(t, h, r)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", code)
	}
	errs := body["errors"].(map[string]any)
	if msg := errs["email"].([]any)[0]; msg != "حقل email مطلوب." {
		t.Fatalf("got %v", msg)
	}
}

func TestLocalizedSummaryAndStatusTexts(t *testing.T) {
	var got *createOrder
	h := nethttp.Localize(handler(&got))

	r := jsonRequest(`{"email":"nope","items":[{"sku":"","qty":0}]}`)
	r.Header.Set("Accept-Language", "ar")
	_, body := do(t, h, r)
	if body["message"] != "يجب أن يكون حقل email عنوان بريد إلكتروني صحيحًا. (و 2 أخطاء أخرى)" {
		t.Errorf("422: %v", body["message"])
	}

	r = httptest.NewRequest(http.MethodGet, "/orders?email=blocked@example.com&items[0][sku]=a&items[0][qty]=1", nil)
	r.Header.Set("Accept-Language", "ar-EG,ar;q=0.9")
	code, body := do(t, h, r)
	if code != http.StatusForbidden || body["message"] != "غير مصرح لك بتنفيذ هذا الإجراء." {
		t.Errorf("403: %d %v", code, body["message"])
	}

	// No Accept-Language: English, as before.
	r = jsonRequest(`{"email":"nope","items":[{"sku":"a","qty":1}]}`)
	_, body = do(t, h, r)
	if body["message"] != "The email field must be a valid email address." {
		t.Errorf("en: %v", body["message"])
	}
}

func TestOverrideHTTPTexts(t *testing.T) {
	tr := validator.NewTranslations().Add("en", map[string]string{
		"http.unauthorized": "Nope.",
		"http.more_errors":  ":message (+:count)",
	})
	opt := validator.WithTranslator(tr, "en")
	var got *createOrder
	h := nethttp.Handle(func(w http.ResponseWriter, _ *http.Request, req *createOrder) { got = req }, opt)

	_, body := do(t, h, jsonRequest(`{"email":"nope","items":[{"sku":"","qty":0}]}`))
	if body["message"] != "The email field must be a valid email address. (+2)" {
		t.Errorf("422: %v", body["message"])
	}
	_, body = do(t, h, jsonRequest(`{"email":"blocked@example.com","items":[{"sku":"a","qty":1}]}`))
	if body["message"] != "Nope." {
		t.Errorf("403: %v", body["message"])
	}
	if got != nil {
		t.Error("handler should not run")
	}

	if s := nethttp.SummaryFor(validator.ContextWithLocale(context.Background(), "ar"), validator.NewErrors()); s != "البيانات المدخلة غير صالحة." {
		t.Errorf("empty: %q", s)
	}
}
