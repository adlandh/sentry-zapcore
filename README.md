# Sentry Zap Core

[![Go Reference](https://pkg.go.dev/badge/github.com/adlandh/sentry-zapcore/v2.svg)](https://pkg.go.dev/github.com/adlandh/sentry-zapcore/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/adlandh/sentry-zapcore/v2)](https://goreportcard.com/report/github.com/adlandh/sentry-zapcore/v2)

A Go library that integrates [Zap](https://pkg.go.dev/go.uber.org/zap) structured logging with [Sentry](https://sentry.io) error tracking. This integration allows you to:

- Continue using Zap's powerful structured logging capabilities
- Automatically send error reports to Sentry
- Include structured log fields as context in Sentry events
- Integrate with Sentry's performance monitoring

## Features

- Seamless integration between Zap and Sentry
- Configurable minimum log level for Sentry reporting
- Optional stack trace inclusion
- Support for structured logging fields
- Integration with Sentry's tracing system
- Minimal performance overhead

## Requirements

- Go 1.23 or higher
- [go.uber.org/zap](https://pkg.go.dev/go.uber.org/zap) v1.27.0 or higher
- [github.com/getsentry/sentry-go](https://pkg.go.dev/github.com/getsentry/sentry-go) v0.32.0 or higher

## Installation

```bash
go get github.com/adlandh/sentry-zapcore
```

## Quick Start

```go
package main

import (
    "errors"

    sentryzapcore "github.com/adlandh/sentry-zapcore"
    "github.com/getsentry/sentry-go"
    "go.uber.org/zap"
)

func main() {
    // Initialize Sentry
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         "your-sentry-dsn",
		Environment: "production",
		EnableLogs:  true,
	})
    if err != nil {
        panic(err)
    }

    // Create a Zap logger with Sentry integration
    logger, err := zap.NewProduction(sentryzapcore.WithSentryOption())
    if err != nil {
        panic(err)
    }

    // Log an error that will be sent to Sentry
    logger.Error("Something went wrong",
        zap.String("user_id", "123"),
        zap.Error(errors.New("operation failed")),
    )
}
```

## Usage

### Initializing Sentry

Before using this library, you need to initialize Sentry:

```go
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         "your-sentry-dsn",
		Environment: "production",
		EnableLogs:  true,
		// Other Sentry options...
	})
if err != nil {
    // Handle error
}
```

### Adding Sentry to a Zap Logger

There are two ways to add Sentry integration to your Zap logger:

#### 1. When creating a new logger:

```go
// Create a new production logger with Sentry integration
logger, err := zap.NewProduction(sentryzapcore.WithSentryOption())
if err != nil {
    // Handle error
}

// Or with a development logger
logger, err := zap.NewDevelopment(sentryzapcore.WithSentryOption())
if err != nil {
    // Handle error
}
```

#### 2. Adding to an existing logger:

```go
// Add Sentry to an existing logger
logger = sentryzapcore.WithSentry(logger)
```

### Configuration Options

By default, only logs at Error level or above are sent to Sentry. You can customize this behavior with options:

```go
// Send all logs at Info level or above to Sentry
logger = sentryzapcore.WithSentry(logger, sentryzapcore.WithMinLevel(zapcore.InfoLevel))

// Include stack traces with error logs
logger = sentryzapcore.WithSentry(logger, sentryzapcore.WithStackTrace())

// Combine multiple options
logger = sentryzapcore.WithSentry(logger, 
    sentryzapcore.WithMinLevel(zapcore.WarnLevel),
    sentryzapcore.WithStackTrace(),
)
```

### Structured Logging

All structured fields added to log entries will be included in the Sentry event as additional context:

```go
logger.Error("Something went wrong",
    zap.String("user_id", "123"),
    zap.Int("status_code", 500),
    zap.Error(err),
)
```

### Tracing Integration

You can include Sentry tracing information by passing a context with a Sentry span:

```go
// Start a Sentry transaction
span := sentry.StartSpan(ctx, "operation_name")
defer span.Finish()

// Create a special field with the context
ctxField := zap.Field{
    Key:       "ctx",
    Type:      zapcore.SkipType,
    Interface: span.Context(),
}

// Log with the context
logger.Error("Error during operation", ctxField, zap.Error(err))
```

## Complete Example

See the [example](./example/main.go) for a complete working example.
