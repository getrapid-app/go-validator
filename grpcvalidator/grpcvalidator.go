// Package grpcvalidator validates incoming gRPC requests with go-validator.
//
// Generated protobuf structs cannot carry `validate` tags, so declare rules
// with the request pattern in a separate file of the same package:
//
//	func (r *CreateUserRequest) Rules() validator.Rules {
//		return validator.Rules{
//			"email": []string{"required", "email"},
//			"name":  []string{"required", "max:255"},
//		}
//	}
//
// Field names are the json names protoc-gen-go generates, which are the
// proto field names ("display_name"). Install the interceptors on the
// server:
//
//	grpc.NewServer(
//		grpc.UnaryInterceptor(grpcvalidator.UnaryServerInterceptor()),
//		grpc.StreamInterceptor(grpcvalidator.StreamServerInterceptor()),
//	)
//
// proto3 sends unset numbers as 0, and 0 is checked by every rule except
// the presence rules. Declare optional numeric fields with the proto3
// "optional" keyword, which generates pointer fields that count as missing
// when unset, so "sometimes|integer|min:13" skips them.
//
// Failures become codes.InvalidArgument with an errdetails.BadRequest that
// lists every field violation; a failed Authorize becomes
// codes.PermissionDenied.
package grpcvalidator

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/getrapid-app/go-validator"
)

type config struct {
	v    *validator.Validator
	opts []validator.Option
}

// Option configures the interceptors.
type Option func(*config)

// WithValidator uses v, for example one with a PresenceVerifier or a
// Translator.
func WithValidator(v *validator.Validator) Option { return func(c *config) { c.v = v } }

// WithOptions passes validator options to every validation.
func WithOptions(opts ...validator.Option) Option {
	return func(c *config) { c.opts = append(c.opts, opts...) }
}

func newConfig(opts []Option) *config {
	c := &config{v: validator.New()}
	for _, o := range opts {
		o(c)
	}
	return c
}

// UnaryServerInterceptor validates each request message that declares
// rules or request hooks before the handler runs.
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	c := newConfig(opts)
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := c.validate(ctx, req); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamServerInterceptor validates every message received on a stream.
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor {
	c := newConfig(opts)
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, &stream{ServerStream: ss, c: c})
	}
}

type stream struct {
	grpc.ServerStream
	c *config
}

func (s *stream) RecvMsg(m any) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	return s.c.validate(s.Context(), m)
}

// validate runs the request pattern on messages that opt in by
// implementing at least one hook.
func (c *config) validate(ctx context.Context, req any) error {
	switch req.(type) {
	case validator.RulesProvider, validator.Authorizer, validator.Preparer, validator.AfterValidator:
	default:
		return nil
	}
	return Status(c.v.ValidateRequest(ctx, req, c.opts...))
}

// Status converts a validation result to a gRPC status error. It returns
// nil for nil, and leaves errors that already carry a status unchanged.
func Status(err error) error {
	if err == nil {
		return nil
	}
	if bag, ok := validator.ErrorsOf(err); ok {
		br := &errdetails.BadRequest{}
		for _, field := range bag.Fields() {
			for _, msg := range bag.Get(field) {
				br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{Field: field, Description: msg})
			}
		}
		st := status.New(codes.InvalidArgument, summary(bag))
		if withDetails, derr := st.WithDetails(br); derr == nil {
			st = withDetails
		}
		return st.Err()
	}
	if errors.Is(err, validator.ErrUnauthorized) {
		return status.Error(codes.PermissionDenied, "This action is unauthorized.")
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	return status.Error(codes.Internal, "validation could not complete")
}

func summary(bag *validator.Errors) string {
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
