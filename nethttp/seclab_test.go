//go:build seclab

// The security lab's attack probes. They log time and memory for hostile
// inputs rather than assert, so they only build with the seclab tag:
//
//	go test -tags seclab -run Probe -v . ./nethttp
//
// See docs/SECURITY_LAB.md.

package nethttp_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/getrapid-app/go-validator/nethttp"
)

func measure(t *testing.T, name string, fn func()) {
	t.Helper()
	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	start := time.Now()
	fn()
	runtime.ReadMemStats(&b)
	t.Logf("%-44s %10v  alloc=%7.1f MB", name, time.Since(start).Round(time.Millisecond), float64(b.TotalAlloc-a.TotalAlloc)/1e6)
}

func TestProbeFormDepth(t *testing.T) {
	for _, n := range []int{10_000, 100_000, 1_000_000} {
		body := strings.Repeat("a.", n) + "a=1"
		measure(t, fmt.Sprintf("urlencoded key depth=%d (%d KB)", n, len(body)/1024), func() {
			r := httptest.NewRequest("POST", "/", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			_, err := nethttp.Input(r)
			if err != nil {
				t.Log(err)
			}
		})
	}
}

func TestProbeFormWideList(t *testing.T) {
	var b strings.Builder
	for i := range 200_000 {
		fmt.Fprintf(&b, "i[%d]=x&", i)
	}
	body := b.String()
	measure(t, fmt.Sprintf("urlencoded 200k list keys (%d KB)", len(body)/1024), func() {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_, _ = nethttp.Input(r)
	})
}

func TestProbeMultipartLimit(t *testing.T) {
	old := nethttp.MaxMultipartBytes
	nethttp.MaxMultipartBytes = 1 << 20
	defer func() { nethttp.MaxMultipartBytes = old }()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("doc", "big.bin")
	fw.Write(bytes.Repeat([]byte{'x'}, 50<<20))
	mw.Close()

	tmpBefore := countTmp()
	r := httptest.NewRequest("POST", "/", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	in, err := nethttp.Input(r)
	t.Logf("50 MB multipart with MaxMultipartBytes=1MB: err=%v, got file=%v, tmp files +%d", err, in["doc"] != nil, countTmp()-tmpBefore)
	if r.MultipartForm != nil {
		r.MultipartForm.RemoveAll()
	}
}

func countTmp() int {
	m, _ := filepath.Glob(filepath.Join(os.TempDir(), "multipart-*"))
	return len(m)
}

func TestProbeJSONDepth(t *testing.T) {
	body := strings.Repeat(`{"a":`, 100_000) + "1" + strings.Repeat("}", 100_000)
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	_, err := nethttp.Input(r)
	t.Logf("JSON depth 100k: err=%v", err)
}

func TestProbeKeyConflicts(t *testing.T) {
	for _, body := range []string{"a=1&a[b]=2", "a[b]=2&a=1", "a[0]=x&a[b]=y", "a[]=1&a[0]=2", "a[0][b]=1&a[0]=2", "__proto__[x]=1", "a..b=1", ".=1", "[=1", "]=1"} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		in, err := nethttp.Input(r)
		t.Logf("%-22q -> %v err=%v", body, in, err)
	}
}
