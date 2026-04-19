// Package sentryzapcore provides a zapcore.Core implementation that sends
// log entries to Sentry using the native sentry.Logger.
package sentryzapcore

import (
	"context"
	"errors"
	"runtime/debug"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
	"go.uber.org/zap/zapcore"
)

// errFlushTimeout is returned by Sync when Sentry does not finish delivering
// buffered events within flushTimeout.
var errFlushTimeout = errors.New("sentryzapcore: flush timed out")

// Ensure SentryCore implements zapcore.Core interface.
var _ zapcore.Core = (*SentryCore)(nil)

// SentryCore is a zapcore.Core implementation that sends log entries to Sentry.
// It can be used alongside other cores to send logs to multiple destinations.
type SentryCore struct {
	zapcore.LevelEnabler // determines which log levels are enabled
	logger               sentry.Logger
	attributes           []attribute.Builder
	stackTrace           bool // include stack traces for error-level logs
}

// NewSentryCore creates a new SentryCore with the provided options.
// By default, it only sends logs at Error level or above to Sentry.
func NewSentryCore(ctx context.Context, options ...SentryCoreOptions) *SentryCore {
	if ctx == nil {
		ctx = context.Background()
	}

	logger := sentry.NewLogger(ctx)

	s := &SentryCore{
		LevelEnabler: zapcore.ErrorLevel,
		logger:       logger,
	}

	for _, opt := range options {
		opt(s)
	}

	if len(s.attributes) > 0 {
		s.logger.SetAttributes(s.attributes...)
	}

	return s
}

// With adds structured context as additional attributes on the Core.
// It implements the zapcore.Core interface.
func (s *SentryCore) With(fields []zapcore.Field) zapcore.Core {
	ctx := s.logger.GetCtx()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := append([]attribute.Builder(nil), s.attributes...)

	fieldCtx, values := encodeFields(fields)
	if fieldCtx != nil {
		ctx = fieldCtx
	}

	attrs = append(attrs, attributesFromValues(values)...)

	logger := sentry.NewLogger(ctx)
	if len(attrs) > 0 {
		logger.SetAttributes(attrs...)
	}

	return &SentryCore{
		LevelEnabler: s.LevelEnabler,
		logger:       logger,
		attributes:   attrs,
		stackTrace:   s.stackTrace,
	}
}

// Check determines whether the supplied Entry should be logged.
// It implements the zapcore.Core interface.
func (s *SentryCore) Check(entry zapcore.Entry, checkEntry *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(entry.Level) {
		return checkEntry.AddCore(entry, s)
	}

	return checkEntry
}

// Write takes a log entry and sends it to Sentry as a structured log.
// It implements the zapcore.Core interface.
func (s *SentryCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	logEntry := logEntryForLevel(s.logger, entry.Level)

	ctx, values := encodeFields(fields)
	if ctx != nil {
		logEntry = logEntry.WithCtx(ctx)
	}

	for k, v := range values {
		logEntry = applyValueToLogEntry(logEntry, k, v)
	}

	if entry.LoggerName != "" {
		logEntry = logEntry.String("logger", entry.LoggerName)
	}

	if entry.Caller.Defined {
		logEntry = logEntry.String("caller.file", entry.Caller.File)
		logEntry = logEntry.Int("caller.line", entry.Caller.Line)
	}

	if s.stackTrace && entry.Level >= zapcore.ErrorLevel {
		stack := entry.Stack
		if stack == "" {
			stack = string(debug.Stack())
		}

		logEntry = logEntry.String("stacktrace", stack)
	}

	logEntry.Emit(entry.Message)

	return nil
}

// flushTimeout is the maximum time Sync waits for buffered Sentry events
// to be delivered before returning.
const flushTimeout = 2 * time.Second

// Sync flushes any buffered log entries to Sentry, blocking up to
// flushTimeout. It returns an error if the flush times out.
// It implements the zapcore.Core interface.
func (*SentryCore) Sync() error {
	if !sentry.Flush(flushTimeout) {
		return errFlushTimeout
	}

	return nil
}

// logEntryForLevel returns a sentry.LogEntry for the given zap log level.
// Debug/Info/Warn map to their sentry counterparts;
// Error, DPanic, Panic, and Fatal all map to Error (sentry logs do not
// have separate panic/fatal channels).
func logEntryForLevel(logger sentry.Logger, level zapcore.Level) sentry.LogEntry {
	switch level {
	case zapcore.DebugLevel:
		return logger.Debug()
	case zapcore.InfoLevel:
		return logger.Info()
	case zapcore.WarnLevel:
		return logger.Warn()
	default:
		return logger.Error()
	}
}

// encodeFields iterates zap fields, extracts a context.Context if present
// (SkipType fields), and collects the rest into a flat map via a
// zapcore.MapObjectEncoder.
func encodeFields(fields []zapcore.Field) (context.Context, map[string]interface{}) {
	var ctx context.Context

	enc := zapcore.NewMapObjectEncoder()

	for _, f := range fields {
		if f.Type == zapcore.SkipType {
			if v, ok := f.Interface.(context.Context); ok && v != nil {
				ctx = v
			}

			continue
		}

		f.AddTo(enc)
	}

	return ctx, enc.Fields
}
