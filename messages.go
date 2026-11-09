package validator

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Translator supplies messages for a locale. Keys are rule names or variants
// such as "max.string", plus "attributes.<field>" for display names and
// "custom.<field>.<key>" for messages of one field. Return ok=false to fall
// back to the registry's default English message.
//
// Translations is the built-in implementation; implement Translator to plug
// in another i18n library.
type Translator interface {
	Translate(locale, key string) (msg string, ok bool)
}

// MapTranslator is a minimal Translator backed by nested maps: locale, then
// key. It has no fallback between locales; use Translations for that.
type MapTranslator map[string]map[string]string

func (m MapTranslator) Translate(locale, key string) (string, bool) {
	msg, ok := m[locale][key]
	return msg, ok
}

// Message returns the text for message key on field, resolved and filled
// in exactly as for a failing rule. Adapters use it to report problems found
// before validation, such as a value that cannot be decoded.
//
//	v.Message("age", "integer", nil) // "The age field must be an integer."
func (v *Validator) Message(field, key string, params map[string]any) string {
	rule, _, _ := strings.Cut(key, ".")
	return v.message(Field{Path: field, Pattern: field, v: v}, rule, &Failure{Key: key, Params: params})
}

// Translate returns the message for key in the locale of ctx (set with
// ContextWithLocale) or the validator's locale, with :name placeholders
// filled from params. It looks in the translator, then the registry's
// default messages, and returns key itself when nothing is found.
// Adapters use it for texts that are not about one field, such as
// "http.unauthorized".
func (v *Validator) Translate(ctx context.Context, key string, params map[string]any) string {
	if l, ok := LocaleFrom(ctx); ok && l != v.locale {
		c := *v
		c.locale = l
		v = &c
	}
	tmpl, ok := v.translate(key)
	if !ok {
		return key
	}
	names := make([]string, 0, len(params))
	for k := range params {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	pairs := make([]string, 0, 2*len(names))
	for _, k := range names {
		pairs = append(pairs, ":"+k, formatParam(params[k]))
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

// Translate translates key with the default validator. See
// Validator.Translate.
func Translate(ctx context.Context, key string, params map[string]any) string {
	return std.Translate(ctx, key, params)
}

// Summary is Laravel's top-level message for a failed validation: the first
// error, plus how many more there are, such as "The email field is
// required. (and 2 more errors)". It is translated like any message, through
// the keys http.invalid (no errors), http.more_error and http.more_errors
// (:message and :count), in the locale of ctx.
func (v *Validator) Summary(ctx context.Context, bag *Errors) string {
	fields := bag.Fields()
	if len(fields) == 0 {
		return v.Translate(ctx, "http.invalid", nil)
	}
	first := bag.First(fields[0])
	switch n := bag.Count() - 1; n {
	case 0:
		return first
	case 1:
		return v.Translate(ctx, "http.more_error", map[string]any{"message": first, "count": n})
	default:
		return v.Translate(ctx, "http.more_errors", map[string]any{"message": first, "count": n})
	}
}

// message resolves the text for a failure, in Laravel's order:
//  1. custom message for "field.key" or "pattern.key" (also "field.rule")
//  2. custom message for "key" (also "rule")
//  3. the translator's "custom.field.key" or "custom.pattern.key"
//  4. the translator's "key" (also "rule"), then the registry default
func (v *Validator) message(f Field, ruleName string, fail *Failure) string {
	keys := []string{fail.Key}
	if fail.Key != ruleName {
		keys = append(keys, ruleName)
	}
	if tmpl, ok := v.customMessage(f, keys); ok {
		return v.replace(tmpl, f, fail.Params)
	}
	for _, k := range keys {
		if m, ok := v.translate(k); ok {
			return v.replace(m, f, fail.Params)
		}
	}
	return v.replace(v.fallbackMessage(), f, fail.Params)
}

// customMessage finds a message written for this field or key, in code
// (WithMessages, a request's Messages) or in the translations' "custom"
// section.
func (v *Validator) customMessage(f Field, keys []string) (string, bool) {
	for _, k := range keys {
		for _, c := range []string{f.Path + "." + k, f.Pattern + "." + k, k} {
			if m, ok := v.messages[c]; ok {
				return m, true
			}
		}
	}
	if v.translator == nil {
		return "", false
	}
	for _, k := range keys {
		for _, c := range []string{f.Path + "." + k, f.Pattern + "." + k} {
			if m, ok := v.translator.Translate(v.locale, "custom."+c); ok {
				return m, true
			}
		}
	}
	return "", false
}

// translate looks key up in the translator, then in the registry's default
// messages.
func (v *Validator) translate(key string) (string, bool) {
	if v.translator != nil {
		if m, ok := v.translator.Translate(v.locale, key); ok {
			return m, true
		}
	}
	if v.registry != nil {
		return v.registry.message(key)
	}
	return "", false
}

func (v *Validator) fallbackMessage() string {
	if m, ok := v.translate("invalid"); ok {
		return m
	}
	return "The :attribute field is invalid."
}

func (v *Validator) replace(tmpl string, f Field, params map[string]any) string {
	pairs := []string{":attribute", v.attributeName(f.Path, f.Pattern)}
	// Longest names first, so ":min_digits" is not cut by ":min".
	names := make([]string, 0, len(params))
	for k := range params {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, k := range names {
		val := params[k]
		switch t := val.(type) {
		case attrNames:
			names := make([]string, len(t))
			for i, n := range t {
				names[i] = v.attributeName(n, n)
			}
			val = strings.Join(names, " / ")
		case string:
			if k == "other" {
				val = v.attributeName(t, t)
			}
		}
		pairs = append(pairs, ":"+k, formatParam(val))
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

// attributeName returns the display name of a field: a custom attribute name
// when one is set, otherwise the path with underscores turned into spaces.
func (v *Validator) attributeName(path, pattern string) string {
	if n, ok := v.attributes[path]; ok {
		return n
	}
	if n, ok := v.attributes[pattern]; ok {
		return n
	}
	if v.translator != nil {
		if n, ok := v.translator.Translate(v.locale, "attributes."+path); ok {
			return n
		}
		if n, ok := v.translator.Translate(v.locale, "attributes."+pattern); ok {
			return n
		}
	}
	return strings.ReplaceAll(path, "_", " ")
}

func formatParam(v any) string {
	switch t := v.(type) {
	case []string:
		return strings.Join(t, ", ")
	case float64:
		return formatFloat(t)
	default:
		return fmt.Sprint(v)
	}
}
