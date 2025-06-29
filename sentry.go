// Package sentryzapcore contains the zap Core for sending log entries to Sentry
package sentryzapcore

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

// Ensure SentryCore implements zapcore.Core interface
var _ zapcore.Core = (*SentryCore)(nil)

// SentryCore is a zapcore.Core implementation that sends log entries to Sentry.
// It can be used alongside other cores to send logs to multiple destinations.
type SentryCore struct {
	zapcore.LevelEnabler                        // Determines which log levels are enabled
	fields               map[string]interface{} // Additional fields to include with each log entry
	context              context.Context        // Context for Sentry operations, may contain a Sentry span
	stackTrace           bool                   // Whether to include stack traces with error-level logs
}

// NewSentryCore creates a new SentryCore with the provided options.
// By default, it only sends logs at Error level or above to Sentry.
func NewSentryCore(options ...SentryCoreOptions) *SentryCore {
	s := &SentryCore{
		LevelEnabler: zapcore.ErrorLevel,
		fields:       make(map[string]interface{}),
		context:      context.Background(),
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}

// With adds structured context to the Core. It implements zapcore.Core interface.
func (s *SentryCore) With(fields []zapcore.Field) zapcore.Core {
	return s.addFields(fields)
}

// addFields creates a new SentryCore with the given fields added to the existing fields.
// It also extracts any context.Context from the fields to use for Sentry operations.
func (s *SentryCore) addFields(fields []zapcore.Field) *SentryCore {
	// Start with the current context or a background context if none exists
	currentContext := s.context
	if currentContext == nil {
		currentContext = context.Background()
	}

	// Copy existing fields
	m := make(map[string]interface{}, len(s.fields))
	for k, v := range s.fields {
		m[k] = v
	}

	// Add fields to an in-memory encoder
	enc := zapcore.NewMapObjectEncoder()

	for _, f := range fields {
		// Extract context if present
		if v, ok := f.Interface.(context.Context); ok && v != nil {
			currentContext = v
			continue
		}

		// Add non-skip fields to the encoder
		if f.Type != zapcore.SkipType {
			f.AddTo(enc)
		}
	}

	// Merge the encoded fields into our map
	for k, v := range enc.Fields {
		m[k] = v
	}

	// Create a new core with the updated fields and context
	return &SentryCore{
		LevelEnabler: s.LevelEnabler,
		fields:       m,
		context:      currentContext,
		stackTrace:   s.stackTrace,
	}
}

// Check determines whether the supplied Entry should be logged.
// It implements zapcore.Core interface.
func (s *SentryCore) Check(entry zapcore.Entry, checkEntry *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(entry.Level) {
		return checkEntry.AddCore(entry, s)
	}

	return checkEntry
}

// flushSentry flushes any buffered Sentry events with the given timeout
func flushSentry() {
	sentry.Flush(2 * time.Second)
}

// Write takes a log entry and sends it to Sentry asynchronously.
// It implements zapcore.Core interface.
func (s *SentryCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// Create a clone with the additional fields
	clone := s.addFields(fields)

	go func(clone *SentryCore, entry zapcore.Entry) {
		// Extract span from context if present
		span := sentry.SpanFromContext(clone.context)

		// Create a local hub to avoid modifying the global hub
		localHub := sentry.CurrentHub().Clone()

		// Get the Sentry client
		client := localHub.Client()
		if client == nil {
			// No client configured, nothing to do
			return
		}

		// Configure the scope with caller information and span
		localHub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("file", entry.Caller.File)
			scope.SetTag("line", strconv.Itoa(entry.Caller.Line))
			scope.SetSpan(span)
		})

		// Create the Sentry event
		event := &sentry.Event{
			Extra:       clone.fields,
			Fingerprint: []string{entry.Message},
			Level:       sentrySeverity(entry.Level),
			Message:     entry.Message,
			Platform:    "go",
			Timestamp:   entry.Time,
			Logger:      entry.LoggerName,
		}

		// Add exception with stack trace for error-level logs if enabled
		if entry.Level >= zapcore.ErrorLevel && s.stackTrace {
			event.SetException(errors.New(entry.Message), client.Options().MaxErrorDepth)
		}

		// Send the event to Sentry
		client.CaptureEvent(event, nil, localHub.Scope())

		// Optionally flush, but do not block main goroutine
		go flushSentry()
	}(clone, entry)

	// Since this is async, we can't return errors from Sentry
	return nil
}

// Sync flushes any buffered log entries.
// It implements zapcore.Core interface.
func (*SentryCore) Sync() error {
	go flushSentry()
	return nil
}

// sentrySeverity converts a Zap log level to the corresponding Sentry level.
// This ensures that log levels are properly mapped between the two systems.
func sentrySeverity(lvl zapcore.Level) sentry.Level {
	switch lvl {
	case zapcore.DebugLevel:
		return sentry.LevelDebug
	case zapcore.InfoLevel:
		return sentry.LevelInfo
	case zapcore.WarnLevel:
		return sentry.LevelWarning
	case zapcore.ErrorLevel:
		return sentry.LevelError
	case zapcore.DPanicLevel:
		return sentry.LevelFatal
	case zapcore.PanicLevel:
		return sentry.LevelFatal
	case zapcore.FatalLevel:
		return sentry.LevelFatal
	default:
		// Unrecognized levels are treated as fatal to ensure they're noticed
		return sentry.LevelFatal
	}
}
