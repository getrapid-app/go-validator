# go-validator

Laravel-style validation for Go. The same rules work in an HTTP handler, a gRPC service or a plain Go program.

> Status: early development. The core API is in place with a first set of rules; see [docs/DESIGN.md](docs/DESIGN.md) for the design and roadmap.

```go
import "github.com/getrapid-app/go-validator"

// Laravel string syntax
err := validator.Validate(ctx, input, validator.Rules{
    "email":         validator.Parse("required|email|max:255"),
    "items.*.qty":   validator.Parse("required|integer|min:1"),
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

## License

MIT
