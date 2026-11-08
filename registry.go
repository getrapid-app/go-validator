package validator

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Factory builds a rule from the parameters written after the colon in a
// rule string: "between:1,10" calls the "between" factory with ["1", "10"].
type Factory func(params []string) (Rule, error)

// Registry maps rule names to factories and default messages. Built-in rules
// are registered through the same API that users call for their own rules.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
	messages  map[string]string
	version   uint64 // bumped on Register, so compiled tag rules are rebuilt
}

// NewRegistry returns a registry with every built-in rule and its English
// message.
func NewRegistry() *Registry {
	r := &Registry{factories: map[string]Factory{}, messages: map[string]string{}}
	registerBuiltins(r)
	return r
}

// Register adds or replaces a rule factory. It returns r so calls chain.
func (r *Registry) Register(name string, f Factory) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = f
	r.version++
	return r
}

func (r *Registry) currentVersion() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

// SetMessage sets the default message for a key such as "even" or
// "max.string". It returns r so calls chain.
func (r *Registry) SetMessage(key, message string) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages[key] = message
	return r
}

// Make builds the named rule with params.
func (r *Registry) Make(name string, params ...string) (Rule, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("validator: unknown rule %q", name)
	}
	rule, err := f(params)
	if err != nil {
		return nil, fmt.Errorf("validator: rule %q: %w", name, err)
	}
	return rule, nil
}

func (r *Registry) message(key string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.messages[key]
	return m, ok
}

// Define starts a fluent definition of a custom rule:
//
//	reg.Define("even").
//		Message("The :attribute must be an even number.").
//		Check(func(ctx context.Context, f validator.Field, params []string) error {
//			n, ok := validator.As[int](f.Value)
//			if !ok || n%2 != 0 {
//				return validator.Fail("even", nil)
//			}
//			return nil
//		})
//
// After Check the rule is usable by name in rule strings ("required|even"),
// in struct tags and through Chain().Rule("even").
func (r *Registry) Define(name string) *Definition {
	return &Definition{reg: r, name: name}
}

// Definition is the builder returned by Registry.Define.
type Definition struct {
	reg      *Registry
	name     string
	implicit bool
	minArgs  int
}

// Message sets the default message for the rule.
func (d *Definition) Message(msg string) *Definition {
	d.reg.SetMessage(d.name, msg)
	return d
}

// Implicit makes the rule run even when the field is missing or empty.
func (d *Definition) Implicit() *Definition {
	d.implicit = true
	return d
}

// MinParams makes the rule string fail to compile unless at least n
// parameters are given.
func (d *Definition) MinParams(n int) *Definition {
	d.minArgs = n
	return d
}

// Check registers the rule with fn as its validation function and returns a
// constructor that builds the rule directly, for use with List or Chain.Use.
func (d *Definition) Check(fn func(ctx context.Context, f Field, params []string) error) func(params ...string) Rule {
	build := func(params ...string) Rule {
		p := append([]string(nil), params...)
		return &funcRule{name: d.name, implicit: d.implicit, fn: func(ctx context.Context, f Field) error {
			return fn(ctx, f, p)
		}}
	}
	minArgs := d.minArgs
	d.reg.Register(d.name, func(params []string) (Rule, error) {
		if len(params) < minArgs {
			return nil, fmt.Errorf("needs at least %d parameter(s)", minArgs)
		}
		return build(params...), nil
	})
	return build
}

// parsed is the RuleSet returned by Parse.
type parsed string

// Parse returns the rules written in Laravel's string syntax, for example
// "required|string|max:255". Rules are resolved when the validator runs, so
// custom rules registered later still work. Parameters are split on commas;
// for rules whose parameter may contain "|" or ",", such as regex, use the
// fluent API instead.
func Parse(rules string) RuleSet { return parsed(rules) }

func (p parsed) Build(r *Registry) ([]Rule, error) {
	var out []Rule
	for _, part := range strings.Split(string(p), "|") {
		rule, err := parseOne(part, r)
		if err != nil {
			return nil, err
		}
		if rule != nil {
			out = append(out, rule)
		}
	}
	return out, nil
}

// rawParamRules take everything after the colon as a single parameter,
// because their parameter is a pattern that may contain commas.
var rawParamRules = map[string]bool{"regex": true, "not_regex": true}

// parseOne builds a single rule written as "name" or "name:p1,p2".
func parseOne(s string, r *Registry) (Rule, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	name, rawParams, hasParams := strings.Cut(s, ":")
	name = strings.TrimSpace(name)
	var params []string
	if hasParams {
		if rawParamRules[name] {
			params = []string{rawParams}
		} else {
			params = strings.Split(rawParams, ",")
			for i := range params {
				params[i] = strings.TrimSpace(params[i])
			}
		}
	}
	return r.Make(name, params...)
}
