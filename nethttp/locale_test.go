package nethttp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
