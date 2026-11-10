# Contributing

Thanks for helping. Bug reports, rule fixes and new adapters are all welcome.

## Development

The repository holds three published Go modules: the core (root, with `nethttp`), `grpcvalidator` and `playground`. A fourth, `benchmarks`, compares the core with go-playground/validator and is not published. To work on the published modules together, create a workspace the same way CI does:

```sh
go work init . ./grpcvalidator ./playground
for mod in grpcvalidator playground; do
  core=$(awk '$1 == "github.com/getrapid-app/go-validator" {print $2}' $mod/go.mod)
  go work edit -replace=github.com/getrapid-app/go-validator@"$core"=./
done
```

`go.work` is ignored by git. Before opening a pull request, run:

```sh
gofmt -l .                                  # must print nothing
go vet ./... ./grpcvalidator/... ./playground/...
go test -race ./... ./grpcvalidator/... ./playground/...
(cd benchmarks && GOWORK=off go test ./...)
```

## Guidelines

- Keep the core free of third-party dependencies. Code that needs one belongs in its own module, like `grpcvalidator` and `playground`.
- A new rule needs a constructor, a factory registered under its Laravel name, an English message in `messages_en.go`, a `Chain()` method, a row in `docs/RULES.md`, and table tests in `rules_test.go`.
- When behaviour differs from Laravel because of Go, document why in `docs/DESIGN.md`.
- Add an entry to `CHANGELOG.md` under "Unreleased".

## Releasing

1. Move the "Unreleased" changelog entries under the new version.
2. If the core version changes, update the `github.com/getrapid-app/go-validator` requirement in `grpcvalidator/go.mod` and `playground/go.mod` when they need the new version.
3. After the release commit is merged, tag each module that changed on it: `vX.Y.Z`, `grpcvalidator/vX.Y.Z` and `playground/vX.Y.Z`.
