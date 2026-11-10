package validator

import (
	"context"
	"encoding"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"
)

// Bind decodes input into a new T and validates it with the request
// pattern (see ValidateRequest). input is JSON-shaped data such as the
// map[string]any from json.Unmarshal or nethttp.Input; struct fields are
// matched by their json names.
//
// Rules see the decoded struct, plus any input keys that have no struct
// field (such as password_confirmation), so Prepare's changes and
// accept_zero apply as usual.
//
// Decoding is lenient in the way form input needs: "42" fills an int field
// and "true" a bool field. A value that cannot be converted is reported as
// a validation error on its field ("The age field must be an integer."),
// replacing that field's other messages, and the field keeps its zero value.
//
//	req, err := validator.Bind[CreateOrder](ctx, input)
func Bind[T any](ctx context.Context, input any, opts ...Option) (*T, error) {
	return BindWith[T](ctx, std, input, opts...)
}

// BindWith is Bind using a specific Validator.
func BindWith[T any](ctx context.Context, v *Validator, input any, opts ...Option) (*T, error) {
	req := new(T)
	typeErrs := Decode(input, req)
	err := v.validateRequest(ctx, req, input, opts)
	if len(typeErrs) == 0 {
		return req, err
	}
	bag, isValidation := ErrorsOf(err)
	if err != nil && !isValidation {
		return req, err
	}
	if bag == nil {
		bag = NewErrors()
	}
	vv := v.with(requestOptions(req, opts))
	for _, te := range typeErrs {
		bag.Set(te.Path, vv.Message(te.Path, te.Kind, nil))
	}
	return req, &ValidationError{Errors: bag}
}

// TypeError reports an input value that could not be stored in its Go
// field. Kind is the message key that describes the expected type:
// "integer", "numeric", "boolean", "string", "array", "date" or "file".
type TypeError struct {
	Path string
	Kind string
}

// Decode stores input in the value dst points to, matching struct fields by
// their json names and converting strings to numbers, booleans and
// encoding.TextUnmarshaler types. It returns one TypeError per value that
// did not fit; those fields keep their zero values.
func Decode(input any, dst any) []TypeError {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return []TypeError{{Path: "", Kind: "array"}}
	}
	d := &decoder{}
	d.assign(rv.Elem(), input, "")
	return d.errs
}

type decoder struct{ errs []TypeError }

func (d *decoder) fail(path, kind string) { d.errs = append(d.errs, TypeError{Path: path, Kind: kind}) }

func join(path, seg string) string {
	if path == "" {
		return seg
	}
	return path + "." + seg
}

var (
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
	fileHeaderPtrType   = reflect.TypeFor[*multipart.FileHeader]()
)

func (d *decoder) assign(dst reflect.Value, src any, path string) {
	if src == nil {
		return
	}
	t := dst.Type()

	// Files.
	if t == fileHeaderPtrType {
		if fh, ok := src.(*multipart.FileHeader); ok {
			dst.Set(reflect.ValueOf(fh))
		} else {
			d.fail(path, "file")
		}
		return
	}
	if t == fileType {
		switch f := src.(type) {
		case *multipart.FileHeader:
			dst.Set(reflect.ValueOf(FromMultipart(f)))
		case File:
			dst.Set(reflect.ValueOf(f))
		default:
			d.fail(path, "file")
		}
		return
	}

	switch t.Kind() {
	case reflect.Pointer:
		elem := reflect.New(t.Elem())
		before := len(d.errs)
		d.assign(elem.Elem(), src, path)
		if len(d.errs) == before {
			dst.Set(elem)
		}
		return
	case reflect.Interface:
		if v := reflect.ValueOf(src); v.Type().AssignableTo(t) {
			dst.Set(v)
		}
		return
	}

	// Already the right type (time.Time from a struct input, for example).
	if v := reflect.ValueOf(src); v.Type().AssignableTo(t) && t.Kind() != reflect.Map && t.Kind() != reflect.Slice {
		dst.Set(v)
		return
	}

	// Text-based types such as time.Time, netip.Addr or custom IDs.
	if reflect.PointerTo(t).Implements(textUnmarshalerType) {
		s, ok := src.(string)
		if !ok {
			d.fail(path, textKind(t))
			return
		}
		if err := dst.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s)); err != nil {
			d.fail(path, textKind(t))
		}
		return
	}

	switch t.Kind() {
	case reflect.Struct:
		m, ok := src.(map[string]any)
		if !ok {
			d.fail(path, "array")
			return
		}
		d.assignStruct(dst, m, path)
	case reflect.Map:
		m, ok := src.(map[string]any)
		if !ok || t.Key().Kind() != reflect.String {
			d.fail(path, "array")
			return
		}
		out := reflect.MakeMapWithSize(t, len(m))
		for k, v := range m {
			ev := reflect.New(t.Elem()).Elem()
			d.assign(ev, v, join(path, k))
			out.SetMapIndex(reflect.ValueOf(k).Convert(t.Key()), ev)
		}
		dst.Set(out)
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			if s, ok := src.(string); ok && t.Kind() == reflect.Slice {
				dst.SetBytes([]byte(s))
				return
			}
		}
		list, ok := src.([]any)
		if !ok {
			list = []any{src} // a single form value for a list field
		}
		if t.Kind() == reflect.Array {
			for i := 0; i < t.Len() && i < len(list); i++ {
				d.assign(dst.Index(i), list[i], join(path, strconv.Itoa(i)))
			}
			return
		}
		out := reflect.MakeSlice(t, len(list), len(list))
		for i, v := range list {
			d.assign(out.Index(i), v, join(path, strconv.Itoa(i)))
		}
		dst.Set(out)
	case reflect.String:
		s, ok := src.(string)
		if !ok {
			d.fail(path, "string")
			return
		}
		dst.SetString(s)
	case reflect.Bool:
		b, ok := toBool(src)
		if !ok {
			d.fail(path, "boolean")
			return
		}
		dst.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, ok := toInt(src)
		if !ok || dst.OverflowInt(n) {
			d.fail(path, "integer")
			return
		}
		dst.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, ok := toInt(src)
		if !ok || n < 0 || dst.OverflowUint(uint64(n)) {
			d.fail(path, "integer")
			return
		}
		dst.SetUint(uint64(n))
	case reflect.Float32, reflect.Float64:
		f, ok := toFloat(src)
		if !ok || dst.OverflowFloat(f) {
			d.fail(path, "numeric")
			return
		}
		dst.SetFloat(f)
	default:
		d.fail(path, "string")
	}
}

func (d *decoder) assignStruct(dst reflect.Value, m map[string]any, path string) {
	t := dst.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		name, ok := fieldName(sf)
		if !ok {
			continue
		}
		fv := dst.Field(i)
		if sf.Anonymous && name == "" {
			if fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				fv = fv.Elem()
			}
			d.assignStruct(fv, m, path)
			continue
		}
		v, present := m[name]
		if !present {
			continue
		}
		d.assign(fv, v, join(path, name))
	}
}

func textKind(t reflect.Type) string {
	if t == timeType {
		return "date"
	}
	return "string"
}

func toBool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "1", "true", "on", "yes":
			return true, true
		case "0", "false", "off", "no", "":
			return false, true
		}
		return false, false
	}
	if n, ok := toFloat(v); ok && (n == 0 || n == 1) {
		return n == 1, true
	}
	return false, false
}

// requestOptions adds the options a request supplies through its
// MessagesProvider and AttributesProvider hooks.
func requestOptions(req any, opts []Option) []Option {
	out := append([]Option(nil), opts...)
	if m, ok := req.(MessagesProvider); ok {
		out = append(out, WithMessages(m.Messages()))
	}
	if a, ok := req.(AttributesProvider); ok {
		out = append(out, WithAttributes(a.Attributes()))
	}
	return out
}
