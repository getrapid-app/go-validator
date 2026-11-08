package nethttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/getrapid-app/go-validator"
	"github.com/getrapid-app/go-validator/nethttp"
)

type item struct {
	SKU string `json:"sku" validate:"required"`
	Qty int    `json:"qty" validate:"required|integer|min:1"`
}

type createOrder struct {
	Email  string    `json:"email" validate:"required|email"`
	Items  []item    `json:"items" validate:"required|array|min:1"`
	Rush   bool      `json:"rush"`
	ShipOn time.Time `json:"ship_on"`
}

func (o *createOrder) Authorize(context.Context) error {
	if o.Email == "blocked@example.com" {
		return errors.New("blocked")
	}
	return nil
}

func do(t *testing.T, h http.Handler, r *http.Request) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	var body map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, body
}

func handler(got **createOrder) http.Handler {
	return nethttp.Handle(func(w http.ResponseWriter, _ *http.Request, req *createOrder) {
		*got = req
		w.WriteHeader(http.StatusNoContent)
	})
}

func jsonRequest(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestBindJSON(t *testing.T) {
	var got *createOrder
	code, _ := do(t, handler(&got), jsonRequest(`{"email":"jane@example.com","items":[{"sku":"a","qty":2}],"rush":true,"ship_on":"2026-11-09T10:00:00Z"}`))
	if code != http.StatusNoContent {
		t.Fatalf("status %d", code)
	}
	want := &createOrder{Email: "jane@example.com", Items: []item{{"a", 2}}, Rush: true, ShipOn: time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestValidationErrorBody(t *testing.T) {
	var got *createOrder
	code, body := do(t, handler(&got), jsonRequest(`{"email":"nope","items":[{"sku":"","qty":0}]}`))
	if code != http.StatusUnprocessableEntity || got != nil {
		t.Fatalf("status %d, handler called: %v", code, got != nil)
	}
	if body["message"] != "The email field must be a valid email address. (and 2 more errors)" {
		t.Errorf("message: %v", body["message"])
	}
	errs := body["errors"].(map[string]any)
	if len(errs) != 3 || errs["items.0.qty"].([]any)[0] != "The items.0.qty field is required." {
		t.Errorf("errors: %v", errs)
	}
}

func TestFormInputWithConversion(t *testing.T) {
	form := url.Values{
		"email":         {"jane@example.com"},
		"items[0][sku]": {"a"},
		"items[0][qty]": {"3"},
		"items[1][sku]": {"b"},
		"items[1][qty]": {"lots"},
		"rush":          {"on"},
	}
	r := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req, err := nethttp.Bind[createOrder](r)
	bag, ok := validator.ErrorsOf(err)
	if !ok {
		t.Fatalf("want validation error, got %v", err)
	}
	if got := bag.All(); !reflect.DeepEqual(got, map[string][]string{"items.1.qty": {"The items.1.qty field must be an integer."}}) {
		t.Fatalf("errors: %v", got)
	}
	if req.Items[0].Qty != 3 || !req.Rush {
		t.Fatalf("decoded: %+v", req)
	}
}

func TestQueryAndUnauthorized(t *testing.T) {
	var got *createOrder
	r := httptest.NewRequest(http.MethodGet, "/orders?email=blocked@example.com&items[0][sku]=a&items[0][qty]=1", nil)
	code, body := do(t, handler(&got), r)
	if code != http.StatusForbidden || body["message"] != "This action is unauthorized." {
		t.Fatalf("got %d %v", code, body)
	}
}

func TestBadJSON(t *testing.T) {
	var got *createOrder
	code, body := do(t, handler(&got), jsonRequest(`{"email":`))
	if code != http.StatusBadRequest || !strings.HasPrefix(body["message"].(string), "invalid request body") {
		t.Fatalf("got %d %v", code, body)
	}
}

type avatarForm struct {
	Name   string                `json:"name" validate:"required"`
	Avatar *multipart.FileHeader `json:"avatar" validate:"required|image|dimensions:max_width=100"`
	Photos []validator.File      `json:"photos" validate:"array|max:2"`
}

func multipartRequest(t *testing.T, fields map[string]string, files map[string][][]byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	for k, list := range files {
		for i, data := range list {
			fw, _ := mw.CreateFormFile(k, k+string(rune('a'+i))+".png")
			_, _ = fw.Write(data)
		}
	}
	_ = mw.Close()
	r := httptest.NewRequest(http.MethodPost, "/avatar", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func pngBytes(w, h int) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)))
	return buf.Bytes()
}

func TestMultipartFiles(t *testing.T) {
	r := multipartRequest(t, map[string]string{"name": "Jane"}, map[string][][]byte{
		"avatar":   {pngBytes(50, 50)},
		"photos[]": {pngBytes(1, 1), pngBytes(2, 2)},
	})
	req, err := nethttp.Bind[avatarForm](r)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if req.Avatar == nil || len(req.Photos) != 2 || req.Photos[1].FileSize() == 0 {
		t.Fatalf("decoded: %+v", req)
	}

	r = multipartRequest(t, map[string]string{"name": "Jane"}, map[string][][]byte{"avatar": {pngBytes(500, 50)}})
	_, err = nethttp.Bind[avatarForm](r)
	if bag, ok := validator.ErrorsOf(err); !ok || bag.First("avatar") != "The avatar field has invalid image dimensions." {
		t.Fatalf("got %v", err)
	}
}

func TestValidateRawInput(t *testing.T) {
	r := jsonRequest(`{"nickname": null}`)
	in, err := nethttp.Validate(r, validator.Rules{
		"nickname": []string{"present", "nullable", "string"},
		"bio":      []string{"sometimes", "required"},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, ok := in["nickname"]; !ok {
		t.Fatalf("input: %v", in)
	}
}

func TestSummary(t *testing.T) {
	bag := validator.NewErrors()
	bag.Add("a", "A is wrong.")
	if got := nethttp.Summary(bag); got != "A is wrong." {
		t.Errorf("one: %q", got)
	}
	bag.Add("a", "A again.")
	if got := nethttp.Summary(bag); got != "A is wrong. (and 1 more error)" {
		t.Errorf("two: %q", got)
	}
}
