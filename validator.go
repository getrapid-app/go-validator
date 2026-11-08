// Package validator brings Laravel-style validation to Go.
//
// Rules can be written three ways, and all three compile to the same Rule
// values, so they behave identically:
//
//	// 1. Laravel string syntax
//	validator.Rules{"email": validator.Parse("required|email|max:255")}
//
//	// 2. Struct tags (same syntax as the strings)
//	type SignUp struct {
//		Email string `json:"email" validate:"required|email|max:255"`
//	}
//
//	// 3. Fluent API
//	validator.Rules{"email": validator.Chain().Required().Email().Max(255)}
//
// Input can be a struct, a map[string]any or anything that nests them.
// Paths use dot notation with wildcards ("items.*.price"). The core package
// has no dependencies and knows nothing about transports; HTTP and gRPC
// adapters live in their own subpackages.
package validator

import (
	"context"
	"sort"
	"strings"
)

// Validator holds the configuration for running validations: the rule
// registry, translations and custom messages. It is safe for concurrent use.
// The zero value is not usable; call New.
type Validator struct {
	registry   *Registry
	translator Translator
	locale     string
	messages   map[string]string
	attributes map[string]string
}

// Option configures a Validator, either once in New or per call.
type Option func(*Validator)

// WithRegistry uses reg instead of the default registry.
func WithRegistry(reg *Registry) Option { return func(v *Validator) { v.registry = reg } }

// WithTranslator sets the translator and locale for messages.
func WithTranslator(t Translator, locale string) Option {
	return func(v *Validator) { v.translator, v.locale = t, locale }
}

// WithLocale changes the locale passed to the translator.
func WithLocale(locale string) Option { return func(v *Validator) { v.locale = locale } }

// WithMessages adds custom messages. Keys are "rule", "field.rule" or
// "pattern.rule", for example "required", "email.required" or
// "items.*.price.min".
func WithMessages(m map[string]string) Option {
	return func(v *Validator) { v.messages = merge(v.messages, m) }
}

// WithAttributes sets display names for fields used in :attribute and
// :other, for example {"email": "email address"}.
func WithAttributes(m map[string]string) Option {
	return func(v *Validator) { v.attributes = merge(v.attributes, m) }
}

func merge(dst, src map[string]string) map[string]string {
	out := make(map[string]string, len(dst)+len(src))
	for k, val := range dst {
		out[k] = val
	}
	for k, val := range src {
		out[k] = val
	}
	return out
}

// DefaultRegistry is the registry used by validators created without
// WithRegistry. Register custom rules on it to make them available
// everywhere, or create your own with NewRegistry.
var DefaultRegistry = NewRegistry()

// Define starts a fluent custom rule definition on DefaultRegistry.
func Define(name string) *Definition { return DefaultRegistry.Define(name) }

// New returns a Validator.
func New(opts ...Option) *Validator {
	v := &Validator{registry: DefaultRegistry, locale: "en"}
	for _, o := range opts {
		o(v)
	}
	return v
}

func (v *Validator) with(opts []Option) *Validator {
	if len(opts) == 0 {
		return v
	}
	c := *v
	for _, o := range opts {
		o(&c)
	}
	return &c
}

// Registry returns the validator's rule registry.
func (v *Validator) Registry() *Registry { return v.registry }

// fieldSpec is one compiled entry: a path pattern and its rules.
type fieldSpec struct {
	pattern string
	rules   []Rule
}

// Validate checks data against rules. It returns nil when everything
// passes, a *ValidationError when at least one field fails, or another error
// when a rule set cannot be built or a rule aborts.
func (v *Validator) Validate(ctx context.Context, data any, rules Rules, opts ...Option) error {
	vv := v.with(opts)
	specs, err := vv.compile(rules)
	if err != nil {
		return err
	}
	return vv.run(ctx, normalize(data), specs, nil)
}

// Struct validates a struct using its `validate` tags.
func (v *Validator) Struct(ctx context.Context, s any, opts ...Option) error {
	vv := v.with(opts)
	specs, err := vv.tagSpecs(s)
	if err != nil {
		return err
	}
	return vv.run(ctx, normalize(s), specs, nil)
}

func (v *Validator) compile(rules Rules) ([]fieldSpec, error) {
	patterns := make([]string, 0, len(rules))
	for p := range rules {
		patterns = append(patterns, p)
	}
	// Go maps are unordered; sort so error order is stable between runs.
	sort.Strings(patterns)
	specs := make([]fieldSpec, 0, len(patterns))
	for _, p := range patterns {
		if rules[p] == nil {
			continue
		}
		built, err := rules[p].Build(v.registry)
		if err != nil {
			return nil, err
		}
		specs = append(specs, fieldSpec{pattern: p, rules: built})
	}
	return specs, nil
}

func (v *Validator) run(ctx context.Context, root any, specs []fieldSpec, bag *Errors) error {
	if bag == nil {
		bag = NewErrors()
	}
	for _, spec := range specs {
		for _, path := range expand(root, spec.pattern) {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := v.field(ctx, root, spec, path, bag); err != nil {
				return err
			}
		}
	}
	if bag.IsEmpty() {
		return nil
	}
	return &ValidationError{Errors: bag}
}

func (v *Validator) field(ctx context.Context, root any, spec fieldSpec, path string, bag *Errors) error {
	value, present := lookup(root, path)
	var bail, nullable, sometimes bool
	for _, r := range spec.rules {
		if fr, ok := r.(flagRule); ok {
			switch fr {
			case flagBail:
				bail = true
			case flagNullable:
				nullable = true
			case flagSometimes:
				sometimes = true
			}
		}
	}
	if sometimes && !present {
		return nil
	}
	f := Field{Path: path, Pattern: spec.pattern, Value: value, Present: present, root: root, rules: spec.rules}
	for _, r := range spec.rules {
		if _, ok := r.(flagRule); ok {
			continue
		}
		if !isImplicit(r) {
			if !present || isBlankString(value) {
				continue
			}
			if nullable && value == nil {
				continue
			}
		}
		err := r.Validate(ctx, f)
		if err == nil {
			continue
		}
		if cause, ok := isAbort(err); ok {
			return cause
		}
		bag.Add(path, v.failureMessage(f, r, err))
		if bail {
			break
		}
	}
	return nil
}

func (v *Validator) failureMessage(f Field, r Rule, err error) string {
	if fail, ok := err.(*Failure); ok {
		return v.message(f, r.Name(), fail)
	}
	// A plain error: its text is the message, still allowing a custom
	// message to override it and :attribute to be filled in.
	if m, ok := v.messages[f.Path+"."+r.Name()]; ok {
		return v.replace(m, f, nil)
	}
	if m, ok := v.messages[f.Pattern+"."+r.Name()]; ok {
		return v.replace(m, f, nil)
	}
	return v.replace(err.Error(), f, nil)
}

func isBlankString(v any) bool {
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) == ""
}

var std = New()

// Validate checks data against rules with the default validator.
func Validate(ctx context.Context, data any, rules Rules, opts ...Option) error {
	return std.Validate(ctx, data, rules, opts...)
}

// Struct validates a struct's `validate` tags with the default validator.
func Struct(ctx context.Context, s any, opts ...Option) error {
	return std.Struct(ctx, s, opts...)
}
