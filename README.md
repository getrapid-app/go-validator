# go-validator

Laravel-style validation for Go. The same rules work in an HTTP handler, a gRPC service or a plain Go program.

> Status: early development. The core, all of Laravel's rules, and the HTTP and gRPC adapters are in place. See [docs/DESIGN.md](docs/DESIGN.md) for the design and [docs/RULES.md](docs/RULES.md) for every rule.

```go
import "github.com/getrapid-app/go-validator"

// Laravel rule lists
err := validator.Validate(ctx, input, validator.Rules{
    "email":       []string{"required", "email", "max:255"},
    "items.*.qty": "required|integer|min:1", // pipe syntax works too
})

// Fluent API
err = validator.Validate(ctx, input, validator.Rules{
    "email": validator.Chain().Required().Email().Max(255),
})

// Struct tags
type SignUp struct {
    Email string `json:"email" validate:"required|email|max:255"`
}
err = validator.Struct(ctx, &signUp)

if bag, ok := validator.ErrorsOf(err); ok {
    fmt.Println(bag.First("email")) // The email field must be a valid email address.
}
```

`required` treats `0` as empty, because a Go struct can't tell a missing number from zero. Add `accept_zero` when 0 is a valid value: `[]string{"required", "accept_zero", "integer"}`.

Custom rules use the same interface as the built-in ones:

```go
validator.Define("even").
    Message("The :attribute must be an even number.").
    Check(func(ctx context.Context, f validator.Field, params []string) error {
        n, ok := validator.As[int](f.Value)
        if !ok || n%2 != 0 {
            return validator.Fail("even", nil)
        }
        return nil
    })
```

## HTTP

```go
import "github.com/getrapid-app/go-validator/nethttp"

type CreatePost struct {
    Title string   `json:"title" validate:"required|string|max:100"`
    Tags  []string `json:"tags" validate:"array|max:3"`
}

mux.Handle("POST /posts", nethttp.Handle(func(w http.ResponseWriter, r *http.Request, req *CreatePost) {
    // req is decoded and valid
}))
```

`nethttp.Bind[T](r)` reads query parameters and JSON, urlencoded or multipart bodies (files included), and validates the result. `nethttp.WriteError` answers like Laravel: 422 with `{"message": ..., "errors": {...}}`, 403 when `Authorize` fails, and 400 for an unreadable body.

## gRPC

```go
import "github.com/getrapid-app/go-validator/grpcvalidator"

// rules.go, in the same package as the generated code
func (r *CreateUserRequest) Rules() validator.Rules {
    return validator.Rules{"email": []string{"required", "email"}}
}

grpc.NewServer(
    grpc.UnaryInterceptor(grpcvalidator.UnaryServerInterceptor()),
    grpc.StreamInterceptor(grpcvalidator.StreamServerInterceptor()),
)
```

Failures become `InvalidArgument` with an `errdetails.BadRequest` listing each field. The adapter is its own Go module, so the core keeps zero dependencies.

## Plain programs

```go
req, err := validator.Bind[Signup](ctx, input) // input: map[string]any from JSON, a queue message, flags...
```

## License

MIT
