package sentryzapcore

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// WithSentry wraps the given logger so that error-level (and above) entries
// are also forwarded to Sentry. Use WithMinLevel to lower the threshold.
func WithSentry(logger *zap.Logger, options ...SentryCoreOptions) *zap.Logger {
	return logger.WithOptions(WithSentryOption(options...))
}

// WithSentryOption returns a zap.Option that wraps the core with a SentryCore.
// This is useful when you want to compose the option into a zap.Config
// or a custom logger construction.
func WithSentryOption(options ...SentryCoreOptions) zap.Option {
	return zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, NewSentryCore(context.Background(), options...))
	})
}
