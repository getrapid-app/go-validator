package validator

import (
	"fmt"
	"sort"
	"strings"
)

// Translator supplies messages for a locale. Keys are rule names or variants
// such as "max.string". Return ok=false to fall back to the registry's
// default English message.
type Translator interface {
	Translate(locale, key string) (msg string, ok bool)
}

// MapTranslator is a Translator backed by nested maps: locale, then key.
type MapTranslator map[string]map[string]string

func (m MapTranslator) Translate(locale, key string) (string, bool) {
	msg, ok := m[locale][key]
	return msg, ok
}

// message resolves the text for a failure, in Laravel's order:
//  1. custom message for "field.key" or "pattern.key" (also "field.rule")
//  2. custom message for "key" (also "rule")
//  3. translator, then the registry default
func (v *Validator) message(f Field, ruleName string, fail *Failure) string {
	keys := []string{fail.Key}
	if fail.Key != ruleName {
		keys = append(keys, ruleName)
	}
	var tmpl string
	found := false
	for _, k := range keys {
		for _, c := range []string{f.Path + "." + k, f.Pattern + "." + k, k} {
			if m, ok := v.messages[c]; ok {
				tmpl, found = m, true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		for _, k := range keys {
			if v.translator != nil {
				if m, ok := v.translator.Translate(v.locale, k); ok {
					tmpl, found = m, true
					break
				}
			}
			if m, ok := v.registry.message(k); ok {
				tmpl, found = m, true
				break
			}
		}
	}
	if !found {
		tmpl = "The :attribute field is invalid."
	}
	return v.replace(tmpl, f, fail.Params)
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
		if k == "other" {
			if s, ok := val.(string); ok {
				val = v.attributeName(s, s)
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
