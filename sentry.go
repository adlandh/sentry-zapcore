package sentry_zapcore

import (
	"errors"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

var _ zapcore.Core = (*SentryCore)(nil)

type SentryCore struct {
	zapcore.LevelEnabler
	fields     map[string]interface{}
	stackTrace bool
}

func NewSentryCore(options ...SentryCoreOptions) *SentryCore {
	s := &SentryCore{LevelEnabler: zapcore.ErrorLevel, fields: make(map[string]interface{})}

	for _, opt := range options {
		opt(s)
	}

	return s
}

func (s *SentryCore) With(fields []zapcore.Field) zapcore.Core {
	return s.with(fields)
}

func (s *SentryCore) with(fields []zapcore.Field) *SentryCore {
	// Copy our map.
	m := make(map[string]interface{}, len(s.fields))
	for k, v := range s.fields {
		m[k] = v
	}

	// Add fields to an in-memory encoder.
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}

	// Merge the two maps.
	for k, v := range enc.Fields {
		m[k] = v
	}

	return &SentryCore{
		LevelEnabler: s.LevelEnabler,
		fields:       m,
	}
}

func (s *SentryCore) Check(entry zapcore.Entry, checkEntry *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(entry.Level) {
		return checkEntry.AddCore(entry, s)
	}
	return checkEntry
}

func (s *SentryCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	defer sentry.Flush(2 * time.Second)
	localHub := sentry.CurrentHub().Clone()
	localHub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("file", entry.Caller.File)
		scope.SetTag("line", strconv.Itoa(entry.Caller.Line))
	})

	clone := s.with(fields)

	event := &sentry.Event{
		Extra:       clone.fields,
		Fingerprint: []string{entry.Message},
		Level:       sentrySeverity(entry.Level),
		Message:     entry.Message,
		Platform:    "go",
		Timestamp:   entry.Time,
		Logger:      entry.LoggerName,
	}

	if entry.Level >= zapcore.ErrorLevel && s.stackTrace {
		event.SetException(errors.New(entry.Message), localHub.Client().Options().MaxErrorDepth)
	}

	localHub.CaptureEvent(event)

	return nil
}

func (s *SentryCore) Sync() error {
	sentry.Flush(2 * time.Second)
	return nil
}

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
		// Unrecognized levels are fatal.
		return sentry.LevelFatal
	}
}
