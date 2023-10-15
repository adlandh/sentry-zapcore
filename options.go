package sentry_zapcore

import "go.uber.org/zap/zapcore"

type SentryCoreOptions func(*SentryCore)

// WithStackTrace adds sending stacktrace
func WithStackTrace() SentryCoreOptions {
	return func(s *SentryCore) {
		s.stackTrace = true
	}
}

// WithMinLevel set minimum log level for sending entries to sentry
func WithMinLevel(level zapcore.Level) SentryCoreOptions {
	return func(s *SentryCore) {
		s.LevelEnabler = level
	}
}
