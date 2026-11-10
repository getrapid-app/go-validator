package nethttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Limits for reading request bodies. Change them at program start.
var (
	// MaxBodyBytes caps JSON and urlencoded bodies.
	MaxBodyBytes int64 = 10 << 20
	// MaxMultipartMemory is how much of a multipart body is kept in memory;
	// the rest of each file is stored in temporary files.
	MaxMultipartMemory int64 = 32 << 20
	// MaxMultipartBytes caps the whole multipart body, files included, so
	// uploads cannot fill the disk with temporary files. Each file is also
	// checked against validator.DefaultMaxFileSize or its field's max rule;
	// raise this cap when a field allows larger files.
	MaxMultipartBytes int64 = 32 << 20
	// MaxKeyDepth caps how many segments a form or query key may have, as in
	// "a[b][c]" (3). Deeper keys are rejected with a DecodeError, since each
	// segment becomes a nested map.
	MaxKeyDepth = 32
)

// DecodeError is returned when the request body cannot be read, such as
// malformed JSON. WriteError answers it with 400 Bad Request.
type DecodeError struct{ Err error }

func (e *DecodeError) Error() string { return "invalid request body: " + e.Err.Error() }
func (e *DecodeError) Unwrap() error { return e.Err }

// Input reads everything a request carries into one map, the way Laravel's
// $request->all() does: query parameters first, then the body, whose values
// win. JSON, urlencoded and multipart bodies are supported. Uploaded files
// appear as *multipart.FileHeader values (or lists of them).
//
// Form and query keys may use brackets or dots for nesting:
// "items[0][name]", "items.0.name" and "tags[]" all work.
func Input(r *http.Request) (map[string]any, error) {
	in := map[string]any{}
	if err := mergeValues(in, r.URL.Query()); err != nil {
		return nil, err
	}

	if r.Body == nil || r.Body == http.NoBody {
		return in, nil
	}
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch {
	case ct == "application/json" || strings.HasSuffix(ct, "+json"):
		body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, MaxBodyBytes))
		if err != nil {
			return nil, &DecodeError{err}
		}
		if len(bytes.TrimSpace(body)) == 0 {
			return in, nil
		}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, &DecodeError{err}
		}
		obj, ok := numbers(v).(map[string]any)
		if !ok {
			return nil, &DecodeError{errors.New("the JSON body must be an object")}
		}
		maps.Copy(in, obj)
	case ct == "application/x-www-form-urlencoded":
		r.Body = http.MaxBytesReader(nil, r.Body, MaxBodyBytes)
		if err := r.ParseForm(); err != nil {
			return nil, &DecodeError{err}
		}
		if err := mergeValues(in, r.PostForm); err != nil {
			return nil, err
		}
	case ct == "multipart/form-data":
		r.Body = http.MaxBytesReader(nil, r.Body, MaxMultipartBytes)
		if err := r.ParseMultipartForm(MaxMultipartMemory); err != nil {
			return nil, &DecodeError{err}
		}
		if err := mergeValues(in, r.MultipartForm.Value); err != nil {
			return nil, err
		}
		if err := mergeFiles(in, r.MultipartForm.File); err != nil {
			return nil, err
		}
	}
	return in, nil
}

// numbers turns json.Number values into int64 or float64, so the rules see
// ordinary Go numbers.
func numbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		f, _ := t.Float64()
		return f
	case map[string]any:
		for k, x := range t {
			t[k] = numbers(x)
		}
	case []any:
		for i, x := range t {
			t[i] = numbers(x)
		}
	}
	return v
}

func mergeValues(dst map[string]any, vals url.Values) error {
	for _, key := range sortedKeys(vals) {
		list := vals[key]
		segs, appendLast, err := splitKey(key)
		if err != nil {
			return err
		}
		if appendLast || len(list) > 1 {
			items := make([]any, len(list))
			for i, s := range list {
				items[i] = s
			}
			setPath(dst, segs, items)
			continue
		}
		setPath(dst, segs, list[0])
	}
	listify(dst)
	return nil
}

func mergeFiles(dst map[string]any, files map[string][]*multipart.FileHeader) error {
	for _, key := range slices.Sorted(maps.Keys(files)) {
		list := files[key]
		segs, appendLast, err := splitKey(key)
		if err != nil {
			return err
		}
		if appendLast || len(list) > 1 {
			items := make([]any, len(list))
			for i, f := range list {
				items[i] = f
			}
			setPath(dst, segs, items)
			continue
		}
		setPath(dst, segs, list[0])
	}
	listify(dst)
	return nil
}

func sortedKeys(v url.Values) []string { return slices.Sorted(maps.Keys(v)) }

// splitKey turns "a[b][0]" or "a.b.0" into its segments. appendLast is true
// for keys ending in "[]". Keys deeper than MaxKeyDepth are an error.
func splitKey(key string) (segs []string, appendLast bool, err error) {
	if before, ok := strings.CutSuffix(key, "[]"); ok {
		key, appendLast = before, true
	}
	key = strings.ReplaceAll(key, "]", "")
	key = strings.ReplaceAll(key, "[", ".")
	if strings.Count(key, ".") >= MaxKeyDepth {
		short := key
		if len(short) > 64 {
			short = short[:64] + "..."
		}
		return nil, false, &DecodeError{fmt.Errorf("key %q is nested more than %d levels deep", short, MaxKeyDepth)}
	}
	return strings.Split(key, "."), appendLast, nil
}

// setPath stores v under segs, creating nested maps. Lists are made later
// by listify, once all keys are in.
func setPath(m map[string]any, segs []string, v any) {
	for i, s := range segs {
		if i == len(segs)-1 {
			m[s] = v
			return
		}
		next, ok := m[s].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[s] = next
		}
		m = next
	}
}

// listify converts nested maps whose keys are exactly 0..n-1 into lists,
// so "items[0][name]" yields a list of items.
func listify(m map[string]any) {
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			listify(sub)
			if l, ok := asList(sub); ok {
				m[k] = l
			}
		}
	}
}

func asList(m map[string]any) ([]any, bool) {
	if len(m) == 0 {
		return nil, false
	}
	out := make([]any, len(m))
	for k, v := range m {
		i, err := strconv.Atoi(k)
		if err != nil || i < 0 || i >= len(m) || strconv.Itoa(i) != k {
			return nil, false
		}
		if sub, ok := v.(map[string]any); ok {
			if l, ok := asList(sub); ok {
				v = l
			}
		}
		out[i] = v
	}
	return out, true
}
