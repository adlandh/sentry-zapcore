// Package sentryzapcore contains the zap Core for sending log entries to Sentry
package sentryzapcore

import (
	"context"
	"fmt"
	"math"
	"runtime/debug"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
	"go.uber.org/zap/zapcore"
)

// Ensure SentryCore implements zapcore.Core interface
var _ zapcore.Core = (*SentryCore)(nil)

// SentryCore is a zapcore.Core implementation that sends log entries to Sentry.
// It can be used alongside other cores to send logs to multiple destinations.
type SentryCore struct {
	zapcore.LevelEnabler // Determines which log levels are enabled
	logger               sentry.Logger
	attributes           []attribute.Builder
	stackTrace           bool // Whether to include stack traces with error-level logs
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

// With adds structured context to the Core. It implements zapcore.Core interface.
func (s *SentryCore) With(fields []zapcore.Field) zapcore.Core {
	ctxOld := s.logger.GetCtx()
	if ctxOld == nil {
		ctxOld = context.Background()
	}

	attrs := append([]attribute.Builder(nil), s.attributes...)

	ctx, values := encodeFields(fields)
	if ctx != nil {
		ctxOld = ctx
	}

	attrs = append(attrs, attributesFromValues(values)...)

	logger := sentry.NewLogger(ctxOld)
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

// Write takes a log entry and sends it to Sentry.
// It implements zapcore.Core interface.
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

	go flushSentry()

	return nil
}

// Sync flushes any buffered log entries.
// It implements zapcore.Core interface.
func (*SentryCore) Sync() error {
	go flushSentry()
	return nil
}

func logEntryForLevel(logger sentry.Logger, level zapcore.Level) sentry.LogEntry {
	switch level {
	case zapcore.DebugLevel:
		return logger.Debug()
	case zapcore.InfoLevel:
		return logger.Info()
	case zapcore.WarnLevel:
		return logger.Warn()
	case zapcore.ErrorLevel:
		return logger.Error()
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		return logger.Error()
	default:
		return logger.Error()
	}
}

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

func attributesFromValues(values map[string]interface{}) []attribute.Builder {
	attrs := make([]attribute.Builder, 0, len(values))

	for k, v := range values {
		attrs = append(attrs, attributeFromValue(k, v))
	}

	return attrs
}

func attributeFromValue(key string, value interface{}) attribute.Builder {
	sink := &attributeSink{key: key}
	applyValue(value, sink)

	return sink.result
}

func applyValueToLogEntry(entry sentry.LogEntry, key string, value interface{}) sentry.LogEntry {
	sink := &logEntrySink{key: key, entry: entry}
	applyValue(value, sink)

	return sink.entry
}

type valueSink interface {
	SetString(string)
	SetBool(bool)
	SetInt(int)
	SetInt64(int64)
	SetFloat64(float64)
}

type attributeSink struct {
	key    string
	result attribute.Builder
}

func (sink *attributeSink) SetString(value string) { sink.result = attribute.String(sink.key, value) }
func (sink *attributeSink) SetBool(value bool)     { sink.result = attribute.Bool(sink.key, value) }
func (sink *attributeSink) SetInt(value int)       { sink.result = attribute.Int(sink.key, value) }
func (sink *attributeSink) SetInt64(value int64)   { sink.result = attribute.Int64(sink.key, value) }
func (sink *attributeSink) SetFloat64(value float64) {
	sink.result = attribute.Float64(sink.key, value)
}

type logEntrySink struct {
	entry sentry.LogEntry
	key   string
}

func (sink *logEntrySink) SetString(value string) { sink.entry = sink.entry.String(sink.key, value) }
func (sink *logEntrySink) SetBool(value bool)     { sink.entry = sink.entry.Bool(sink.key, value) }
func (sink *logEntrySink) SetInt(value int)       { sink.entry = sink.entry.Int(sink.key, value) }
func (sink *logEntrySink) SetInt64(value int64)   { sink.entry = sink.entry.Int64(sink.key, value) }
func (sink *logEntrySink) SetFloat64(value float64) {
	sink.entry = sink.entry.Float64(sink.key, value)
}

//nolint:cyclop,funlen // Type-switch is the clearest way to map values into sink methods.
func applyValue(value interface{}, sink valueSink) {
	switch v := value.(type) {
	case string:
		sink.SetString(v)
	case []byte:
		sink.SetString(string(v))
	case bool:
		sink.SetBool(v)
	case int:
		sink.SetInt(v)
	case int8:
		sink.SetInt(int(v))
	case int16:
		sink.SetInt(int(v))
	case int32:
		sink.SetInt(int(v))
	case int64:
		sink.SetInt64(v)
	case uint:
		if v > math.MaxInt64 {
			sink.SetString(fmt.Sprint(v))
			return
		}

		sink.SetInt64(int64(v))
	case uint8:
		sink.SetInt64(int64(v))
	case uint16:
		sink.SetInt64(int64(v))
	case uint32:
		sink.SetInt64(int64(v))
	case uint64:
		if v > math.MaxInt64 {
			sink.SetString(fmt.Sprint(v))
			return
		}

		sink.SetInt64(int64(v))
	case float32:
		sink.SetFloat64(float64(v))
	case float64:
		sink.SetFloat64(v)
	case time.Time:
		sink.SetString(v.Format(time.RFC3339Nano))
	case time.Duration:
		sink.SetString(v.String())
	case error:
		sink.SetString(v.Error())
	case fmt.Stringer:
		sink.SetString(v.String())
	default:
		sink.SetString(fmt.Sprint(v))
	}
}
