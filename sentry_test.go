package sentryzapcore

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

var _ sentry.Transport = (*transportMock)(nil)

type transportMock struct {
	sync.Mutex
	events []*sentry.Event
}

func (*transportMock) Configure(_ sentry.ClientOptions) { /* stub */ }
func (t *transportMock) SendEvent(event *sentry.Event) {
	t.Lock()
	defer t.Unlock()
	t.events = append(t.events, event)
}
func (*transportMock) Flush(_ time.Duration) bool {
	return true
}
func (t *transportMock) FlushWithContext(_ context.Context) bool {
	return t.Flush(0)
}

func (t *transportMock) Events() []*sentry.Event {
	t.Lock()
	defer t.Unlock()
	return t.events
}
func (*transportMock) Close() {
	/* stub */
}

func findLog(events []*sentry.Event, message string) (*sentry.Log, bool) {
	for _, event := range events {
		for i := range event.Logs {
			log := &event.Logs[i]
			if log.Body == message {
				return log, true
			}
		}
	}

	return nil, false
}

type sentryZapCoreTest struct {
	suite.Suite
	transport *transportMock
}

func (s *sentryZapCoreTest) SetupTest() {
	s.transport = &transportMock{}
}

func (s *sentryZapCoreTest) Test0WithoutSentryInit() {
	s.Nil(sentry.CurrentHub().Client())
	s.Run("with info level", func() {
		logger := WithSentry(zaptest.NewLogger(s.T()), WithStackTrace())
		message := gofakeit.Sentence()
		logger.Info(message)
		sentry.Flush(2 * time.Second)
		_, found := findLog(s.transport.Events(), message)
		s.Require().False(found)
	})

	s.Run("with error level", func() {
		logger := WithSentry(zaptest.NewLogger(s.T()), WithStackTrace())
		message := gofakeit.Sentence()
		logger.Error(message)
		sentry.Flush(2 * time.Second)
		_, found := findLog(s.transport.Events(), message)
		s.Require().False(found)
	})
}

func (s *sentryZapCoreTest) TestWithErrorLog() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
		EnableLogs:  true,
	})

	s.Require().NoError(err)

	s.NotNil(sentry.CurrentHub().Client())

	s.Run("without stacktrace", func() {
		fakeId := gofakeit.UUID()
		message := gofakeit.Sentence()
		logger := WithSentry(zaptest.NewLogger(s.T()))
		logger.Error(message, zap.String("id", fakeId), zap.String("func", "test"), zap.Error(errors.New("error")))
		sentry.Flush(2 * time.Second)
		logEntry, found := findLog(s.transport.Events(), message)
		s.Require().True(found)
		s.Require().Equal(message, logEntry.Body)
		s.Require().Equal(fakeId, logEntry.Attributes["id"].String())
		s.Require().Equal("test", logEntry.Attributes["func"].String())
		s.Require().Equal("error", logEntry.Attributes["error"].String())
		s.Require().Equal(sentry.LogLevelError, logEntry.Level)
		s.Require().Equal("test", logEntry.Attributes["sentry.environment"].String())
	})
	s.Run("with stacktrace", func() {
		err := sentry.Init(sentry.ClientOptions{
			Transport:   s.transport,
			Environment: "test",
			EnableLogs:  true,
		})

		s.Require().NoError(err)

		fakeId := gofakeit.UUID()
		message := gofakeit.Sentence()
		logger := WithSentry(zaptest.NewLogger(s.T()), WithStackTrace())
		logger.Error(message, zap.String("id", fakeId), zap.String("func", "test"), zap.Error(errors.New("error")))
		sentry.Flush(2 * time.Second)
		logEntry, found := findLog(s.transport.Events(), message)
		s.Require().True(found)
		s.Require().NotEmpty(logEntry.Attributes["stacktrace"])
	})
}

func (s *sentryZapCoreTest) TestWithInfoLog() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
		EnableLogs:  true,
	})

	s.Require().NoError(err)

	s.NotNil(sentry.CurrentHub().Client())

	s.Run("without min level", func() {
		logger := WithSentry(zaptest.NewLogger(s.T()))
		message := gofakeit.Sentence()
		logger.Info(message)
		sentry.Flush(2 * time.Second)
		_, found := findLog(s.transport.Events(), message)
		s.Require().False(found)
	})
	s.Run("with min level info", func() {
		logger := WithSentry(zaptest.NewLogger(s.T()), WithMinLevel(zapcore.InfoLevel))
		message := gofakeit.Sentence()
		logger.Info(message)
		sentry.Flush(2 * time.Second)
		_, found := findLog(s.transport.Events(), message)
		s.Require().True(found)
	})
}

func (s *sentryZapCoreTest) TestWithSpanContext() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
		EnableLogs:  true,
	})

	s.Require().NoError(err)

	s.NotNil(sentry.CurrentHub().Client())

	opName := gofakeit.Word()
	rootSpan := sentry.StartSpan(context.Background(), opName+"_root")
	defer rootSpan.Finish()
	span := sentry.StartSpan(rootSpan.Context(), opName)
	defer span.Finish()

	fakeId := gofakeit.UUID()
	message := gofakeit.Sentence()
	ctxField := zap.Field{
		Key:       "ctx",
		Type:      zapcore.SkipType,
		Interface: span.Context(),
	}

	logger := WithSentry(zaptest.NewLogger(s.T()))
	logger.Error(message, zap.String("id", fakeId), zap.String("func", "test"), ctxField, zap.Error(errors.New("error")))
	sentry.Flush(2 * time.Second)
	logEntry, found := findLog(s.transport.Events(), message)
	s.Require().True(found)
	s.Require().Equal(fakeId, logEntry.Attributes["id"].String())
	s.Require().Equal("test", logEntry.Attributes["func"].String())
	s.Require().Equal("error", logEntry.Attributes["error"].String())
	_, hasCtx := logEntry.Attributes["ctx"]
	s.Require().False(hasCtx)
	s.Require().Equal(sentry.LogLevelError, logEntry.Level)
	s.Require().Equal("test", logEntry.Attributes["sentry.environment"].String())
	s.Require().Equal(span.TraceID, logEntry.TraceID)
	s.Require().Equal(span.SpanID, logEntry.SpanID)
}

func (s *sentryZapCoreTest) TestNewSentryCoreNilCtx() {
	core := NewSentryCore(nil) //nolint:staticcheck // intentional nil ctx to exercise default branch
	s.Require().NotNil(core)
}

func (s *sentryZapCoreTest) TestSync() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
		EnableLogs:  true,
	})
	s.Require().NoError(err)

	logger := WithSentry(zaptest.NewLogger(s.T()))
	s.Require().NoError(logger.Sync())
}

func (s *sentryZapCoreTest) TestDebugAndWarnLevels() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
		EnableLogs:  true,
	})
	s.Require().NoError(err)

	logger := WithSentry(zaptest.NewLogger(s.T()), WithMinLevel(zapcore.DebugLevel))

	debugMsg := gofakeit.Sentence()
	logger.Debug(debugMsg)

	warnMsg := gofakeit.Sentence()
	logger.Warn(warnMsg)

	sentry.Flush(2 * time.Second)

	debugLog, found := findLog(s.transport.Events(), debugMsg)
	s.Require().True(found)
	s.Require().Equal(sentry.LogLevelDebug, debugLog.Level)

	warnLog, found := findLog(s.transport.Events(), warnMsg)
	s.Require().True(found)
	s.Require().Equal(sentry.LogLevelWarn, warnLog.Level)
}

func (s *sentryZapCoreTest) TestWriteWithLoggerNameAndCaller() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
		EnableLogs:  true,
	})
	s.Require().NoError(err)

	logger := WithSentry(zaptest.NewLogger(s.T(), zaptest.WrapOptions(zap.AddCaller()))).Named("mylogger")
	message := gofakeit.Sentence()
	logger.Error(message)
	sentry.Flush(2 * time.Second)

	logEntry, found := findLog(s.transport.Events(), message)
	s.Require().True(found)
	s.Require().Equal("mylogger", logEntry.Attributes["logger"].String())
	s.Require().NotEmpty(logEntry.Attributes["caller.file"].String())
	s.Require().Greater(logEntry.Attributes["caller.line"].AsInt64(), int64(0))
}

func (s *sentryZapCoreTest) TestWithFields() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
		EnableLogs:  true,
	})
	s.Require().NoError(err)

	s.Run("with ctx field", func() {
		opName := gofakeit.Word()
		rootSpan := sentry.StartSpan(context.Background(), opName+"_root")
		defer rootSpan.Finish()

		ctxField := zap.Field{
			Key:       "ctx",
			Type:      zapcore.SkipType,
			Interface: rootSpan.Context(),
		}

		base := WithSentry(zaptest.NewLogger(s.T()))
		child := base.With(zap.String("component", "auth"), zap.Int("retry", 3), ctxField)

		message := gofakeit.Sentence()
		child.Error(message)
		sentry.Flush(2 * time.Second)

		logEntry, found := findLog(s.transport.Events(), message)
		s.Require().True(found)
		s.Require().Equal("auth", logEntry.Attributes["component"].String())
		s.Require().Equal(int64(3), logEntry.Attributes["retry"].AsInt64())
	})

	s.Run("without ctx field", func() {
		base := WithSentry(zaptest.NewLogger(s.T()))
		child := base.With(zap.String("service", "api"))

		message := gofakeit.Sentence()
		child.Error(message)
		sentry.Flush(2 * time.Second)

		logEntry, found := findLog(s.transport.Events(), message)
		s.Require().True(found)
		s.Require().Equal("api", logEntry.Attributes["service"].String())
	})
}

// stringerType exercises the fmt.Stringer branch of stringValue.
type stringerType struct{ s string }

func (st stringerType) String() string { return st.s }

func (s *sentryZapCoreTest) TestAllFieldTypes() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
		EnableLogs:  true,
	})
	s.Require().NoError(err)

	logger := WithSentry(zaptest.NewLogger(s.T()), WithMinLevel(zapcore.InfoLevel))

	now := time.Now().UTC().Truncate(time.Second)
	dur := 5 * time.Second

	message := gofakeit.Sentence()
	logger.Info(message,
		zap.Bool("b", true),
		zap.Int("i", -1),
		zap.Int8("i8", -8),
		zap.Int16("i16", -16),
		zap.Int32("i32", -32),
		zap.Int64("i64", -64),
		zap.Uint("u", 1),
		zap.Uint8("u8", 8),
		zap.Uint16("u16", 16),
		zap.Uint32("u32", 32),
		zap.Uint64("u64", 64),
		zap.Uint64("u64_overflow", math.MaxUint64),
		zap.Float32("f32", 1.5),
		zap.Float64("f64", 2.5),
		zap.Duration("dur", dur),
		zap.Time("t", now),
		zap.Binary("bin", []byte("hello")),
		zap.Stringer("stg", stringerType{s: "stringer-value"}),
		zap.Any("struct", struct{ A int }{A: 7}),
	)
	sentry.Flush(2 * time.Second)

	logEntry, found := findLog(s.transport.Events(), message)
	s.Require().True(found)

	s.Require().True(logEntry.Attributes["b"].AsBool())
	s.Require().Equal(int64(-1), logEntry.Attributes["i"].AsInt64())
	s.Require().Equal(int64(-8), logEntry.Attributes["i8"].AsInt64())
	s.Require().Equal(int64(-16), logEntry.Attributes["i16"].AsInt64())
	s.Require().Equal(int64(-32), logEntry.Attributes["i32"].AsInt64())
	s.Require().Equal(int64(-64), logEntry.Attributes["i64"].AsInt64())
	s.Require().Equal(int64(1), logEntry.Attributes["u"].AsInt64())
	s.Require().Equal(int64(8), logEntry.Attributes["u8"].AsInt64())
	s.Require().Equal(int64(16), logEntry.Attributes["u16"].AsInt64())
	s.Require().Equal(int64(32), logEntry.Attributes["u32"].AsInt64())
	s.Require().Equal(int64(64), logEntry.Attributes["u64"].AsInt64())
	// math.MaxUint64 overflows int64 → string fallback.
	s.Require().Equal("18446744073709551615", logEntry.Attributes["u64_overflow"].String())
	s.Require().InDelta(1.5, logEntry.Attributes["f32"].AsFloat64(), 0.0001)
	s.Require().InDelta(2.5, logEntry.Attributes["f64"].AsFloat64(), 0.0001)
	s.Require().Equal(dur.String(), logEntry.Attributes["dur"].String())
	s.Require().Equal(now.Format(time.RFC3339Nano), logEntry.Attributes["t"].String())
	s.Require().Equal("hello", logEntry.Attributes["bin"].String())
	s.Require().Equal("stringer-value", logEntry.Attributes["stg"].String())
	s.Require().NotEmpty(logEntry.Attributes["struct"].String())
}

func TestSentryZapCore(t *testing.T) {
	suite.Run(t, new(sentryZapCoreTest))
}

// TestConversionHelpers directly exercises the type-conversion helpers to
// cover `default` branches that zap's encoder normalizes away before
// applyValue sees them.
func TestConversionHelpers(t *testing.T) {
	// stringValue: non-matching type -> ("", false)
	if s, ok := stringValue(123); ok || s != "" {
		t.Errorf("stringValue(int) = (%q, %v), want (\"\", false)", s, ok)
	}
	// signedInt64Value: non-matching type -> (0, false)
	if n, ok := signedInt64Value("not-an-int"); ok || n != 0 {
		t.Errorf("signedInt64Value(string) = (%d, %v), want (0, false)", n, ok)
	}
	// unsignedInt64Value: non-matching type -> (0, false)
	if n, ok := unsignedInt64Value("not-a-uint"); ok || n != 0 {
		t.Errorf("unsignedInt64Value(string) = (%d, %v), want (0, false)", n, ok)
	}
}
