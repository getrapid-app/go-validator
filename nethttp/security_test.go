package nethttp_test

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	validator "github.com/getrapid-app/go-validator"
	"github.com/getrapid-app/go-validator/nethttp"
)

// Regression tests for issues found by the security lab (docs/SECURITY_LAB.md).

// Every key segment becomes a nested map, so a 2 MB key used to allocate
// over 400 MB.
func TestInputRejectsDeepKeys(t *testing.T) {
	deep := strings.Repeat("a[b]", nethttp.MaxKeyDepth) + "=1"
	ok := strings.Repeat("a.", nethttp.MaxKeyDepth-1) + "a=1"
	for _, tc := range []struct {
		name, ct, body, query string
		wantErr               bool
	}{
		{"form deep", "application/x-www-form-urlencoded", deep, "", true},
		{"form dots deep", "application/x-www-form-urlencoded", strings.Repeat("a.", 100_000) + "a=1", "", true},
		{"query deep", "", "", deep, true},
		{"form at limit", "application/x-www-form-urlencoded", ok, "", false},
	} {
		r := httptest.NewRequest("POST", "/?"+tc.query, strings.NewReader(tc.body))
		if tc.ct != "" {
			r.Header.Set("Content-Type", tc.ct)
		}
		_, err := nethttp.Input(r)
		var de *nethttp.DecodeError
		if got := errors.As(err, &de); got != tc.wantErr {
			t.Errorf("%s: err = %v, want DecodeError: %v", tc.name, err, tc.wantErr)
		}
		if err != nil && len(err.Error()) > 200 {
			t.Errorf("%s: error repeats the whole key (%d bytes)", tc.name, len(err.Error()))
		}
	}
}

// Multipart bodies used to ignore every size limit, so uploads could fill
// the disk with temporary files.
func TestInputCapsMultipartBodies(t *testing.T) {
	old := nethttp.MaxMultipartBytes
	nethttp.MaxMultipartBytes = 1 << 20
	defer func() { nethttp.MaxMultipartBytes = old }()

	for _, tc := range []struct {
		size    int
		wantErr bool
	}{{512 << 10, false}, {2 << 20, true}} {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("doc", "a.bin")
		fw.Write(bytes.Repeat([]byte{'x'}, tc.size))
		mw.Close()
		r := httptest.NewRequest("POST", "/", &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		_, err := nethttp.Input(r)
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
		var de *nethttp.DecodeError
		if got := errors.As(err, &de); got != tc.wantErr {
			t.Errorf("%d byte upload: err = %v, want DecodeError: %v", tc.size, err, tc.wantErr)
		}
	}
}

// Uploaded files get validator.DefaultMaxFileSize unless their rules set a
// size of their own.
func TestUploadsGetDefaultMaxFileSize(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, name := range []string{"avatar", "doc"} {
		fw, _ := mw.CreateFormFile(name, name+".bin")
		fw.Write(bytes.Repeat([]byte{'x'}, 11<<20))
	}
	mw.Close()
	r := httptest.NewRequest("POST", "/", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	_, err := nethttp.Validate(r, validator.Rules{"avatar": "required|file", "doc": "required|file|max:20480"})
	if r.MultipartForm != nil {
		r.MultipartForm.RemoveAll()
	}
	bag, ok := validator.ErrorsOf(err)
	if !ok {
		t.Fatalf("err = %v, want validation errors", err)
	}
	if f := bag.Fields(); len(f) != 1 || f[0] != "avatar" {
		t.Fatalf("failed fields %v, want [avatar]", f)
	}
	if got, want := bag.First("avatar"), "The avatar field must not be greater than 10240 kilobytes."; got != want {
		t.Fatalf("message %q, want %q", got, want)
	}
}
