// Package playground lets go-validator use the validations of
// go-playground/validator alongside its Laravel rules. Each go-playground
// tag becomes a rule named "pg.<tag>", so it can never clash with a
// Laravel rule of the same name:
//
//	playground.Use() // once, at program start
//
//	type Signup struct {
//		Phone   string `json:"phone" validate:"required|pg.e164"`
//		Country string `json:"country" validate:"required|pg.iso3166_1_alpha2"`
//		Code    string `json:"code" validate:"required|pg.len:6"`
//		Color   string `json:"color" validate:"pg.oneof:red,green,blue"`
//	}
//
// The rules work in all three syntaxes: rule strings ("required|pg.e164"),
// struct tags, and the fluent API (Chain().Rule("pg.len", "6")).
//
// Parameters follow go-validator's syntax: a colon, then values separated
// by commas. "pg.len:6" runs go-playground's "len=6", and
// "pg.oneof:red,green,blue" runs "oneof=red green blue".
//
// Like every non-implicit rule, a pg rule is skipped when the field is
// missing or empty; use "required" (or "nullable", "sometimes") for
// presence. Messages use the keys "pg.<tag>", plus "pg.<tag>.string",
// ".numeric", ".array" or ".date" for size and comparison tags. English and
// Arabic are included, and they are translated and overridden like any
// other message.
//
// Only validations that look at a single value are available. Cross-field
// tags (eqfield, required_if, ...) need the whole struct, presence tags
// (required, omitempty, isdefault) are covered by the Laravel rules, and
// file-system tags (file, dir, image, mimetype, uds_exists) would read the
// server's disk from user input; use the Laravel rules for those cases.
package playground

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/getrapid-app/go-validator"
	pgv "github.com/go-playground/validator/v10"
)

// Prefix starts the name of every rule this package registers.
const Prefix = "pg."

// Option configures Register and Use.
type Option func(*config)

type config struct {
	validate *pgv.Validate
	extra    []string
}

// WithValidate makes the rules run on v instead of a new go-playground
// validator, so validations and aliases registered on it with
// RegisterValidation or RegisterAlias can be used too. Name those with
// Tags.
func WithValidate(v *pgv.Validate) Option {
	return func(c *config) { c.validate = v }
}

// Tags registers rules for tags that are not built into go-playground,
// such as custom validations or aliases registered on the validator given
// to WithValidate. Their parameter is optional.
func Tags(names ...string) Option {
	return func(c *config) { c.extra = append(c.extra, names...) }
}

// Use registers the pg rules on validator.DefaultRegistry and their Arabic
// messages on validator.DefaultTranslations. Call it once at program start,
// before validating.
func Use(opts ...Option) {
	Register(validator.DefaultRegistry, opts...)
	AddTranslations(validator.DefaultTranslations)
}

// Register adds the pg rules and their English messages to reg.
func Register(reg *validator.Registry, opts ...Option) {
	var c config
	for _, o := range opts {
		o(&c)
	}
	if c.validate == nil {
		c.validate = pgv.New()
	}
	for tag, info := range tags {
		reg.Register(Prefix+tag, factory(c.validate, tag, info))
	}
	for _, tag := range c.extra {
		reg.Register(Prefix+tag, factory(c.validate, tag, tagInfo{param: paramOptional}))
	}
	for key, msg := range english {
		reg.SetMessage(Prefix+key, msg)
	}
}

// AddTranslations adds the Arabic messages of the pg rules to t. English
// comes with Register, since English messages live in the registry.
func AddTranslations(t *validator.Translations) {
	ar := make(map[string]string, len(arabic))
	for key, msg := range arabic {
		ar[Prefix+key] = msg
	}
	t.Add("ar", ar)
}

func factory(v *pgv.Validate, tag string, info tagInfo) validator.Factory {
	return func(params []string) (validator.Rule, error) {
		if len(params) == 0 && info.param == paramRequired {
			return nil, fmt.Errorf("needs a parameter, as in %s%s:value", Prefix, tag)
		}
		if len(params) > 0 && info.param == paramNone {
			return nil, fmt.Errorf("takes no parameters")
		}
		r := &rule{name: Prefix + tag, tag: tag, typed: info.typed, list: info.listSep, v: v}
		if len(params) > 0 {
			r.tag += "=" + tagParam(params, info.listSep)
			shown := strings.Join(params, ", ")
			r.params = map[string]any{"param": shown, "values": shown}
		}
		return r, nil
	}
}

// tagParam writes params as a go-playground tag parameter. Lists are
// separated by spaces, with values that contain one quoted. Commas and
// pipes, which separate go-playground tags, are written as the hex escapes
// go-playground reads back.
func tagParam(params []string, list bool) string {
	var s string
	if list {
		quoted := make([]string, len(params))
		for i, p := range params {
			if strings.ContainsAny(p, " \t") {
				p = "'" + p + "'"
			}
			quoted[i] = p
		}
		s = strings.Join(quoted, " ")
	} else {
		s = strings.Join(params, ",")
	}
	return strings.NewReplacer(",", "0x2C", "|", "0x7C").Replace(s)
}

// rule runs one go-playground tag on a field's value.
type rule struct {
	name   string // "pg.len"
	tag    string // "len=6"
	typed  bool
	list   bool
	params map[string]any
	v      *pgv.Validate
}

func (r *rule) Name() string { return r.name }

// Validate runs the tag. go-playground panics when a tag is used on a type
// it does not support (email on a number, say); that counts as a failure,
// just as a Laravel rule fails on a value of the wrong type.
func (r *rule) Validate(ctx context.Context, f validator.Field) (err error) {
	defer func() {
		if recover() != nil {
			err = r.fail(f.Value)
		}
	}()
	value := f.Value
	if r.list {
		// Decoded JSON numbers are float64, which go-playground's list
		// tags do not accept; whole numbers are compared as integers.
		if n, ok := value.(float64); ok && n == float64(int64(n)) {
			value = int64(n)
		}
	}
	if r.v.VarCtx(ctx, value, r.tag) != nil {
		return r.fail(f.Value)
	}
	return nil
}

func (r *rule) fail(value any) error {
	key := r.name
	if r.typed {
		if k := kind(value); k != "" {
			key += "." + k
		}
	}
	return validator.Fail(key, r.params)
}

// kind picks the message variant for size and comparison tags.
func kind(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case time.Time:
		return "date"
	case []any, map[string]any:
		return "array"
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "numeric"
	case reflect.String:
		return "string"
	case reflect.Slice, reflect.Array, reflect.Map:
		return "array"
	}
	return ""
}

// TagNames returns the go-playground tags available as pg rules, without
// the prefix, sorted.
func TagNames() []string {
	return slices.Sorted(maps.Keys(tags))
}

// MessageKeys returns the message keys of the pg rules, with the prefix,
// sorted. Use them to write translations for another language.
func MessageKeys() []string {
	keys := make([]string, 0, len(english))
	for k := range english {
		keys = append(keys, Prefix+k)
	}
	slices.Sort(keys)
	return keys
}
