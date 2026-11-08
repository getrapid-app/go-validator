package validator_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"
	"time"

	"github.com/getrapid-app/go-validator"
)

// ruleCase validates field "f" in data with rules and expects pass or not.
type ruleCase struct {
	name  string
	data  map[string]any
	rules any
	pass  bool
}

func runCases(t *testing.T, cases []ruleCase, opts ...validator.Option) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validator.Validate(context.Background(), c.data, validator.Rules{"f": c.rules}, opts...)
			if _, isValidation := validator.ErrorsOf(err); err != nil && !isValidation {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := err == nil; got != c.pass {
				t.Fatalf("pass = %v, want %v (err: %v)", got, c.pass, err)
			}
		})
	}
}

func f(v any) map[string]any { return map[string]any{"f": v} }

func with(v any, kv ...any) map[string]any {
	m := map[string]any{"f": v}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

func TestPresenceRules(t *testing.T) {
	none := map[string]any{}
	runCases(t, []ruleCase{
		{"required ok", f("x"), "required", true},
		{"required missing", none, "required", false},
		{"required blank", f("  "), "required", false},
		{"required empty list", f([]any{}), "required", false},
		{"required zero fails", f(0), "required", false},
		{"required zero float fails", f(0.0), "required", false},
		{"required accept_zero", f(0), []string{"required", "accept_zero"}, true},
		{"required accept_zero still needs a value", none, []string{"required", "accept_zero"}, false},
		{"required false ok", f(false), "required", true},

		{"required_if triggered", map[string]any{"t": "a"}, "required_if:t,a", false},
		{"required_if not triggered", map[string]any{"t": "b"}, "required_if:t,a", true},
		{"required_if bool", map[string]any{"t": true}, "required_if:t,true", false},
		{"required_unless", map[string]any{"t": "b"}, "required_unless:t,a", false},
		{"required_unless matched", map[string]any{"t": "a"}, "required_unless:t,a", true},
		{"required_with", map[string]any{"a": "x"}, "required_with:a,b", false},
		{"required_with none", none, "required_with:a,b", true},
		{"required_with other zero counts", map[string]any{"a": 0}, "required_with:a", false},
		{"required_with_all partial", map[string]any{"a": "x"}, "required_with_all:a,b", true},
		{"required_with_all all", map[string]any{"a": "x", "b": "y"}, "required_with_all:a,b", false},
		{"required_without", map[string]any{"a": "x"}, "required_without:a,b", false},
		{"required_without all there", map[string]any{"a": "x", "b": "y"}, "required_without:a,b", true},
		{"required_without_all", none, "required_without_all:a,b", false},
		{"required_without_all one there", map[string]any{"a": "x"}, "required_without_all:a,b", true},
		{"required_if_accepted", map[string]any{"t": "yes"}, "required_if_accepted:t", false},
		{"required_if_declined", map[string]any{"t": "off"}, "required_if_declined:t", false},
		{"required_array_keys ok", f(map[string]any{"a": 1, "b": 2}), "required_array_keys:a,b", true},
		{"required_array_keys missing", f(map[string]any{"a": 1}), "required_array_keys:a,b", false},

		{"filled missing ok", none, "filled", true},
		{"filled blank", f(""), "filled", false},
		{"present ok with blank", f(""), "present", true},
		{"present missing", none, "present", false},
		{"present_if", map[string]any{"t": "a"}, "present_if:t,a", false},
		{"present_with", map[string]any{"a": nil}, "present_with:a", false},
		{"present_with_all", map[string]any{"a": 1}, "present_with_all:a,b", true},
		{"missing ok", none, "missing", true},
		{"missing present", f(nil), "missing", false},
		{"missing_if", with("x", "t", "a"), "missing_if:t,a", false},
		{"missing_unless", with("x", "t", "b"), "missing_unless:t,a", false},
		{"missing_with", with("x", "a", 1), "missing_with:a", false},
		{"missing_with_all", with("x", "a", 1), "missing_with_all:a,b", true},

		{"prohibited ok", none, "prohibited", true},
		{"prohibited blank ok", f(""), "prohibited", true},
		{"prohibited filled", f("x"), "prohibited", false},
		{"prohibited_if", with("x", "t", "a"), "prohibited_if:t,a", false},
		{"prohibited_unless", with("x", "t", "b"), "prohibited_unless:t,a", false},
		{"prohibited_if_accepted", with("x", "t", true), "prohibited_if_accepted:t", false},
		{"prohibited_if_declined", with("x", "t", "no"), "prohibited_if_declined:t", false},
		{"prohibits", with("x", "a", "y"), "prohibits:a", false},
		{"prohibits other empty", with("x"), "prohibits:a", true},

		{"accepted yes", f("yes"), "accepted", true},
		{"accepted true", f(true), "accepted", true},
		{"accepted 1", f(1), "accepted", true},
		{"accepted no", f("no"), "accepted", false},
		{"accepted missing", none, "accepted", false},
		{"accepted_if", with("no", "t", "a"), "accepted_if:t,a", false},
		{"declined off", f("off"), "declined", true},
		{"declined false", f(false), "declined", true},
		{"declined_if", with("yes", "t", "a"), "declined_if:t,a", false},

		{"exclude skips", f("x"), "exclude|integer", true},
		{"exclude_if skips", with("x", "t", "a"), "exclude_if:t,a|integer", true},
		{"exclude_if not matched", with("x", "t", "b"), "exclude_if:t,a|integer", false},
		{"exclude_unless", with("x", "t", "b"), "exclude_unless:t,a|integer", true},
		{"exclude_with", with("x", "t", 1), "exclude_with:t|integer", true},
		{"exclude_without", f("x"), "exclude_without:t|integer", true},

		{"sometimes missing", none, "sometimes|required", true},
		{"nullable nil", f(nil), "nullable|integer", true},
		{"no nullable nil", f(nil), "integer", false},
	})
}

type status string

const (
	active   status = "active"
	archived status = "archived"
)

type level int

func (l level) IsValid() bool { return l >= 1 && l <= 3 }

func TestTypeRules(t *testing.T) {
	runCases(t, []ruleCase{
		{"string", f("x"), "string", true},
		{"string number", f(1), "string", false},
		{"integer int", f(3), "integer", true},
		{"integer whole float", f(3.0), "integer", true},
		{"integer fraction", f(3.5), "integer", false},
		{"integer string", f("42"), "integer", true},
		{"integer strict string", f("42"), "integer:strict", false},
		{"numeric string", f("4.2"), "numeric", true},
		{"numeric word", f("four"), "numeric", false},
		{"numeric strict", f("4.2"), "numeric:strict", false},
		{"boolean 0", f(0), "boolean", true},
		{"boolean string", f("true"), "boolean", true},
		{"boolean 2", f(2), "boolean", false},
		{"boolean strict", f("1"), "boolean:strict", false},
		{"array list", f([]any{1}), "array", true},
		{"array keys ok", f(map[string]any{"a": 1}), "array:a,b", true},
		{"array keys extra", f(map[string]any{"c": 1}), "array:a,b", false},
		{"list map", f(map[string]any{"a": 1}), "list", false},
		{"json", f(`{"a":1}`), "json", true},
		{"json bad", f(`{a:1}`), "json", false},
		{"enum ok", f("active"), validator.Enum(active, archived), true},
		{"enum bad", f("deleted"), validator.Enum(active, archived), false},
		{"valid enum", f(2.0), validator.ValidEnum[level](), true},
		{"valid enum bad", f(7), validator.ValidEnum[level](), false},
	})
}

func TestStringRules(t *testing.T) {
	runCases(t, []ruleCase{
		{"alpha", f("abcé"), "alpha", true},
		{"alpha ascii", f("abcé"), "alpha:ascii", false},
		{"alpha digit", f("abc1"), "alpha", false},
		{"alpha_num", f("abc1"), "alpha_num", true},
		{"alpha_dash", f("a-b_c1"), "alpha_dash", true},
		{"alpha_dash space", f("a b"), "alpha_dash", false},
		{"ascii", f("abc!"), "ascii", true},
		{"ascii unicode", f("é"), "ascii", false},
		{"lowercase", f("abc"), "lowercase", true},
		{"lowercase bad", f("aBc"), "lowercase", false},
		{"uppercase", f("ABC"), "uppercase", true},
		{"starts_with", f("hello"), "starts_with:x,he", true},
		{"starts_with bad", f("hello"), "starts_with:x,y", false},
		{"ends_with", f("hello"), "ends_with:lo", true},
		{"doesnt_start_with", f("hello"), "doesnt_start_with:he", false},
		{"doesnt_end_with", f("hello"), "doesnt_end_with:x", true},
		{"regex in slice keeps pipe", f("b"), []string{"regex:/^(a|b)$/"}, true},
		{"regex php flags", f("ABC"), []string{"regex:/^abc$/i"}, true},
		{"regex go syntax", f("abc"), validator.Regex(`^a.c$`), true},
		{"not_regex", f("abc"), []string{"not_regex:/^a/"}, false},
		{"email", f("jane@example.com"), "email", true},
		{"email name", f("Jane <jane@example.com>"), "email", false},
		{"email strict quoted", f(`"j a"@example.com`), "email:strict", false},
		{"email filter unicode", f("jöe@example.com"), "email:filter", false},
		{"email spoof", f("jаne@example.com"), "email:spoof", false}, // Cyrillic а
		{"url", f("https://example.com/a?b=c"), "url", true},
		{"url no scheme", f("example.com"), "url", false},
		{"url protocols", f("ftp://example.com"), "url:http,https", false},
		{"uuid", f("6ba7b810-9dad-11d1-80b4-00c04fd430c8"), "uuid", true},
		{"uuid v4 wrong", f("6ba7b810-9dad-11d1-80b4-00c04fd430c8"), "uuid:4", false},
		{"uuid bad", f("6ba7b810"), "uuid", false},
		{"ulid", f("01ARZ3NDEKTSV4RRFFQ69G5FAV"), "ulid", true},
		{"ulid bad", f("01ARZ3NDEKTSV4RRFFQ69G5FAU!"), "ulid", false},
		{"ip", f("::1"), "ip", true},
		{"ipv4", f("10.0.0.1"), "ipv4", true},
		{"ipv4 v6", f("::1"), "ipv4", false},
		{"ipv6", f("2001:db8::1"), "ipv6", true},
		{"mac", f("00:1A:2B:3C:4D:5E"), "mac_address", true},
		{"hex color", f("#ffcc00"), "hex_color", true},
		{"hex color bad", f("ffcc00"), "hex_color", false},
	})
}

func TestNumberRules(t *testing.T) {
	runCases(t, []ruleCase{
		{"size string", f("abc"), "size:3", true},
		{"size numeric string", f("3"), "numeric|size:3", true},
		{"size list", f([]any{1, 2}), "size:2", true},
		{"min number", f(5), "min:5", true},
		{"min number low", f(4), "min:5", false},
		{"max string", f("abcdef"), "max:5", false},
		{"between", f(7.5), "between:1,10", true},
		{"gt field", with(5, "o", 3), "gt:o", true},
		{"gt field equal", with(3, "o", 3), "gt:o", false},
		{"gt number", f(5), "gt:3", true},
		{"gte string fields", with("abc", "o", "abc"), "gte:o", true},
		{"lt type mismatch", with("abc", "o", 10), "lt:o", false},
		{"lte list", with([]any{1}, "o", []any{1, 2}), "lte:o", true},
		{"digits", f("1234"), "digits:4", true},
		{"digits number", f(1234), "digits:4", true},
		{"digits wrong", f("123"), "digits:4", false},
		{"digits non digit", f("12.4"), "digits:4", false},
		{"digits_between", f("123"), "digits_between:2,4", true},
		{"min_digits", f(12), "min_digits:3", false},
		{"max_digits", f(12), "max_digits:3", true},
		{"decimal exact", f("1.25"), "decimal:2", true},
		{"decimal range", f("1.2"), "decimal:1,3", true},
		{"decimal wrong", f("1.2"), "decimal:2", false},
		{"multiple_of", f(0.3), "multiple_of:0.1", true},
		{"multiple_of bad", f(10), "multiple_of:3", false},
	})
}

func TestDateRules(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	clock := validator.WithClock(func() time.Time { return now })
	runCases(t, []ruleCase{
		{"date", f("2026-11-08"), "date", true},
		{"date time.Time", f(now), "date", true},
		{"date bad", f("not a date"), "date", false},
		{"date_format php", f("08/11/2026"), "date_format:d/m/Y", true},
		{"date_format go", f("2026-11-08"), "date_format:2006-01-02", true},
		{"date_format wrong", f("2026-11-8"), "date_format:Y-m-d", false},
		{"after tomorrow", f("2026-11-10"), "after:tomorrow", true},
		{"after today fails", f("2026-11-07"), "after:today", false},
		{"after date", f("2026-01-02"), "after:2026-01-01", true},
		{"after field", with("2026-01-02", "start", "2026-01-01"), "after:start", true},
		{"after field fails", with("2026-01-01", "start", "2026-01-02"), "after:start", false},
		{"after_or_equal", f("2026-01-01"), "after_or_equal:2026-01-01", true},
		{"before", f("2025-12-31"), "before:2026-01-01", true},
		{"before_or_equal", f("2026-01-02"), "before_or_equal:2026-01-01", false},
		{"date_equals", f("2026-11-08"), "date_equals:today", true},
		{"after with date_format", f("09/11/2026"), "date_format:d/m/Y|after:today", true},
		{"timezone", f("Africa/Cairo"), "timezone", true},
		{"timezone bad", f("Mars/Base"), "timezone", false},
	}, clock)
}

func TestCompareRules(t *testing.T) {
	runCases(t, []ruleCase{
		{"same", with("a", "o", "a"), "same:o", true},
		{"different", with("a", "o", "a"), "different:o", false},
		{"confirmed", with("pw", "f_confirmation", "pw"), "confirmed", true},
		{"confirmed missing", f("pw"), "confirmed", false},
		{"confirmed custom", with("pw", "repeat", "pw"), "confirmed:repeat", true},
		{"in", f("b"), "in:a,b", true},
		{"in number", f(2.0), "in:1,2", true},
		{"in list", f([]any{"a", "b"}), "in:a,b", true},
		{"in list bad", f([]any{"a", "c"}), "in:a,b", false},
		{"not_in", f("a"), "not_in:a,b", false},
		{"in_array", with("x", "allowed", []any{"x", "y"}), "in_array:allowed.*", true},
		{"in_array missing", with("z", "allowed", []any{"x", "y"}), "in_array:allowed.*", false},
		{"in_array_keys", f(map[string]any{"a": 1}), "in_array_keys:a,b", true},
		{"contains", f([]any{"a", "b", "c"}), "contains:a,c", true},
		{"contains missing", f([]any{"a"}), "contains:a,c", false},
		{"doesnt_contain", f([]any{"a"}), "doesnt_contain:b", true},
		{"any_of first", f("jane@example.com"), validator.AnyOf([]string{"email"}, []string{"integer"}), true},
		{"any_of second", f(5), validator.AnyOf([]string{"email"}, []string{"integer"}), true},
		{"any_of none", f("nope"), validator.AnyOf([]string{"email"}, []string{"integer"}), false},
		{"when true", f("x"), validator.When(true, []string{"integer"}), false},
		{"when false", f("x"), validator.When(false, []string{"integer"}, "string"), true},
	})
}

func TestDistinct(t *testing.T) {
	data := map[string]any{"items": []any{
		map[string]any{"id": 1}, map[string]any{"id": 2}, map[string]any{"id": "1"},
	}}
	bag := mustErrors(t, validator.Validate(ctx, data, validator.Rules{"items.*.id": "distinct"}))
	if got := bag.Fields(); len(got) != 2 || got[0] != "items.0.id" || got[1] != "items.2.id" {
		t.Fatalf("loose: %v", got)
	}
	if err := validator.Validate(ctx, data, validator.Rules{"items.*.id": "distinct:strict"}); err != nil {
		t.Fatalf("strict: %v", err)
	}
}

// memFile is an in-memory File.
type memFile struct {
	name string
	data []byte
}

func (m memFile) FileName() string             { return m.name }
func (m memFile) FileSize() int64              { return int64(len(m.data)) }
func (m memFile) Open() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(m.data)), nil }

func pngFile(t *testing.T, w, h int) memFile {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return memFile{name: "a.png", data: buf.Bytes()}
}

func TestFileRules(t *testing.T) {
	pic := pngFile(t, 300, 200)
	pdf := memFile{name: "doc.png", data: []byte("%PDF-1.4 fake")}
	big := memFile{name: "big.txt", data: bytes.Repeat([]byte("a"), 3*1024)}
	gif, _ := base64.StdEncoding.DecodeString("R0lGODlhAQABAAAAACwAAAAAAQABAAAC")
	runCases(t, []ruleCase{
		{"file", f(pic), "file", true},
		{"file string", f("a.png"), "file", false},
		{"image", f(pic), "image", true},
		{"image gif", f(memFile{"x.gif", gif}), "image", true},
		{"image pdf named png", f(pdf), "image", false},
		{"mimes", f(pic), "mimes:jpg,png", true},
		{"mimes pdf", f(pdf), "mimes:pdf", true},
		{"mimes wrong", f(pdf), "mimes:png", false},
		{"mimetypes wildcard", f(pic), "mimetypes:image/*", true},
		{"extensions", f(pdf), "extensions:png", true},
		{"max kilobytes", f(big), "max:2", false},
		{"min kilobytes", f(big), "min:2", true},
		{"dimensions", f(pic), "dimensions:min_width=200,max_height=200", true},
		{"dimensions ratio", f(pic), "dimensions:ratio=3/2", true},
		{"dimensions too small", f(pic), "dimensions:min_width=400", false},
		{"dimensions fluent", f(pic), validator.Dimensions().Width(300).Height(200), true},
		{"required empty file", f(memFile{}), "required", false},
	})
}

type fakeDB map[string][]any // "table.column" -> values

func (db fakeDB) Count(_ context.Context, q validator.PresenceQuery) (int64, error) {
	var n int64
	for _, v := range db[q.Table+"."+q.Column] {
		if v == q.Value && (q.Ignore == nil || v != q.Ignore) {
			n++
		}
	}
	return n, nil
}

func TestDatabaseRules(t *testing.T) {
	db := validator.WithPresenceVerifier(fakeDB{"users.email": {"taken@example.com"}, "roles.id": {1.0, 2.0}})
	runCases(t, []ruleCase{
		{"unique ok", f("new@example.com"), "unique:users,email", true},
		{"unique taken", f("taken@example.com"), "unique:users,email", false},
		{"unique ignore self", f("taken@example.com"), validator.Unique("users").Column("email").Ignore("taken@example.com"), true},
		{"exists", f(2.0), "exists:roles,id", true},
		{"exists missing", f(3.0), "exists:roles,id", false},
		{"exists list", f([]any{1.0, 3.0}), "exists:roles,id", false},
	}, db)

	err := validator.Validate(ctx, f("x"), validator.Rules{"f": "unique:users"})
	if !errors.Is(err, validator.ErrNoPresenceVerifier) {
		t.Fatalf("want ErrNoPresenceVerifier, got %v", err)
	}
}

func TestPasswordRules(t *testing.T) {
	pw := validator.WithCurrentPassword(validator.CurrentPasswordFunc(func(_ context.Context, p string) (bool, error) {
		return p == "old-secret", nil
	}))
	strong := validator.Password().Min(10).MixedCase().Numbers().Symbols()
	runCases(t, []ruleCase{
		{"current ok", f("old-secret"), "current_password", true},
		{"current wrong", f("guess"), "current_password", false},
		{"strong", f("Abcdefgh1!"), strong, true},
		{"too short", f("Ab1!"), strong, false},
		{"no symbol", f("Abcdefgh12"), strong, false},
		{"no upper", f("abcdefgh1!"), strong, false},
	}, pw)

	bag := mustErrors(t, validator.Validate(ctx, f("abcdefgh12"), validator.Rules{"f": validator.Password().MixedCase()}))
	if got := bag.First("f"); got != "The f field must contain at least one uppercase and one lowercase letter." {
		t.Errorf("message: %q", got)
	}
}

func TestMessagesForNewRules(t *testing.T) {
	data := map[string]any{"password": "secret", "first_name": "x", "age": 3}
	bag := mustErrors(t, validator.Validate(ctx, data, validator.Rules{
		"password":   []string{"required", "confirmed"},
		"last_name":  []string{"required_with:first_name"},
		"age":        []string{"integer", "gt:5"},
		"started_at": []string{"required_without_all:first_name,age"},
	}, validator.WithAttributes(map[string]string{"first_name": "first name"})))
	want := map[string]string{
		"password":  "The password field confirmation does not match.",
		"last_name": "The last name field is required when first name is present.",
		"age":       "The age field must be greater than 5.",
	}
	for k, w := range want {
		if got := bag.First(k); got != w {
			t.Errorf("%s: got %q, want %q", k, got, w)
		}
	}
	if bag.Has("started_at") {
		t.Errorf("started_at should pass")
	}
}

func TestRulesValueForms(t *testing.T) {
	in := map[string]any{"a": "", "b": "", "c": "", "d": "", "e": "", "f": ""}
	bag := mustErrors(t, validator.Validate(ctx, in, validator.Rules{
		"a": []string{"required", "email"},
		"b": "required|email",
		"c": validator.Required(),
		"d": []validator.Rule{validator.Required()},
		"e": []any{"required", validator.Email()},
		"f": validator.Chain().Required(),
	}))
	if bag.Len() != 6 {
		t.Fatalf("want 6 failing fields, got %v", bag.All())
	}
	err := validator.Validate(ctx, in, validator.Rules{"a": 42})
	if err == nil || err.Error() != `validator: unsupported rules value of type int (field "a")` {
		t.Fatalf("got %v", err)
	}
}

func TestAcceptZeroStruct(t *testing.T) {
	type form struct {
		Qty   int `json:"qty" validate:"required"`
		Floor int `json:"floor" validate:"required|accept_zero|integer"`
	}
	bag := mustErrors(t, validator.Struct(ctx, form{}))
	if !bag.Has("qty") || bag.Has("floor") {
		t.Fatalf("got %v", bag.All())
	}
}

func TestImplicitFailureStops(t *testing.T) {
	bag := mustErrors(t, validator.Validate(ctx, f(nil), validator.Rules{"f": "required|integer|min:3"}))
	if got := bag.Get("f"); len(got) != 1 {
		t.Fatalf("want only the required message, got %v", got)
	}
}
