package nethttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
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
	mergeValues(in, r.URL.Query())

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
		for k, val := range obj {
			in[k] = val
		}
	case ct == "application/x-www-form-urlencoded":
		r.Body = http.MaxBytesReader(nil, r.Body, MaxBodyBytes)
		if err := r.ParseForm(); err != nil {
			return nil, &DecodeError{err}
		}
		mergeValues(in, r.PostForm)
	case ct == "multipart/form-data":
		if err := r.ParseMultipartForm(MaxMultipartMemory); err != nil {
			return nil, &DecodeError{err}
		}
		mergeValues(in, r.MultipartForm.Value)
		mergeFiles(in, r.MultipartForm.File)
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

func mergeValues(dst map[string]any, vals url.Values) {
	for _, key := range sortedKeys(vals) {
		list := vals[key]
		segs, appendLast := splitKey(key)
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
}

func mergeFiles(dst map[string]any, files map[string][]*multipart.FileHeader) {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		list := files[key]
		segs, appendLast := splitKey(key)
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
}

func sortedKeys(v url.Values) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// splitKey turns "a[b][0]" or "a.b.0" into its segments. appendLast is true
// for keys ending in "[]".
func splitKey(key string) (segs []string, appendLast bool) {
	if strings.HasSuffix(key, "[]") {
		key, appendLast = strings.TrimSuffix(key, "[]"), true
	}
	key = strings.ReplaceAll(key, "]", "")
	key = strings.ReplaceAll(key, "[", ".")
	return strings.Split(key, "."), appendLast
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
