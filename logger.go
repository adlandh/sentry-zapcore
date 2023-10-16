package sentryzapcore

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func WithSentry(logger *zap.Logger, options ...SentryCoreOptions) *zap.Logger {
	return logger.WithOptions(WithSentryOption(options...))
}

func WithSentryOption(options ...SentryCoreOptions) zap.Option {
	return zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, NewSentryCore(options...))
	})
}
