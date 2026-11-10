package nethttp_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getrapid-app/go-validator/nethttp"
)

// FuzzInput reads fuzzed request bodies of every supported content type.
func FuzzInput(f *testing.F) {
	f.Add("application/json", `{"a":[1,{"b":2}]}`, "x=1")
	f.Add("application/x-www-form-urlencoded", "a[0][b]=1&a[]=2&a.c=3&%zz", "a[=1")
	f.Add("multipart/form-data; boundary=X", "--X\r\nContent-Disposition: form-data; name=\"a[0]\"\r\n\r\n1\r\n--X--\r\n", "")
	f.Add("application/vnd.api+json", "[1]", "..=1")
	f.Fuzz(func(t *testing.T, contentType, body, query string) {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.URL.RawQuery = query
		r.Header.Set("Content-Type", contentType)
		_, _ = nethttp.Input(r)
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	})
}
