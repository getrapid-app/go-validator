package validator

import (
	"context"
	"math/rand"
	"net/mail"
	"reflect"
	"testing"
	"time"
)

// simpleEmail must only accept addresses that net/mail accepts unchanged,
// since validEmail skips net/mail for them.
func checkSimpleEmail(t *testing.T, s string) {
	t.Helper()
	if !simpleEmail(s) {
		return
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		t.Fatalf("simpleEmail(%q) = true, but net/mail gives %v, %v", s, addr, err)
	}
}

func TestSimpleEmailAgreesWithNetMail(t *testing.T) {
	for _, s := range []string{
		"jane@example.com", "a@b", "first.last+tag@sub.example.co", "x!#$%&'*/=?^_`{|}~-@d",
		".a@b", "a.@b", "a..b@c", "a@.b", "a@b.", "a@b..c", "@b", "a@", "a@b@c", "a b@c",
		"\"a\"@b", "a@[1.2.3.4]", "a(c)@b", "Jane <jane@example.com>", "jäne@example.com", "",
	} {
		checkSimpleEmail(t, s)
	}
	if !simpleEmail("jane@example.com") || simpleEmail("a..b@c") || simpleEmail("nope") {
		t.Fatal("simpleEmail misses obvious cases")
	}
	// Random strings over an alphabet heavy in the characters that matter.
	const alphabet = "ab.@@..\"()<>[]:;\\, \t-+_~!éA1"
	r := rand.New(rand.NewSource(1))
	runes := []rune(alphabet)
	for i := 0; i < 200000; i++ {
		b := make([]rune, 1+r.Intn(10))
		for j := range b {
			b[j] = runes[r.Intn(len(runes))]
		}
		checkSimpleEmail(t, string(b))
	}
}

func FuzzSimpleEmail(f *testing.F) {
	for _, s := range []string{"jane@example.com", "a..b@c", "a@b.", "\"q\"@x"} {
		f.Add(s)
	}
	f.Fuzz(checkSimpleEmail)
}

type snapInner struct {
	N    int     `json:"n"`
	F    float64 `json:"f"`
	Name string  `json:"name"`
}

type SnapEmbedded struct {
	Code string `json:"code"`
	Big  int64  `json:"big"`
}

type snapOuter struct {
	SnapEmbedded
	S     string            `json:"s"`
	U     uint32            `json:"u"`
	B     bool              `json:"b"`
	Bytes []byte            `json:"bytes"`
	When  time.Time         `json:"when"`
	In    snapInner         `json:"in"`
	Ptr   *snapInner        `json:"ptr"`
	List  []snapInner       `json:"list"`
	PList []*snapInner      `json:"plist"`
	Any   []any             `json:"any"`
	Map   map[string]string `json:"map"`
	Arr   [2]int            `json:"arr"`
	Named Status            `json:"named"`
}

type Status string

// Normalising through a pointer (addressable, so the struct is copied
// first) must give the same tree as normalising a plain value.
func TestSnapshotNormalizesLikeValue(t *testing.T) {
	in := snapInner{N: 1000, F: 2.5, Name: "x"}
	o := snapOuter{
		SnapEmbedded: SnapEmbedded{Code: "c", Big: 1 << 40},
		S:            "s", U: 7, B: true, Bytes: []byte("hi"),
		When: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		In:   in, Ptr: &in, List: []snapInner{in, {N: 300}}, PList: []*snapInner{&in, nil},
		Any: []any{in, &in, "z"}, Map: map[string]string{"k": "v"}, Arr: [2]int{1, 2}, Named: "on",
	}
	if !infoOf(reflect.TypeOf(o)).snapshot || !infoOf(reflect.TypeOf(in)).snapshot {
		t.Fatal("test types should be snapshotted")
	}
	byPtr, byValue := normalizeValue(reflect.ValueOf(&o)), normalizeValue(reflect.ValueOf(o))
	if !reflect.DeepEqual(byPtr, byValue) {
		t.Fatalf("pointer and value differ:\n%#v\n%#v", byPtr, byValue)
	}
	m := byPtr.(map[string]any)
	if m["code"] != "c" || m["big"] != int64(1<<40) || m["named"] != Status("on") || !reflect.DeepEqual(m["when"], o.When) {
		t.Fatalf("unexpected tree: %#v", m)
	}
	// The tree is a snapshot: changing the struct afterwards does not
	// change values already read.
	o.S, o.List[0].Name = "changed", "changed"
	if m["s"] != "s" || m["list"].([]any)[0].(map[string]any)["name"] != "x" {
		t.Fatal("normalised values changed with the struct")
	}
}

func TestErrorsSlab(t *testing.T) {
	var e Errors
	for _, f := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		e.Add(f, f+"1")
	}
	e.Add("b", "b2")
	got := e.Get("a")
	_ = append(got, "extra") // must not overwrite b's message
	if e.First("b") != "b1" || len(e.Get("b")) != 2 || e.Get("b")[1] != "b2" || e.First("c") != "c1" {
		t.Fatalf("messages mixed up: %v", e.All())
	}
	if e.Len() != 10 || e.Count() != 11 || e.Fields()[9] != "j" {
		t.Fatalf("Len %d Count %d Fields %v", e.Len(), e.Count(), e.Fields())
	}
}

func TestListRulesCacheKeepsOwnCopy(t *testing.T) {
	ctx := context.Background()
	in := map[string]any{"name": "abcd"}
	rules := []string{"required", "max:3"}
	if err := Validate(ctx, in, Rules{"name": rules}); err == nil {
		t.Fatal("max:3 should fail for abcd")
	}
	rules[1] = "max:5" // the caller reuses its slice
	if err := Validate(ctx, in, Rules{"name": rules}); err != nil {
		t.Fatalf("cache returned the rules for the old list: %v", err)
	}
	if err := Validate(ctx, map[string]any{}, Rules{"name": []string{"max:3"}}); err != nil {
		t.Fatalf("a different list with the same pattern shares a spec: %v", err)
	}
}

func TestValidateMapDoesNotAllocate(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector adds allocations")
	}
	in := map[string]any{"email": "jane@example.com", "name": "Jane", "age": 30.0}
	rules := Rules{
		"email": []string{"required", "email", "max:255"},
		"name":  "required|string|between:2,100",
		"age":   []string{"required", "integer", "min:18"},
	}
	ctx := context.Background()
	if err := Validate(ctx, in, rules); err != nil {
		t.Fatal(err)
	}
	if n := testing.AllocsPerRun(100, func() { _ = Validate(ctx, in, rules) }); n != 0 {
		t.Fatalf("Validate allocated %v times per run, want 0", n)
	}
}
