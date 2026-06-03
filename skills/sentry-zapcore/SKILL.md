---
name: sentry-zapcore
description: >-
  Use github.com/adlandh/sentry-zapcore/v2 to tee Zap structured logs into
  Sentry. Apply when adding Sentry error/log reporting to a Go zap.Logger,
  forwarding zap fields as Sentry attributes, attaching Sentry span/trace
  context to logs, or configuring which zap levels reach Sentry.
---

# sentry-zapcore

`github.com/adlandh/sentry-zapcore/v2` provides a `zapcore.Core` that tees Zap
log entries into Sentry via Sentry's native `sentry.Logger`. You keep using Zap
normally; error-level entries (by default) also land in Sentry with structured
fields as attributes.

Public API (package `sentryzapcore`): `WithSentry`, `WithSentryOption`,
`NewSentryCore`, `WithMinLevel`, `WithStackTrace`.

## Install & import

```bash
go get github.com/adlandh/sentry-zapcore/v2
```

```go
import (
    sentryzapcore "github.com/adlandh/sentry-zapcore/v2"
    "github.com/getsentry/sentry-go"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)
```

Requires Go 1.25+, `go.uber.org/zap`, and `github.com/getsentry/sentry-go`.

## Prerequisite: init Sentry first

`sentry.Init` MUST run before wrapping the logger, and `EnableLogs: true` is
required — without it, entries never reach Sentry.

```go
err := sentry.Init(sentry.ClientOptions{
    Dsn:         "your-sentry-dsn",
    Environment: "production",
    EnableLogs:  true,
})
if err != nil {
    panic(err)
}
```

## Attach Sentry to a Zap logger

Two paths. Pick one.

**New logger** — pass `WithSentryOption()` as a `zap.Option` to any constructor:

```go
logger, err := zap.NewProduction(sentryzapcore.WithSentryOption())
if err != nil {
    panic(err)
}
defer func() { _ = logger.Sync() }()
```

**Existing logger** — wrap it and reassign:

```go
logger = sentryzapcore.WithSentry(logger)
defer func() { _ = logger.Sync() }()
```

Both accept the same `SentryCoreOptions` variadic.

## Configuration options

Default threshold: only `zapcore.ErrorLevel` and above go to Sentry.

```go
// Lower threshold (e.g. Info and above)
logger = sentryzapcore.WithSentry(logger, sentryzapcore.WithMinLevel(zapcore.InfoLevel))

// Include stack traces for error-level entries
logger = sentryzapcore.WithSentry(logger, sentryzapcore.WithStackTrace())

// Combine
logger = sentryzapcore.WithSentry(logger,
    sentryzapcore.WithMinLevel(zapcore.WarnLevel),
    sentryzapcore.WithStackTrace(),
)
```

- `WithMinLevel(level)` — minimum level forwarded to Sentry; below it is dropped.
- `WithStackTrace()` — adds a `stacktrace` attribute for entries at Error+.

## Structured fields → Sentry attributes

Zap fields on an entry become Sentry attributes automatically:

```go
logger.Error("Something went wrong",
    zap.String("user_id", "123"),
    zap.Int("status_code", 500),
    zap.Error(err),
)
```

Conversion notes: `time.Time` → RFC3339Nano string, `time.Duration` → string,
`[]byte` → string, `error`/`fmt.Stringer` → its string, `uint64` values above
`math.MaxInt64` → string; unknown types → `fmt.Sprint`.

## Tracing: attach Sentry span context

Pass span context as a special `zapcore.SkipType` field whose `Interface` holds
a non-nil `context.Context`. It is consumed as span context, not emitted as a
normal attribute.

```go
span := sentry.StartSpan(ctx, "operation_name")
defer span.Finish()

ctxField := zap.Field{
    Key:       "ctx",
    Type:      zapcore.SkipType,
    Interface: span.Context(),
}

logger.Error("Error during operation", ctxField, zap.Error(err))
```

## Flush before exit

`Sync()` flushes buffered Sentry events via `sentry.Flush` with a 2-second
timeout; on timeout it returns an error. Always defer it before process exit:

```go
defer func() { _ = logger.Sync() }()
```

## Gotchas

- **Levels above Error collapse**: Error, DPanic, Panic, Fatal all map to Sentry
  `Error`. Sentry logs have no separate panic/fatal channel.
- **Global Sentry state**: `sentry.Init`, `CurrentHub()`, and `sentry.Flush`
  are process-global. Init once; Sync flushes the whole process. Avoid parallel
  tests unless you isolate/reset Sentry state.
- **`EnableLogs: true` is mandatory** — easy to forget; logs silently go nowhere
  without it.
- **`example` dir is singular** in this repo (not `examples`); the lint config's
  `examples$` exclude does not match it locally.
