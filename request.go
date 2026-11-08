package validator

import (
	"context"
	"errors"
)

// The request pattern is the package's recommended way to organise
// validation, modelled on Laravel's Form Requests. A request is a struct
// that holds the input and describes how to validate it. Only Rules (or
// `validate` tags) are required; every other hook is optional.
//
//	type CreateOrder struct {
//		CustomerID int         `json:"customer_id" validate:"required|integer"`
//		Items      []OrderItem `json:"items" validate:"required|array"`
//	}
//
//	func (r *CreateOrder) Rules() validator.Rules {
//		return validator.Rules{
//			"items.*.qty": validator.Chain().Required().Integer().Min(1),
//		}
//	}
//
//	func (r *CreateOrder) Authorize(ctx context.Context) error { ... }
//
//	err := validator.ValidateRequest(ctx, &order)
//
// Transport adapters decode input into the request and then call
// ValidateRequest, so the same request type works over HTTP, gRPC or in a
// plain program.

// RulesProvider declares rules in code, in addition to struct tags.
type RulesProvider interface {
	Rules() Rules
}

// Authorizer decides whether the caller may make the request. A non-nil
// error stops validation; it is returned wrapped so errors.Is(err,
// ErrUnauthorized) is true.
type Authorizer interface {
	Authorize(ctx context.Context) error
}

// Preparer normalises input before validation, like Laravel's
// prepareForValidation (trim strings, set defaults, ...).
type Preparer interface {
	Prepare(ctx context.Context) error
}

// MessagesProvider supplies custom messages, keyed like WithMessages.
type MessagesProvider interface {
	Messages() map[string]string
}

// AttributesProvider supplies display names, keyed like WithAttributes.
type AttributesProvider interface {
	Attributes() map[string]string
}

// AfterValidator adds checks that need the whole input, after the field
// rules ran. It can add messages to errs.
type AfterValidator interface {
	After(ctx context.Context, errs *Errors) error
}

// ErrUnauthorized is matched by errors returned when Authorize fails.
var ErrUnauthorized = errors.New("validator: unauthorized")

type unauthorizedError struct{ err error }

func (e *unauthorizedError) Error() string   { return "unauthorized: " + e.err.Error() }
func (e *unauthorizedError) Unwrap() []error { return []error{ErrUnauthorized, e.err} }

// ValidateRequest runs the request pattern on req, which must be a pointer
// to a struct: Authorize, Prepare, tag rules plus Rules(), then After.
func (v *Validator) ValidateRequest(ctx context.Context, req any, opts ...Option) error {
	return v.validateRequest(ctx, req, nil, opts)
}

// validateRequest is ValidateRequest with the raw input the request was
// decoded from, if any. Input keys without a struct field stay visible to
// the rules.
func (v *Validator) validateRequest(ctx context.Context, req, input any, opts []Option) error {
	if a, ok := req.(Authorizer); ok {
		if err := a.Authorize(ctx); err != nil {
			return &unauthorizedError{err: err}
		}
	}
	if p, ok := req.(Preparer); ok {
		if err := p.Prepare(ctx); err != nil {
			return err
		}
	}
	vv := v.with(requestOptions(req, opts))
	specs, err := vv.tagSpecs(req)
	if err != nil {
		return err
	}
	if rp, ok := req.(RulesProvider); ok {
		extra, err := vv.compile(rp.Rules())
		if err != nil {
			return err
		}
		specs = append(specs, extra...)
	}
	bag := NewErrors()
	data := normalize(req)
	if input != nil {
		data = overlay(data, input)
	}
	err = vv.run(ctx, data, specs, bag)
	if _, isValidation := ErrorsOf(err); err != nil && !isValidation {
		return err
	}
	if av, ok := req.(AfterValidator); ok {
		if err := av.After(ctx, bag); err != nil {
			return err
		}
	}
	if bag.IsEmpty() {
		return nil
	}
	return &ValidationError{Errors: bag}
}

// ValidateRequest runs the request pattern with the default validator.
func ValidateRequest(ctx context.Context, req any, opts ...Option) error {
	return std.ValidateRequest(ctx, req, opts...)
}
