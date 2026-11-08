package grpcvalidator_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/getrapid-app/go-validator"
	"github.com/getrapid-app/go-validator/grpcvalidator"
)

// createUser mimics a protoc-gen-go struct: json tags with omitempty and
// unexported bookkeeping fields.
type createUser struct {
	state       struct{}
	Email       string `protobuf:"bytes,1,opt,name=email,proto3" json:"email,omitempty"`
	DisplayName string `protobuf:"bytes,2,opt,name=display_name,json=displayName,proto3" json:"display_name,omitempty"`
	Age         int32  `protobuf:"varint,3,opt,name=age,proto3" json:"age,omitempty"`
}

func (r *createUser) Rules() validator.Rules {
	return validator.Rules{
		"email":        []string{"required", "email"},
		"display_name": []string{"required", "max:20"},
		"age":          []string{"integer", "min:13"},
	}
}

func (r *createUser) Authorize(context.Context) error {
	if r.Email == "banned@example.com" {
		return errors.New("banned")
	}
	return nil
}

type noRules struct{ Email string }

func call(t *testing.T, req any) (bool, error) {
	t.Helper()
	called := false
	_, err := grpcvalidator.UnaryServerInterceptor()(context.Background(), req, &grpc.UnaryServerInfo{FullMethod: "/users/Create"},
		func(context.Context, any) (any, error) { called = true; return "ok", nil })
	return called, err
}

func TestUnaryValid(t *testing.T) {
	called, err := call(t, &createUser{Email: "jane@example.com", DisplayName: "Jane", Age: 30})
	if err != nil || !called {
		t.Fatalf("called=%v err=%v", called, err)
	}
}

func TestUnaryInvalid(t *testing.T) {
	called, err := call(t, &createUser{Email: "nope", Age: 5})
	if called {
		t.Fatal("handler must not run")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code %v", st.Code())
	}
	if st.Message() != "The age field must be at least 13. (and 2 more errors)" {
		t.Errorf("message %q", st.Message())
	}
	var violations []*errdetails.BadRequest_FieldViolation
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			violations = br.FieldViolations
		}
	}
	if len(violations) != 3 || violations[1].Field != "display_name" || violations[1].Description != "The display name field is required." {
		t.Fatalf("violations: %v", violations)
	}
}

func TestUnaryUnauthorized(t *testing.T) {
	_, err := call(t, &createUser{Email: "banned@example.com", DisplayName: "x"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestUnaryWithoutRulesPasses(t *testing.T) {
	called, err := call(t, &noRules{})
	if err != nil || !called {
		t.Fatalf("called=%v err=%v", called, err)
	}
}

type fakeStream struct {
	grpc.ServerStream
	msgs []*createUser
}

func (f *fakeStream) Context() context.Context { return context.Background() }
func (f *fakeStream) RecvMsg(m any) error {
	if len(f.msgs) == 0 {
		return errors.New("EOF")
	}
	*m.(*createUser) = *f.msgs[0]
	f.msgs = f.msgs[1:]
	return nil
}

func TestStream(t *testing.T) {
	ss := &fakeStream{msgs: []*createUser{{Email: "a@example.com", DisplayName: "A", Age: 20}, {Email: "bad", Age: 20}}}
	var errs []error
	err := grpcvalidator.StreamServerInterceptor()(nil, ss, &grpc.StreamServerInfo{}, func(_ any, s grpc.ServerStream) error {
		for i := 0; i < 2; i++ {
			errs = append(errs, s.RecvMsg(&createUser{}))
		}
		return nil
	})
	if err != nil || errs[0] != nil || status.Code(errs[1]) != codes.InvalidArgument {
		t.Fatalf("err=%v errs=%v", err, errs)
	}
}

func TestStatusPassThrough(t *testing.T) {
	orig := status.Error(codes.NotFound, "gone")
	if got := grpcvalidator.Status(orig); got != orig {
		t.Fatalf("got %v", got)
	}
	if got := grpcvalidator.Status(errors.New("db down")); status.Code(got) != codes.Internal {
		t.Fatalf("got %v", got)
	}
}
