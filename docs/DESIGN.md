# go-validator design

Status: released as v0.2.1.

## Goal

Bring Laravel's request validation to Go, written the way Go code is written, and usable anywhere: an HTTP handler, a gRPC service, a CLI or a background worker. The core package has no dependencies and knows nothing about transports.

## Requirements

- Three ways to write rules, with identical behaviour: Laravel strings (`[]string{"required", "email", "max:255"}` or `"required|email|max:255"`), struct tags (`validate:"required|email"`), and a fluent API (`Chain().Required().Email()`).
- Validate structs and `map[string]any`, including nested fields and wildcards (`address.city`, `items.*.price`).
- Port all of Laravel's validation rules.
- One rule interface used by built-in rules and user rules alike, with a fluent way to define custom rules.
- Use generics where they help.
- Ship an opinionated, recommended structure (the request pattern) rather than only loose helpers.
- Open source, Go 1.22 or later.

## Architecture

```
  []string / string ─┐
         struct tags ─┼─► toRules(value, Registry) ─► []Rule ─┐
        fluent Chain ─┘                                      │
                                                             ▼
 struct / map / JSON ─► normalize ─► tree ─► expand(pattern) ─► run each Rule ─► Errors
                                                                   │
                                     custom messages, Translator ──┘
```

Everything converges on two things: a list of `Rule` values per field pattern, and one normalised data tree. That convergence is what makes the three syntaxes, and structs versus maps, behave the same.

### Rule

```go
type Rule interface {
    Name() string
    Validate(ctx context.Context, f Field) error
}
```

- `nil` means the value passes.
- `validator.Fail(key, params)` is a validation failure with a translatable message key (`"max.string"`) and placeholder values (`{"max": 255}`).
- Any other error is also a failure, and its text becomes the message. This keeps one-off custom rules short.
- `validator.Abort(err)` stops the whole validation and returns `err`, for infrastructure failures such as a database being down during `unique`.
- A rule that also implements `ImplicitRule` (`Implicit() bool`) runs even when the field is missing or blank, as `required` and `required_if` do. Every other rule is skipped for missing and blank values, matching Laravel.
- `context.Context` is passed through so rules that do I/O (`exists`, `unique`) can honour cancellation.

`Field` gives a rule the concrete path (`items.2.qty`), the pattern (`items.*.qty`), the value, whether it was present, `Lookup(other)` to read another field (wildcards resolve to the same position), and `HasRule(name)`.

### Rules values

`validator.Rules` is a `map[string]any` so a rule list can be written the Laravel way, without wrappers:

```go
validator.Rules{
    "email":   []string{"required", "email", "max:255"}, // one rule per element
    "name":    "required|string|max:255",                // pipe syntax
    "age":     validator.Chain().Required().Integer(),   // fluent chain
    "role":    validator.Enum(RoleAdmin, RoleUser),       // a single Rule
    "code":    []any{"required", validator.Regex(`^[A-Z]{3}$`)}, // a mix
}
```

Elements of a `[]string` are not split on `|`, so `"regex:/^(a|b)$/"` is safe there. Any other value type is reported as an error naming the field.

### Registry and custom rules

A `Registry` maps rule names to factories (`func(params []string) (Rule, error)`) and default messages. The built-in rules register themselves through the same public API users call, so there is no special path for them.

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

After that, `"required|even"`, `validate:"even"` and `Chain().Rule("even")` all work. `Check` also returns a constructor for direct use in `List(...)`. `DefaultRegistry` is global for convenience; libraries and tests can use `NewRegistry()` with `WithRegistry` to stay isolated.

### Generics

- `RuleFor[T](name, func(ctx, value T, f Field) error)` builds a rule whose function receives a typed value. A value that is not a `T` fails the rule.
- `As[T](v)` converts a value to `T`, including lossless numeric conversion (a JSON `float64` of `3` becomes `int`, `3.5` does not).
- Step 3 adds `Bind[T]` helpers in the adapters to decode and validate a request type in one call.

### Data layer

Input is normalised once into a tree of `map[string]any`, `[]any` and leaf values. Structs use their `json` tag names (falling back to the field name), embedded structs are flattened like `encoding/json`, and `time.Time`, `[]byte` and `encoding.TextMarshaler` types stay as leaf values. Path lookup and wildcard expansion then work on that one tree.

Wildcards expand only where the data has entries: `items.*.name` over three items yields `items.0.name` to `items.2.name`, and over no items yields nothing. A literal segment is kept even when missing, so a missing name inside an existing item is still reported.

### Errors and messages

`Errors` keeps messages per field in insertion order and marshals to Laravel's JSON shape: `{"email": ["The email field is required."]}`. A failed validation returns `*ValidationError`, and `ErrorsOf(err)` gets the bag back out.

Messages resolve in Laravel's order:

1. custom message in code for `field.rule` or `pattern.rule` (`items.*.price.min`)
2. custom message in code for `rule`
3. the translator's `custom.field.rule` or `custom.pattern.rule`
4. the translator's `rule`, searched in the locale (`ar-EG`), its language (`ar`), then the fallback locale (`en`)
5. the registry's default English message

The default translator is `DefaultTranslations`, which ships English and Arabic. Teams add a language or override messages with `AddTranslations(locale, map)` or JSON files (`LoadTranslations(fsys, "lang/*.json")`, nested like Laravel's lang files). English stays in the registry as each rule's default, so `Registry.SetMessage` keeps working; the `en` locale holds only overrides. Display names come from `WithAttributes`, then the translator's `attributes.field`.

The locale is the validator's (`WithLocale`), unless the context carries one (`ContextWithLocale`). `nethttp.Localize` sets it from `Accept-Language`, and `grpcvalidator.WithAcceptLanguage` from `accept-language` metadata.

Placeholders are `:attribute` (custom attribute name, or the path with `_` turned into spaces), `:other`, and the rule's own params (`:min`, `:max`, `:values`, ...).

### Modifiers

`bail` (stop at the first failure for the field), `nullable` (nil skips the other rules) and `sometimes` (only validate when present) are rules too, so they work in all three syntaxes. The engine recognises them and adjusts how the other rules run.

### The request pattern

This is the structure the package recommends, modelled on Laravel's Form Requests. A request is a struct holding the input, with optional hooks:

| Hook | Laravel equivalent | Purpose |
|---|---|---|
| `validate` tags, `Rules() Rules` | `rules()` | the rules |
| `Authorize(ctx) error` | `authorize()` | permission check; failure wraps `ErrUnauthorized` |
| `Prepare(ctx) error` | `prepareForValidation()` | normalise input first |
| `Messages() map[string]string` | `messages()` | custom messages |
| `Attributes() map[string]string` | `attributes()` | display names |
| `After(ctx, *Errors) error` | `after()` | checks across fields |

`validator.ValidateRequest(ctx, &req)` runs them in that order. The adapters in step 3 only decode transport input into the request and map the result to the transport's error format, so a request type is reusable across HTTP, gRPC and plain code.

### Package layout

```
go-validator/            core package `validator`, no dependencies
  rule.go                Rule, ImplicitRule, RuleSet, Failure, Abort, RuleFor, As
  registry.go            Registry, Define, Parse (string syntax)
  fluent.go              Chain
  tags.go                struct tag collection (cached per type)
  data.go                normalisation, lookup, wildcard expansion
  field.go               Field
  messages.go            message resolution, Translator
  translations.go        Translations catalog, locale fallback, context locale
  messages_ar.go         built-in Arabic messages
  errors.go              Errors, ValidationError
  request.go             request pattern
  rules*.go              built-in rules, grouped by kind (step 2)
  nethttp/               net/http adapter (step 3)
  grpcvalidator/         gRPC interceptor, its own go.mod so the core stays dependency-free (step 3)
  lang/                  message translations (step 2 or later)
```

Built-in rules live in the root package so the fluent API reads as `validator.Chain().Required()` and there is no import cycle between rules and the registry.

## Zero values and `accept_zero`

A Go struct cannot tell a missing `"qty"` from `"qty": 0`. So `required` (and the other presence rules) treat the number `0` as empty. Add `accept_zero` when 0 is a real value:

```go
type Elevator struct {
    Floor int `json:"floor" validate:"required|accept_zero|integer"`
}
```

When a presence rule looks at *another* field (`required_with:qty`), 0 in that field counts as a value, as in Laravel, because that field's rules are not known there.

Nil pointers, slices, maps and interfaces in a struct count as missing, not as present-but-null. So `*int` with `sometimes|integer` is the Go way to say "optional", and `required` on a nil slice reports the field as required.

## Adapters

All adapters follow the same path: decode the transport's input, run the request pattern, and map the result to the transport's error format.

- **Core `Bind[T](ctx, input)`** decodes a `map[string]any` into `T` and validates it. Decoding is lenient for form-style input ("42" into an `int`, "on" into a `bool`). A value that cannot be converted becomes a validation error on its field ("must be an integer"). Rules see the decoded struct plus any input keys without a struct field, so `confirmed` and `same:password_confirmation` work without adding those fields to `T`.
- **`nethttp`** (stdlib only, inside the core module): `Input(r)` merges query and body (JSON, urlencoded, multipart with files; `items[0][name]`, `items.0.name` and `tags[]` keys), `Bind[T](r)`, `Handle[T](fn)`, `Validate(r, rules)` for map-only validation, and `WriteError`/`Response` with Laravel's 422/403/400 shapes.
- **`grpcvalidator`** (separate module, depends on grpc): unary and stream interceptors validate messages that implement a request hook (usually `Rules()` in a file next to the generated code). Errors map to `InvalidArgument` with `errdetails.BadRequest`, and `ErrUnauthorized` maps to `PermissionDenied`. It requires the core by version; CI and local development build both modules together through a `go.work` workspace (see CONTRIBUTING.md).

## Rules

All of Laravel's validation rules are ported except `can` (use `Authorize` in the request pattern) and the upload-error rule `uploaded`. See [RULES.md](RULES.md) for the full list with the fluent names. Notes on behaviour that differs because of Go:

- Size rules measure Go numbers by value even without `numeric`. Strings are measured by value only when the field also has `numeric`, `integer` or `decimal`.
- Date rules accept `time.Time` values and strings. `date_format` takes PHP formats (`Y-m-d`) or Go layouts (`2006-01-02`). `after:today`, `tomorrow`, `yesterday` and `now` use the clock set with `WithClock`.
- `regex` accepts PHP-delimited patterns with flags (`/^abc$/i`) as well as Go syntax.
- `exists` and `unique` call a `PresenceVerifier` you provide (`WithPresenceVerifier`), so the package does not depend on a database driver. `current_password` calls a `CurrentPasswordChecker`.
- File rules work on any value implementing `File`; `*multipart.FileHeader` is converted automatically. `mimes` and `image` check the file's content, not its name.
- After a field fails an implicit rule such as `required`, its remaining rules are skipped, as Laravel does.
- Rules that only exist as objects in Laravel are constructors here: `Enum[T]`, `ValidEnum[T]`, `Password()`, `Dimensions()`, `Exists()/Unique()`, `AnyOf`, `When`, `RequiredIfFunc`, `ProhibitedIfFunc`, `ExcludeIfFunc`.

## Decisions

These were open questions in the first draft and are now settled:

1. The package name stays `validator`.
2. Missing versus zero: solved with `accept_zero` (above) rather than validating raw input.
3. Patterns with `|` work in `[]string` rules, the fluent API and `Regex(...)`; only the pipe syntax (strings and struct tags) cannot hold them.
4. Fields in a `Rules` map are validated in sorted path order; struct tags keep declaration order.
