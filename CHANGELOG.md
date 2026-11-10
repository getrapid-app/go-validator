# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/). Until 1.0.0, minor versions may contain breaking changes.

## [Unreleased]

### Added

- **Validation stops after `validator.DefaultMaxErrors` (100) failed fields** and returns the errors so far, so a request with 100,000 bad items costs 23 ms instead of 220 ms and 100 MB. Change it at program start, or per validator or call with `validator.WithMaxErrors(n)` (negative for no limit). `validator.StopOnFirstFailure()` stops at the first failing field, like Laravel's `$stopOnFirstFailure`, and request types can return true from a `StopOnFirstFailure() bool` method for the same effect. Decode errors from `Bind` count toward the limit.

## [0.4.0] - 2026-11-10

Released with `grpcvalidator/v0.2.2` (the grpc upgrade below) and the first `playground/v0.1.0`.

### Added

- A security lab ([docs/SECURITY_LAB.md](docs/SECURITY_LAB.md)): fuzz targets for every rule, rule strings, field patterns, `Bind`, `Accept-Language`, translation catalogs, `nethttp.Input` and the go-playground tags; attack probes behind the `seclab` build tag; and a Security workflow that runs `govulncheck`, `staticcheck` and the fuzzers on every pull request and weekly. [SECURITY.md](SECURITY.md) explains how to report a vulnerability.
- **A default size limit for every file:** `validator.DefaultMaxFileSize` (10 MB), changed at program start or per validator with `validator.WithMaxFileSize`. A field's own `max`, `size`, `between`, `lt` or `lte` replaces it, higher or lower (`"video": "file|max:51200"` allows 50 MB). Oversized files fail with the `max.file` message before their content is read. Files over 10 MB in fields without a size rule now fail; raise the default or set `max` on those fields.
- `nethttp.MaxMultipartBytes` (32 MB) and `nethttp.MaxKeyDepth` (32).
- The `playground` module, which adds go-playground/validator's single-value validations as rules named `pg.<tag>` (about 150, such as `pg.e164`, `pg.iso3166_1_alpha2` and `pg.len:6`). They work in rule strings, struct tags and the fluent API, with English and Arabic messages that are translated and overridden like any other. `playground.WithValidate` and `playground.Tags` add custom go-playground validations.

### Security

- **`distinct` no longer takes quadratic time.** It compared every item with every other one, so a 300 KB request with 20,000 items took over two minutes and 100 GB of allocations. It now counts values once per call: 35 ms.
- **`numeric`, `decimal`, `between` and the other number rules no longer accept `NaN`, `Inf`, hex floats (`0x1p4`) or digit separators (`1_000`).** `"NaN"` used to pass `numeric|between:1,100`. Strings must now look like a decimal number, as in Laravel, and `float64` NaN and ±Inf values fail too.
- **`nethttp` caps multipart bodies** at `MaxMultipartBytes` (32 MB by default; there was no limit), so uploads cannot fill the disk. Raise it for handlers that accept larger files.
- **`grpcvalidator` requires google.golang.org/grpc v1.83.1** (was v1.67.1), which fixes GO-2026-6348, heap exhaustion through fragmented HTTP/2 DATA frames. `golang.org/x/net`, `protobuf` and `genproto` are updated with it. Found by the new `govulncheck` job.
- **`nethttp` rejects form and query keys nested deeper than `MaxKeyDepth`** (32) with 400 Bad Request. A 2 MB key such as `a.a.a...` used to allocate over 400 MB.

## [0.3.0] - 2026-11-10

### Changed

- **Go 1.26 or later is now required** (was 1.22), for the core module and `grpcvalidator`. Go 1.26 and 1.27 are the releases the Go team currently supports. Benchmarks showed no speed difference from the `go` line itself; speed depends on the toolchain an application builds with.
- Internals use newer standard library APIs (`reflect.TypeFor`, reflect field iterators, `errors.AsType`, `slices`, `maps`, `strings.SplitSeq`) and benchmarks use `testing.B.Loop`. No behaviour change.
- CI tests on Go 1.26 and the latest stable release, and now also runs the go-playground comparison module (`benchmarks/`), which needed Go 1.26 and so could not run on the old 1.22 job.

## [0.2.3] - 2026-11-09

### Changed

- Faster validation with fewer allocations again, with no change in behaviour or API. On the package benchmarks: structs 3.9 µs → 3.0 µs (31 → 14 allocations), 100 wildcard items 71 µs → 59 µs (521 → 309), maps 2.2 µs → 1.1 µs (17 → 0), failures with messages 4.5 µs → 3.7 µs (48 → 26).
  - Plain `local@domain` email addresses are checked without `net/mail`; anything else still goes through it, and a fuzz test checks both agree.
  - A struct reached through a pointer or a slice is copied once before its fields are read, instead of each string or number field being copied to the heap.
  - Rules given as `[]string` are cached by hash instead of by a joined key built on every call.
  - Validate keeps its compiled rules, wildcard matches and error bag on the stack, and the registry version is read atomically instead of under a lock.
  - An error bag stores the first message of each field in one shared slice.
  - Built-in check rules call their test directly instead of through a closure.

## [0.2.2] - 2026-11-09

### Changed

- Faster validation with far fewer allocations, with no change in behaviour. On the package benchmarks: structs 13 µs → 4 µs (156 → 31 allocations), 100 wildcard items 290 µs → 75 µs (3,469 → 521), maps 5.2 µs → 2.2 µs (65 → 17), failures with messages 22 µs → 4.6 µs (204 → 48).
  - Rules written as strings are parsed once per registry version and cached (bounded).
  - Per-type struct information is cached; plain values skip reflection lookups; already-normalised input (such as decoded JSON) is no longer copied.
  - Paths are looked up without splitting, wildcard paths are built in one reused buffer, and paths for `list.*.field` patterns are shared between validations.
  - Messages are filled in one pass instead of with a `strings.Replacer`, and size rules build their placeholders only on failure.
  - `As` converts built-in numbers without reflection.

### Added

- `benchmarks/`, a separate module comparing go-validator with go-playground/validator v10.

## [0.2.1] - 2026-11-09

### Added

- `MatchLocale` to match an Accept-Language value against supported locales.
- `Validator.With` returns a copy of a validator with extra options.
- `grpcvalidator.WithAcceptLanguage` picks the locale from `accept-language` metadata. The InvalidArgument summary and the PermissionDenied text are translated through the `http.*` keys; `StatusFor` converts an error in a given locale.

### Changed

- `grpcvalidator` requires core v0.2.1. The v0.2.0 core tag was made before these changes, so there is no `grpcvalidator/v0.2.0`.

## [0.2.0] - 2026-11-09

### Added

- Built-in Arabic messages for every rule, next to English.
- `Translations`, the default translator: add a language or override messages per locale with `AddTranslations`, or load Laravel-style JSON files with `LoadTranslations`. Lookups fall back from `ar-EG` to `ar` to the fallback locale (`en`).
- `attributes.<field>` and `custom.<field>.<rule>` translation keys for display names and per-field messages.
- `nethttp.WriteErrorFor`, `ResponseFor` and `SummaryFor` answer in the request's language: the 422 summary suffix ("(and 2 more errors)"), the 403 and the 500 texts come from the `http.*` translation keys, which teams can override. `Handle` uses them.
- `Validator.Translate` / `validator.Translate` for texts that are not about one field, and `Validator.Summary` for the 422 top-level message.
- `ContextWithLocale` to choose the language per call, and `nethttp.Localize` / `nethttp.PreferredLocale` to pick it from `Accept-Language`.

### Changed

- Validators use `DefaultTranslations` unless `WithTranslator` is given.
- A message set in code for a bare rule name (`WithMessages{"short": ...}`) now also replaces the text of rules that return plain errors.

## [0.1.0] - 2026-11-08

First release.

### Added

- Core validator with one `Rule` interface for built-in and custom rules, a `Registry`, and fluent custom rules via `Define`.
- Three rule syntaxes that compile to the same rules: rule lists and pipe strings, `validate` struct tags, and the fluent `Chain()` API.
- Struct and map input with nested paths and `*` wildcards.
- Every Laravel validation rule except `can` and `uploaded`, with Laravel's English messages, plus `accept_zero`.
- Request pattern: `Rules`, `Authorize`, `Prepare`, `Messages`, `Attributes` and `After` hooks, run by `ValidateRequest`.
- `Bind[T]` to decode JSON-shaped input into a request type and validate it.
- Ordered `Errors` bag with Laravel's JSON shape, translations, custom messages and attribute names.
- `nethttp` adapter: request input decoding (JSON, forms, multipart files, query), `Bind`, `Handle`, and Laravel-shaped 422/403/400 responses.
- `grpcvalidator` module: unary and stream interceptors returning `InvalidArgument` with field violations.

[0.4.0]: https://github.com/getrapid-app/go-validator/releases/tag/v0.4.0
[0.3.0]: https://github.com/getrapid-app/go-validator/releases/tag/v0.3.0
[0.2.3]: https://github.com/getrapid-app/go-validator/releases/tag/v0.2.3
[0.2.2]: https://github.com/getrapid-app/go-validator/releases/tag/v0.2.2
[0.2.1]: https://github.com/getrapid-app/go-validator/releases/tag/v0.2.1
[0.2.0]: https://github.com/getrapid-app/go-validator/releases/tag/v0.2.0
[0.1.0]: https://github.com/getrapid-app/go-validator/releases/tag/v0.1.0
