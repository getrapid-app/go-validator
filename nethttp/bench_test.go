package nethttp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getrapid-app/go-validator/nethttp"
)

func BenchmarkBindJSON(b *testing.B) {
	body := `{"email":"jane@example.com","items":[{"sku":"a","qty":2},{"sku":"b","qty":1}],"rush":true}`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if _, err := nethttp.Bind[createOrder](r); err != nil {
			b.Fatal(err)
		}
	}
}
