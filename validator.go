// Package validator brings Laravel-style validation to Go.
//
// Rules can be written three ways, and all three compile to the same Rule
// values, so they behave identically:
//
//	// 1. Laravel rule lists (or "required|email|max:255")
//	validator.Rules{"email": []string{"required", "email", "max:255"}}
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
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
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
	presence   PresenceVerifier
	now        func() time.Time
	password   CurrentPasswordChecker
}

// Option configures a Validator, either once in New or per call.
type Option func(*Validator)

// WithRegistry uses reg instead of the default registry.
func WithRegistry(reg *Registry) Option { return func(v *Validator) { v.registry = reg } }

// WithTranslator sets the translator and locale for messages. Validators use
// DefaultTranslations unless this option says otherwise.
func WithTranslator(t Translator, locale string) Option {
	return func(v *Validator) { v.translator, v.locale = t, locale }
}

// WithLocale sets the locale for messages, such as "ar" or "ar-EG". A locale
// set on the context with ContextWithLocale takes precedence.
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

// WithPresenceVerifier sets the lookup used by the exists and unique rules,
// typically backed by your database.
func WithPresenceVerifier(p PresenceVerifier) Option {
	return func(v *Validator) { v.presence = p }
}

// WithCurrentPassword sets the check used by the current_password rule.
func WithCurrentPassword(c CurrentPasswordChecker) Option {
	return func(v *Validator) { v.password = c }
}

// WithClock sets the function date rules use for "now", "today", "tomorrow"
// and "yesterday". It is mostly useful in tests.
func WithClock(now func() time.Time) Option { return func(v *Validator) { v.now = now } }

func merge(dst, src map[string]string) map[string]string {
	out := make(map[string]string, len(dst)+len(src))
	maps.Copy(out, dst)
	maps.Copy(out, src)
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
	v := &Validator{registry: DefaultRegistry, translator: DefaultTranslations, locale: "en"}
	for _, o := range opts {
		o(v)
	}
	return v
}

// With returns a copy of v with opts applied. v itself is not changed.
func (v *Validator) With(opts ...Option) *Validator { return v.with(opts) }

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

// fieldSpec is one compiled entry: a path pattern and its rules, with
// everything the engine needs per value worked out once at compile time.
type fieldSpec struct {
	pattern string
	rules   []Rule

	segs                      []string     // pattern split on dots
	list                      *listPattern // set for "prefix.*.suffix" patterns
	bail, nullable, sometimes bool
	excluders                 []excluder
	checks                    []ruleStep // rules to run, without flags and excluders
}

// ruleStep is a rule to run, with whether it is implicit.
type ruleStep struct {
	rule     Rule
	implicit bool
}

func newFieldSpec(pattern string, rules []Rule) fieldSpec {
	s := fieldSpec{pattern: pattern, rules: rules, segs: splitPath(pattern)}
	s.list = newListPattern(pattern)
	for _, r := range rules {
		switch r {
		case flagBail:
			s.bail = true
		case flagNullable:
			s.nullable = true
		case flagSometimes:
			s.sometimes = true
		}
		if _, ok := r.(flagRule); ok {
			continue
		}
		if ex, ok := r.(excluder); ok {
			s.excluders = append(s.excluders, ex)
			continue
		}
		s.checks = append(s.checks, ruleStep{rule: r, implicit: isImplicit(r)})
	}
	return s
}

// Validate checks data against rules. It returns nil when everything
// passes, a *ValidationError when at least one field fails, or another error
// when a rule set cannot be built or a rule aborts.
func (v *Validator) Validate(ctx context.Context, data any, rules Rules, opts ...Option) error {
	vv := v.with(opts)
	var buf [8]fieldSpec // most rule sets fit, so they need no heap slice
	specs, err := vv.compile(rules, buf[:0])
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

// compile builds the specs for rules, appending them to specs.
func (v *Validator) compile(rules Rules, specs []fieldSpec) ([]fieldSpec, error) {
	var buf [16]string
	patterns := buf[:0]
	for p := range rules {
		patterns = append(patterns, p)
	}
	// Go maps are unordered; sort so error order is stable between runs.
	slices.Sort(patterns)
	for _, p := range patterns {
		spec, ok, err := v.registry.cachedSpec(p, rules[p])
		if err != nil {
			return nil, fmt.Errorf("%w (field %q)", err, p)
		}
		if ok {
			specs = append(specs, spec)
		}
	}
	return specs, nil
}

func (v *Validator) run(ctx context.Context, root any, specs []fieldSpec, bag *Errors) error {
	// Without a caller's bag, failures go into one on the stack, which is
	// copied to the heap only when there is something to return.
	var local Errors
	b := bag
	if b == nil {
		b = &local
	}
	if l, ok := LocaleFrom(ctx); ok && l != v.locale {
		c := *v
		c.locale = l
		v = &c
	}
	var first [8]match
	e := expander{buf: first[:0]}
	for i := range specs {
		spec := &specs[i]
		e = e.expandSpec(root, spec)
		for _, m := range e.buf {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := v.field(ctx, root, spec, m, b); err != nil {
				return err
			}
		}
	}
	if b.IsEmpty() {
		return nil
	}
	if bag == nil {
		heap := local
		bag = &heap
	}
	return &ValidationError{Errors: bag}
}

func (v *Validator) field(ctx context.Context, root any, spec *fieldSpec, m match, bag *Errors) error {
	value, present := m.value, m.present
	if spec.sometimes && !present {
		return nil
	}
	f := Field{Path: m.path, Pattern: spec.pattern, Value: value, Present: present, root: root, rules: spec.rules, v: v}
	for _, ex := range spec.excluders {
		excluded, err := ex.excluded(ctx, f)
		if err != nil {
			return err
		}
		if excluded {
			return nil
		}
	}
	// Rules that are not implicit skip missing, blank and (with nullable)
	// nil values.
	skip := !present || isBlankString(value) || (spec.nullable && value == nil)
	for _, c := range spec.checks {
		if skip && !c.implicit {
			continue
		}
		err := c.rule.Validate(ctx, f)
		if err == nil {
			continue
		}
		if _, isFailure := err.(*Failure); !isFailure {
			if cause, ok := isAbort(err); ok {
				return cause
			}
		}
		bag.Add(m.path, v.failureMessage(f, c.rule, err))
		// Like Laravel: stop after the first failure with bail, and after a
		// failed implicit rule such as required, since the rest would only
		// repeat that the value is missing.
		if spec.bail || c.implicit {
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
	if m, ok := v.customMessage(f, []string{r.Name()}); ok {
		return v.replace(m, f, nil)
	}
	// A translation keyed by the rule's name replaces the error's text, so
	// rules that return plain errors can be translated too.
	if v.translator != nil {
		if m, ok := v.translator.Translate(v.locale, r.Name()); ok {
			return v.replace(m, f, nil)
		}
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
