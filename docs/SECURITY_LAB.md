# Security lab

This is how go-validator is tested against hostile input, and what the first run on 2026-11-10 found. The reporting policy is in [SECURITY.md](../SECURITY.md).

## Threat model

Users of an application control the **input**: JSON, form and multipart bodies, query strings, the `Accept-Language` header, and gRPC messages. They do not control the **rules**: rule strings, struct tags, custom rules and translation catalogs are written by the application. So the lab looks for input that:

1. passes a rule it should fail (a validation bypass);
2. panics, hangs, or costs far more time or memory than its size (denial of service);
3. makes an error response reveal server internals.

## What runs

| Layer | Tool | Where |
|---|---|---|
| Known vulnerabilities in Go and dependencies | `govulncheck` | `security.yml`, on every push, pull request and weekly |
| Static analysis | `staticcheck`, plus a one-off `gosec` review | `security.yml` |
| Panics on arbitrary input | 8 fuzz targets (below) | `security.yml`: 30 s each per pull request, 10 min each weekly |
| Fixed bugs stay fixed | `security_test.go`, `nethttp/security_test.go` | the normal test run |
| Cost of hostile input | attack probes, `seclab` build tag | by hand |

Fuzz targets:

- `FuzzRuleValue`: every built-in rule, with fuzzed parameters, over a fuzzed value given as a string, a number, a list and a map.
- `FuzzRuleString`: arbitrary rule strings.
- `FuzzPattern`: arbitrary field patterns (wildcards, relative references) over arbitrary JSON.
- `FuzzBind`: arbitrary JSON decoded into a struct with every kind of field.
- `FuzzMatchLocale`: arbitrary `Accept-Language` headers; the answer must be a supported locale.
- `FuzzTranslations`: arbitrary JSON catalogs and placeholder values.
- `FuzzInput` (`nethttp`): arbitrary bodies, content types and query strings.
- `FuzzTags` (`playground`): every go-playground tag with a fuzzed parameter and value.

Run the lab locally:

```sh
# fuzz one target (add -fuzztime to stop early)
go test -run '^$' -fuzz '^FuzzRuleValue$' -fuzztime 5m .
go test -run '^$' -fuzz '^FuzzInput$' -fuzztime 5m ./nethttp

# attack probes: time and memory for hostile input
go test -tags seclab -run Probe -v . ./nethttp

# scanners
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
go install honnef.co/go/tools/cmd/staticcheck@v0.8.2
govulncheck ./... ./grpcvalidator/... ./playground/...
staticcheck ./... ./grpcvalidator/... ./playground/...
```

A fuzz crash is saved under `testdata/fuzz/<Target>/`. Commit that file with the fix, so the input is replayed by every later `go test`.

## Findings from the first run

Fuzzing ran about 30 million inputs across the 8 targets with no panics. The attack probes and code review found the following.

| # | Severity | Finding | Status |
|---|---|---|---|
| 1 | High | `distinct` compared every item with every other one and built a new path list for each. 20,000 items (a 300 KB JSON body) took 2 min 11 s and allocated 103 GB. | Fixed: sibling values are counted once per Validate call. 20,000 items now take 35 ms and 15 MB. |
| 2 | High | `numeric`, `decimal` and the size rules parsed strings with `strconv.ParseFloat`, which accepts `NaN`, `Inf`, `0x1p4` and `1_000`. `"NaN"` passed `numeric\|between:1,100`, because every comparison with NaN is false. A Go `float64` NaN passed too. | Fixed: strings must look like a decimal number, as PHP's `is_numeric` requires, and NaN and ±Inf are never numbers. |
| 3 | Medium | `nethttp` multipart bodies had no size cap (`MaxBodyBytes` only covered JSON and urlencoded), so uploads could fill the disk with temporary files. | Fixed: new `nethttp.MaxMultipartBytes`, 32 MB by default, for the whole body. Each file is also limited to `validator.DefaultMaxFileSize` (10 MB) unless its field's `max` rule says otherwise. |
| 4 | Medium | Every segment of a form or query key (`a[b][c]` or `a.b.c`) became a nested map, with no depth limit. A 2 MB key allocated 424 MB. JSON was already safe, since `encoding/json` stops at depth 10,000. | Fixed: new `nethttp.MaxKeyDepth`, 32 by default; deeper keys get 400 Bad Request. |
| 5 | Info | `Bind` fills every struct field the input names, validated or not, as `json.Unmarshal` does. A field such as `IsAdmin bool \`json:"is_admin"\`` is set if a client sends it. | Documented: give server-owned fields `json:"-"`. |
| 6 | Info | Every failing field gets its own messages, so 100,000 empty items with two required fields produce 200,000 errors (100 MB, 0.2 s). The cost is linear in the request size. | Open: cap list sizes with `max` on the list, or a possible `MaxErrors` option later. |
| 7 | Info | `gosec` warnings: integer conversions in `exactInt64` (each is checked by converting back), "hardcoded credentials" in message catalogs (the word *password*), and unchecked `maphash` writes (they never fail). | False positives, no change. |

Checked and found safe:

- Regular expressions use Go's RE2 engine, so `regex` cannot backtrack: a 10 MB string against `^(a+)+$` takes 0.4 s.
- Every built-in rule handles a 10 MB string in under 0.5 s. `date` allocates about 37 times the input, which `MaxBodyBytes` bounds.
- `multiple_of` uses `math/big`, which rejects huge exponents such as `1e10000000` immediately.
- Unsigned integers above `MaxInt64` fail `max` and `between` correctly.
- A 1 MB `Accept-Language` header is matched in 24 ms.
- JSON nesting is limited by `encoding/json`; gRPC nesting by protobuf.
- 500 responses (HTTP) and `codes.Internal` (gRPC) never include the underlying error.
- go-playground panics on some types; the `playground` adapter turns each panic into a failed rule.

`govulncheck` could not reach the vulnerability database from the lab machine, so its first run is the CI job.
