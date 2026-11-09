package nethttp

import (
	"net/http"

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
	return validator.MatchLocale(r.Header.Get("Accept-Language"), supported...)
}
