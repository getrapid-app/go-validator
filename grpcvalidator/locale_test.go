package grpcvalidator_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/getrapid-app/go-validator"
	"github.com/getrapid-app/go-validator/grpcvalidator"
)

func callCtx(ctx context.Context, req any, opts ...grpcvalidator.Option) error {
	_, err := grpcvalidator.UnaryServerInterceptor(opts...)(ctx, req, &grpc.UnaryServerInfo{FullMethod: "/users/Create"},
		func(context.Context, any) (any, error) { return "ok", nil })
	return err
}

func TestAcceptLanguageMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("accept-language", "ar-EG,ar;q=0.9"))

	err := callCtx(ctx, &createUser{Email: "nope", Age: 5}, grpcvalidator.WithAcceptLanguage())
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument || st.Message() != "يجب أن تكون قيمة حقل age 13 على الأقل. (و 2 أخطاء أخرى)" {
		t.Errorf("422: %v %q", st.Code(), st.Message())
	}

	err = callCtx(ctx, &createUser{Email: "banned@example.com", DisplayName: "x", Age: 20}, grpcvalidator.WithAcceptLanguage())
	if st, _ := status.FromError(err); st.Code() != codes.PermissionDenied || st.Message() != "غير مصرح لك بتنفيذ هذا الإجراء." {
		t.Errorf("403: %v %q", st.Code(), st.Message())
	}

	// Without the option the metadata is ignored.
	err = callCtx(ctx, &createUser{Email: "banned@example.com", DisplayName: "x", Age: 20})
	if st, _ := status.FromError(err); st.Message() != "This action is unauthorized." {
		t.Errorf("default: %q", st.Message())
	}

	// A locale already on the context wins over the metadata.
	err = callCtx(validator.ContextWithLocale(ctx, "en"), &createUser{Email: "banned@example.com", DisplayName: "x", Age: 20},
		grpcvalidator.WithAcceptLanguage())
	if st, _ := status.FromError(err); st.Message() != "This action is unauthorized." {
		t.Errorf("context locale: %q", st.Message())
	}
}

func TestStatusForOverride(t *testing.T) {
	tr := validator.NewTranslations().Add("en", map[string]string{"http.unauthorized": "Nope."})
	err := callCtx(context.Background(), &createUser{Email: "banned@example.com", DisplayName: "x", Age: 20},
		grpcvalidator.WithOptions(validator.WithTranslator(tr, "en")))
	if st, _ := status.FromError(err); st.Message() != "Nope." {
		t.Errorf("got %q", st.Message())
	}
}
