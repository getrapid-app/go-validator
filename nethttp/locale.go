package nethttp

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/getrapid-app/go-validator"
)

// Localize is middleware that picks each request's language from its
// Accept-Language header and stores it with validator.ContextWithLocale,
// so validation messages come back in that language.
//
// supported lists the locales to choose from, such as "en", "ar" and
// "ar-EG". When it is empty, the locales of validator.DefaultTranslations
// are used. A request that matches none keeps the validator's own locale.
//
//	http.ListenAndServe(":8080", nethttp.Localize(mux))
func Localize(next http.Handler, supported ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l := PreferredLocale(r, supported...); l != "" {
			r = r.WithContext(validator.ContextWithLocale(r.Context(), l))
		}
		next.ServeHTTP(w, r)
	})
}

// PreferredLocale returns the best match between the request's
// Accept-Language header and supported (or the locales of
// validator.DefaultTranslations when supported is empty), or "" when
// nothing matches. "ar-EG" matches a supported "ar-EG" first, then "ar".
func PreferredLocale(r *http.Request, supported ...string) string {
	if len(supported) == 0 {
		supported = validator.DefaultTranslations.Locales()
	}
	index := make(map[string]string, len(supported))
	for _, s := range supported {
		index[normalize(s)] = s
	}
	for _, tag := range acceptLanguages(r.Header.Get("Accept-Language")) {
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

// acceptLanguages parses an Accept-Language header into tags, best first.
func acceptLanguages(header string) []string {
	type entry struct {
		tag string
		q   float64
	}
	var list []entry
	for _, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag = normalize(tag)
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

func normalize(l string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(l), "_", "-"))
}
