# Benchmarks against go-playground/validator

This is a separate module so the main module keeps no dependencies. It
validates the same order (4 fields plus a list of items) with
go-validator's Laravel tags and with go-playground/validator v10 tags.

```sh
cd benchmarks
go test -bench . -benchmem
```

go-playground/validator v10.30 needs Go 1.26 or later, so this module
does too; the main module still supports Go 1.22.

`BenchmarkStructInvalid` includes turning each failure into a message
for both libraries, since that is what an API returns.
