# go-validator

[![CI](https://github.com/getrapid-app/go-validator/actions/workflows/ci.yml/badge.svg)](https://github.com/getrapid-app/go-validator/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/getrapid-app/go-validator.svg)](https://pkg.go.dev/github.com/getrapid-app/go-validator)

Laravel-style validation for Go. Write rules the way you would in Laravel, and use them in an HTTP handler, a gRPC service, a CLI or a queue worker.

- **Every Laravel rule.** That's 100+ rules, from `required_if` and `exclude_unless` to `distinct`, `dimensions` and `unique`, with Laravel's messages in English and Arabic, and your own languages on top. See [docs/RULES.md](docs/RULES.md).
- **Three ways to write rules, one behaviour.** You can use rule lists (`[]string{"required", "email"}`), struct tags (`validate:"required|email"`) or a fluent API (`Chain().Required().Email()`).
- **Structs and maps**, with nested fields and wildcards (`items.*.price`).
- **One rule interface** for built-in and custom rules, with a fluent `Define` for your own.
- **Form Request pattern.** A request type can declare `Rules`, `Authorize`, `Prepare`, `Messages`, `Attributes` and `After`.
- **Transport-agnostic.** The core has no dependencies. `nethttp` uses only the standard library, and `grpcvalidator` is a separate module.
- **Laravel-compatible errors.** HTTP failures come back as `422 {"message": ..., "errors": {...}}`, so existing front ends keep working.

Requires Go 1.26 or later.

## Install

```sh
go get github.com/getrapid-app/go-validator
go get github.com/getrapid-app/go-validator/grpcvalidator # only for gRPC
go get github.com/getrapid-app/go-validator/playground    # only for go-playground rules
```

## Quick start

```go
import "github.com/getrapid-app/go-validator"

err := validator.Validate(ctx, input, validator.Rules{
    "email":         []string{"required", "email", "max:255"},
    "age":           []string{"required", "integer", "min:18"},
    "items.*.qty":   "required|integer|min:1", // pipe syntax works too
    "password":      validator.Chain().Required().Min(8).Confirmed(),
})

if bag, ok := validator.ErrorsOf(err); ok {
    fmt.Println(bag.First("email")) // The email field must be a valid email address.
}
```

### Request types

```go
type CreateOrder struct {
    Email string      `json:"email" validate:"required|email"`
    Items []OrderItem `json:"items" validate:"required|array|min:1"`
    Note  *string     `json:"note" validate:"sometimes|string|max:500"` // nil means "not sent"
}

func (r *CreateOrder) Rules() validator.Rules {
    return validator.Rules{"items.*.sku": []string{"required", "distinct"}}
}

func (r *CreateOrder) Authorize(ctx context.Context) error {
    if !auth.Can(ctx, "orders.create") {
        return errors.New("not allowed")
    }
    return nil
}

req, err := validator.Bind[CreateOrder](ctx, input) // decode + validate
```

### Zero values

A Go struct can't tell a missing number from `0`, so `required` treats `0` as empty. Add `accept_zero` when 0 is a real value: `validate:"required|accept_zero|integer"`. To make a field optional, use a pointer; a nil pointer counts as missing.

### Custom rules

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

// Now usable everywhere: "required|even", validate:"even", Chain().Rule("even")
```

`exists` and `unique` call a `PresenceVerifier` you implement for your database (`validator.WithPresenceVerifier`), so the package doesn't depend on a database driver.

## HTTP

```go
import "github.com/getrapid-app/go-validator/nethttp"

mux.Handle("POST /orders", nethttp.Handle(func(w http.ResponseWriter, r *http.Request, req *CreateOrder) {
    // req is decoded and valid
}))
```

`nethttp.Bind[T](r)` reads query parameters and JSON, urlencoded or multipart bodies, including files. `nethttp.WriteError` answers like Laravel: 422 with every field's messages, 403 when `Authorize` fails, and 400 for an unreadable body.

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

Failures become `InvalidArgument` with an `errdetails.BadRequest` that lists each field. Add `grpcvalidator.WithAcceptLanguage()` to both interceptors to answer in the language of the client's `accept-language` metadata. Use proto3 `optional` for numbers that may be unset.

## go-playground rules

The optional `playground` module adds the validations of [go-playground/validator](https://github.com/go-playground/validator) as rules named `pg.<tag>`, next to the Laravel rules. The prefix keeps them from clashing with Laravel rules of the same name.

```go
import "github.com/getrapid-app/go-validator/playground"

playground.Use() // once, at program start

type Signup struct {
    Phone   string `json:"phone" validate:"required|pg.e164"`
    Country string `json:"country" validate:"required|pg.iso3166_1_alpha2"`
    Code    string `json:"code" validate:"required|pg.len:6"`
    Color   string `json:"color" validate:"pg.oneof:red,green,blue"`
}
```

- **Syntaxes.** They work in rule strings, struct tags and the fluent API (`Chain().Rule("pg.len", "6")`).
- **Parameters.** They use this package's syntax: `pg.len:6` runs go-playground's `len=6`, and `pg.oneof:red,green,blue` runs `oneof=red green blue`.
- **Presence.** As with any rule, a `pg` rule is skipped for a missing or empty field; use `required`, `nullable` or `sometimes` for presence.
- **Messages.** They are keyed `pg.<tag>`, ship in English and Arabic, and are translated or overridden like any other message.
- **Your own go-playground validations.** Pass your own validator with `playground.WithValidate(v)` and name its custom tags with `playground.Tags(...)`.

About 150 tags are available; see [docs/RULES.md](docs/RULES.md#go-playground-rules). Some are left out:
- **Cross-field tags** (`eqfield`, `required_if`, ...) need the whole struct; use the Laravel rules.
- **Presence tags** (`required`, `omitempty`, ...) are already covered by the Laravel rules.
- **File-system tags** (`file`, `dir`, `image`, `mimetype`, `uds_exists`) would read the server's disk from user input.

## Translations

English and Arabic are built in. Pick the language per validator or per call:

```go
v := validator.New(validator.WithLocale("ar"))
err := validator.Validate(validator.ContextWithLocale(ctx, "ar"), input, rules)
```

Add your own language, or override built-in messages with your team's wording or dialect. A missing key falls back from `ar-EG` to `ar`, then to English:

```go
validator.AddTranslations("ar-EG", map[string]string{
    "required":         "لازم تكتب :attribute.",
    "attributes.email": "الإيميل",
})
validator.AddTranslations("en", map[string]string{"required": "Please fill in :attribute."})
```

Or keep translations in JSON files shaped like Laravel's `lang/<locale>/validation.php`, with `attributes` and `custom` sections, and load them at start-up:

```go
//go:embed lang/*.json
var langFiles embed.FS

validator.LoadTranslations(langFiles, "lang/*.json") // lang/fr.json adds "fr"
```

Over HTTP, `nethttp.Localize(mux)` picks each request's language from `Accept-Language`. `nethttp.Handle` and `nethttp.WriteErrorFor` then answer fully in that language, including the 422 summary (`http.more_errors`) and the 403 and 500 texts (`http.unauthorized`, `http.server_error`), which you can override like any message. Custom messages in code (`WithMessages`, keyed `email.required` or `required`) and display names (`WithAttributes`) work like Laravel's `messages()` and `attributes()` and win over translations. To use another i18n library, implement `validator.Translator`.

## Performance

These numbers come from `go test -bench . -benchmem` on a 4 vCPU Linux VM, so treat them as rough. The comparison uses the same input and rules in both libraries; see [benchmarks/](benchmarks/).

| Benchmark | go-validator | go-playground/validator v10 |
|---|---|---|
| Struct, 4 fields + 3 items, valid | 3.3 µs, 14 allocs | 2.8 µs, 25 allocs |
| Struct with 100 items, valid | 69 µs, 309 allocs | 41 µs, 510 allocs |
| Struct, invalid, with messages | 3.6 µs, 27 allocs | 3.9 µs, 65 allocs |
| Map with 4 fields | 1.1 µs, 0 allocs | 1.7 µs, 10 allocs |

go-validator allocates less than go-playground in every case and is faster on maps and on invalid input. It first turns a struct into a tree of maps and lists, which is what lets structs, maps, wildcards and cross-field rules behave the same way. Building those maps is most of the remaining time gap on large structs.

## Docs

- [docs/RULES.md](docs/RULES.md) has every rule with its string and fluent forms.
- [docs/DESIGN.md](docs/DESIGN.md) covers the architecture and the decisions behind it.
- [pkg.go.dev](https://pkg.go.dev/github.com/getrapid-app/go-validator) has the API reference and runnable examples.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
