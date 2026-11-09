package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Translations holds validation messages per locale. It is the default
// Translator: every validator created with New uses DefaultTranslations
// unless WithTranslator says otherwise.
//
// English and Arabic ship with the package. A team can add its own
// language, or override any message of an existing one, from Go code or
// from JSON files:
//
//	validator.AddTranslations("fr", map[string]string{
//		"required":   "Le champ :attribute est obligatoire.",
//		"max.string": "Le champ :attribute ne doit pas dépasser :max caractères.",
//	})
//
//	// Egyptian slang on top of the built-in Arabic. Keys that are not
//	// overridden still come from "ar", then from English.
//	validator.AddTranslations("ar-EG", map[string]string{
//		"required": "لازم تكتب :attribute.",
//	})
//
// Keys are the message keys listed in docs/RULES.md ("required",
// "max.string", "password.mixed", ...) or the name of a custom rule. Two
// prefixes work like the sections of Laravel's lang files:
//
//   - "attributes.<field>" is the display name of a field, used for
//     :attribute and :other ("attributes.email": "البريد الإلكتروني").
//   - "custom.<field>.<key>" is a message for one field only
//     ("custom.email.required"). The field may be a pattern such as
//     "items.*.price".
//
// A locale is looked up in this order: the locale itself ("ar-EG"), its
// language ("ar"), the fallback locale ("en" unless SetFallback changes it),
// and finally the default message the rule was registered with. Locale names
// are case-insensitive, and "ar_EG" is the same as "ar-EG".
//
// Translations is safe for concurrent use, so messages can be added while
// validations run.
type Translations struct {
	mu       sync.RWMutex
	locales  map[string]map[string]string
	fallback string
	prefixes map[string]bool // "custom." and "attributes." when used
}

// NewTranslations returns translations with the built-in languages (English
// and Arabic) and "en" as the fallback locale.
func NewTranslations() *Translations {
	t := &Translations{locales: map[string]map[string]string{}, fallback: "en"}
	// English lives in the registry as each rule's default message, so a
	// message changed with Registry.SetMessage still shows. The "en" entry
	// only holds overrides.
	t.locales["en"] = map[string]string{}
	t.Add("ar", arabicMessages)
	return t
}

// DefaultTranslations are the translations used by validators that were not
// given a Translator. Add languages or overrides to it at program start to
// make them available everywhere.
var DefaultTranslations = NewTranslations()

// AddTranslations adds messages for locale to DefaultTranslations,
// replacing any message with the same key. See Translations.
func AddTranslations(locale string, messages map[string]string) {
	DefaultTranslations.Add(locale, messages)
}

// LoadTranslations loads JSON translation files into DefaultTranslations.
// See Translations.LoadFS.
func LoadTranslations(fsys fs.FS, pattern string) error {
	return DefaultTranslations.LoadFS(fsys, pattern)
}

// Add merges messages into locale, creating the locale if it is new.
// Existing keys are replaced, so Add both adds languages and overrides
// messages in a built-in one.
func (t *Translations) Add(locale string, messages map[string]string) *Translations {
	locale = normalizeLocale(locale)
	t.mu.Lock()
	defer t.mu.Unlock()
	dst, ok := t.locales[locale]
	if !ok {
		dst = make(map[string]string, len(messages))
		t.locales[locale] = dst
	}
	for k, v := range messages {
		dst[k] = v
		for _, p := range [...]string{"custom.", "attributes."} {
			if strings.HasPrefix(k, p) {
				if t.prefixes == nil {
					t.prefixes = map[string]bool{}
				}
				t.prefixes[p] = true
			}
		}
	}
	return t
}

// AddJSON merges a JSON object of messages into locale. The object may be
// flat ({"max.string": "..."}) or nested like Laravel's lang files:
//
//	{
//	  "required": "The :attribute field is required.",
//	  "max": {"string": "The :attribute may not be longer than :max characters."},
//	  "attributes": {"email": "email address"},
//	  "custom": {"email": {"required": "We need your email."}}
//	}
func (t *Translations) AddJSON(locale string, data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("validator: translations for %q: %w", locale, err)
	}
	flat := map[string]string{}
	if err := flatten("", raw, flat); err != nil {
		return fmt.Errorf("validator: translations for %q: %w", locale, err)
	}
	t.Add(locale, flat)
	return nil
}

// LoadFS loads every file in fsys that matches pattern (as in fs.Glob) with
// AddJSON. The locale is the file name without its extension, so
// "lang/ar-EG.json" adds to "ar-EG". It works well with embed:
//
//	//go:embed lang/*.json
//	var langFiles embed.FS
//
//	validator.LoadTranslations(langFiles, "lang/*.json")
func (t *Translations) LoadFS(fsys fs.FS, pattern string) error {
	files, err := fs.Glob(fsys, pattern)
	if err != nil {
		return fmt.Errorf("validator: %w", err)
	}
	for _, name := range files {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("validator: %w", err)
		}
		base := path.Base(name)
		if err := t.AddJSON(strings.TrimSuffix(base, path.Ext(base)), data); err != nil {
			return err
		}
	}
	return nil
}

// SetFallback sets the locale used when a message is missing from the
// requested locale and its language. It is "en" by default.
func (t *Translations) SetFallback(locale string) *Translations {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fallback = normalizeLocale(locale)
	return t
}

// Locales returns the locales that have messages, sorted.
func (t *Translations) Locales() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]string, 0, len(t.locales))
	for l := range t.locales {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// Has reports whether locale, or its language, has messages.
func (t *Translations) Has(locale string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, l := range t.chain(normalizeLocale(locale), false) {
		if _, ok := t.locales[l]; ok {
			return true
		}
	}
	return false
}

// Translate implements Translator, looking through the locale, its
// language and the fallback locale.
func (t *Translations) Translate(locale, key string) (string, bool) {
	locale = normalizeLocale(locale)
	t.mu.RLock()
	defer t.mu.RUnlock()
	if m, ok := t.locales[locale][key]; ok {
		return m, true
	}
	if base, _, ok := strings.Cut(locale, "-"); ok {
		if m, ok := t.locales[base][key]; ok {
			return m, true
		}
	}
	if t.fallback != "" && t.fallback != locale {
		if m, ok := t.locales[t.fallback][key]; ok {
			return m, true
		}
	}
	return "", false
}

// hasPrefix reports whether any locale has a key starting with prefix
// ("custom." or "attributes."), so lookups that cannot match are skipped.
func (t *Translations) hasPrefix(prefix string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.prefixes[prefix]
}

// chain lists the locales to search: "ar-eg", "ar", then the fallback.
func (t *Translations) chain(locale string, withFallback bool) []string {
	out := []string{locale}
	if base, _, ok := strings.Cut(locale, "-"); ok {
		out = append(out, base)
	}
	if withFallback && t.fallback != "" && t.fallback != locale {
		out = append(out, t.fallback)
	}
	return out
}

func normalizeLocale(l string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(l), "_", "-"))
}

func flatten(prefix string, in map[string]any, out map[string]string) error {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch t := v.(type) {
		case string:
			out[key] = t
		case map[string]any:
			if err := flatten(key, t, out); err != nil {
				return err
			}
		default:
			return fmt.Errorf("key %q: want a string or an object, got %T", key, v)
		}
	}
	return nil
}

// --- locale in the context ------------------------------------------------

type localeKey struct{}

// ContextWithLocale returns a context that makes validations use locale
// instead of the validator's own, so one Validator can answer each request
// in its user's language. The nethttp adapter's Localize middleware sets it
// from the Accept-Language header.
func ContextWithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKey{}, locale)
}

// LocaleFrom returns the locale set with ContextWithLocale.
func LocaleFrom(ctx context.Context) (string, bool) {
	l, ok := ctx.Value(localeKey{}).(string)
	return l, ok && l != ""
}

// MatchLocale returns the entry of supported that best matches an
// Accept-Language value such as "ar-EG,ar;q=0.9,en;q=0.8", or "" when none
// does. Tags are tried in order of their q weight; "ar-EG" matches a
// supported "ar-EG" first, then "ar". When supported is empty, the locales
// of DefaultTranslations are used. Adapters use it to pick a request's
// locale.
func MatchLocale(acceptLanguage string, supported ...string) string {
	if len(supported) == 0 {
		supported = DefaultTranslations.Locales()
	}
	index := make(map[string]string, len(supported))
	for _, s := range supported {
		index[normalizeLocale(s)] = s
	}
	for _, tag := range acceptLanguages(acceptLanguage) {
		if s, ok := index[tag]; ok {
			return s
		}
		if base, _, ok := strings.Cut(tag, "-"); ok {
			if s, ok := index[base]; ok {
				return s
			}
		}
	}
	return ""
}

// acceptLanguages parses an Accept-Language value into tags, best first.
func acceptLanguages(header string) []string {
	type entry struct {
		tag string
		q   float64
	}
	var list []entry
	for _, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag = normalizeLocale(tag)
		if tag == "" || tag == "*" {
			continue
		}
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				q = f
			}
		}
		if q > 0 {
			list = append(list, entry{tag, q})
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].q > list[j].q })
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.tag
	}
	return out
}
