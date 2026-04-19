package sentryzapcore

import "go.uber.org/zap/zapcore"

// SentryCoreOptions is a functional option for configuring SentryCore.
type SentryCoreOptions func(*SentryCore)

// WithStackTrace enables inclusion of stack traces in Sentry log entries
// when the log level is Error or above.
func WithStackTrace() SentryCoreOptions {
	return func(s *SentryCore) {
		s.stackTrace = true
	}
}

// WithMinLevel sets the minimum log level for sending entries to Sentry.
// Entries below this level are silently dropped by the Sentry core.
func WithMinLevel(level zapcore.Level) SentryCoreOptions {
	return func(s *SentryCore) {
		s.LevelEnabler = level
	}
}
