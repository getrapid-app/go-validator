# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/). Until 1.0.0, minor versions may contain breaking changes.

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

[0.1.0]: https://github.com/getrapid-app/go-validator/releases/tag/v0.1.0
