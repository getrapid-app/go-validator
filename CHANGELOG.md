# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/). Until 1.0.0, minor versions may contain breaking changes.

## [Unreleased]

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

[0.2.1]: https://github.com/getrapid-app/go-validator/releases/tag/v0.2.1
[0.2.0]: https://github.com/getrapid-app/go-validator/releases/tag/v0.2.0
[0.1.0]: https://github.com/getrapid-app/go-validator/releases/tag/v0.1.0
