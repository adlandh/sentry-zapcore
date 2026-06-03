# AGENTS.md

## Commands

- Test exactly like CI: `go test -race -coverprofile=coverage.txt -covermode=atomic ./...`.
- Run a focused test: `go test ./... -run TestSentryZapCore/TestWithErrorLog` for testify suite subtests, or `go test ./... -run TestConversionHelpers` for plain tests.
- Local lint config exists at `.golangci.yml`, but CI replaces it with `https://raw.githubusercontent.com/adlandh/golangci-lint-config/refs/heads/main/.golangci.yml` before running `golangci/golangci-lint-action@v9`. If a lint result matters, check both the repo config and the downloaded CI config.
- Format with `gofmt`/`goimports`; `.golangci.yml` enables both formatters.

## Repository Shape

- This is a small Go library module, not an app: `module github.com/adlandh/sentry-zapcore/v2`, `go 1.25.0`.
- Public integration points are `WithSentry`, `WithSentryOption`, `NewSentryCore`, `WithMinLevel`, and `WithStackTrace` in the root package `sentryzapcore`.
- `example/main.go` is documentation/example code; the linter config excludes `examples$`, but this repo's directory is singular `example`, so do not assume it is excluded locally.

## Implementation Notes

- `SentryCore` is a `zapcore.Core` that tees Zap logs into Sentry's native `sentry.Logger`; default threshold is `zapcore.ErrorLevel`.
- A `zapcore.SkipType` field whose `Interface` is a non-nil `context.Context` is treated specially as Sentry span context and is not emitted as a normal attribute.
- `With(fields)` converts Zap fields into persistent Sentry attributes; `Write(entry, fields)` applies per-entry fields and maps Zap debug/info/warn/error+ levels to Sentry log levels.
- `Sync` calls global `sentry.Flush` with the package `flushTimeout` constant (`2 * time.Second`) and returns `errFlushTimeout` on failure.
- Attribute conversion intentionally stringifies `time.Time`, `time.Duration`, `[]byte`, `error`, `fmt.Stringer`, unknown structs, and uint64 values larger than `math.MaxInt64`.

## Test Gotchas

- Tests use global Sentry state through `sentry.Init`, `sentry.CurrentHub()`, and `sentry.Flush`; avoid adding parallel tests unless you isolate/reset that state.
- The test transport is an in-memory `transportMock`; no real DSN or network service is required.
- Random values come from `gofakeit`; assertions should find logs by message instead of relying on event order.
