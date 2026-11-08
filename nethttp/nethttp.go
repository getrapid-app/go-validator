// Package nethttp connects go-validator to net/http. It reads a request
// into a request type, validates it, and writes failures as the JSON
// responses Laravel sends, so existing front ends keep working:
//
//	422 {"message": "The email field is required. (and 1 more error)",
//	     "errors": {"email": ["The email field is required."], ...}}
//
// It depends only on the standard library and works with any router built
// on http.Handler.
package nethttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/getrapid-app/go-validator"
)

// Bind reads r (query, then a JSON, urlencoded or multipart body) into a
// new T and validates it, running T's request hooks (Authorize, Prepare,
// Rules, Messages, Attributes, After). The request's context is passed to
// the rules.
//
//	func createOrder(w http.ResponseWriter, r *http.Request) {
//		req, err := nethttp.Bind[CreateOrder](r)
//		if err != nil {
//			nethttp.WriteError(w, err)
//			return
//		}
//		...
//	}
func Bind[T any](r *http.Request, opts ...validator.Option) (*T, error) {
	return BindWith[T](validator.New(), r, opts...)
}

// BindWith is Bind using a specific Validator, for example one with a
// PresenceVerifier for exists and unique.
func BindWith[T any](v *validator.Validator, r *http.Request, opts ...validator.Option) (*T, error) {
	in, err := Input(r)
	if err != nil {
		return nil, err
	}
	return validator.BindWith[T](r.Context(), v, in, opts...)
}

// Validate validates the request's input against rules without a request
// type, and returns the input. Missing and present fields are seen exactly
// as sent, which suits rules such as sometimes, present and missing.
func Validate(r *http.Request, rules validator.Rules, opts ...validator.Option) (map[string]any, error) {
	in, err := Input(r)
	if err != nil {
		return nil, err
	}
	return in, validator.Validate(r.Context(), in, rules, opts...)
}

// Handle adapts a handler that receives an already validated request. When
// binding fails, the error is written with WriteError and fn is not called.
//
//	mux.Handle("POST /orders", nethttp.Handle(func(w http.ResponseWriter, r *http.Request, req *CreateOrder) {
//		...
//	}))
func Handle[T any](fn func(w http.ResponseWriter, r *http.Request, req *T), opts ...validator.Option) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := Bind[T](r, opts...)
		if err != nil {
			WriteError(w, err)
			return
		}
		fn(w, r, req)
	})
}

// ErrorBody is the JSON body WriteError sends.
type ErrorBody struct {
	Message string            `json:"message"`
	Errors  *validator.Errors `json:"errors,omitempty"`
}

// WriteError writes err as JSON with a status code chosen like Laravel:
//
//   - validation errors: 422 with every field's messages
//   - failed Authorize: 403 "This action is unauthorized."
//   - unreadable body: 400 with the reason
//   - anything else: 500 "Server Error" (the error is logged, not sent)
func WriteError(w http.ResponseWriter, err error) {
	status, body := Response(err)
	if status == http.StatusInternalServerError {
		log.Printf("nethttp: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Response returns the status code and body WriteError would send, for
// handlers that render errors themselves.
func Response(err error) (int, ErrorBody) {
	if bag, ok := validator.ErrorsOf(err); ok {
		return http.StatusUnprocessableEntity, ErrorBody{Message: Summary(bag), Errors: bag}
	}
	if errors.Is(err, validator.ErrUnauthorized) {
		return http.StatusForbidden, ErrorBody{Message: "This action is unauthorized."}
	}
	var de *DecodeError
	if errors.As(err, &de) {
		return http.StatusBadRequest, ErrorBody{Message: de.Error()}
	}
	return http.StatusInternalServerError, ErrorBody{Message: "Server Error"}
}

// Summary is Laravel's top-level message: the first error, plus how many
// more there are.
func Summary(bag *validator.Errors) string {
	fields := bag.Fields()
	if len(fields) == 0 {
		return "The given data was invalid."
	}
	first := bag.First(fields[0])
	switch n := bag.Count() - 1; n {
	case 0:
		return first
	case 1:
		return first + " (and 1 more error)"
	default:
		return fmt.Sprintf("%s (and %d more errors)", first, n)
	}
}
