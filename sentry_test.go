package sentryzapcore

import (
	"context"
	"errors"
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
		message := gofakeit.Sentence(10)
		logger.Info(message)
		time.Sleep(30 * time.Millisecond)
		found := false
		for _, event := range s.transport.Events() {
			if event.Message == message {
				found = true
			}
		}
		s.Require().False(found)
	})

	s.Run("with error level", func() {
		logger := WithSentry(zaptest.NewLogger(s.T()), WithStackTrace())
		message := gofakeit.Sentence(10)
		logger.Error(message)
		time.Sleep(30 * time.Millisecond)
		found := false
		for _, event := range s.transport.Events() {
			if event.Message == message {
				found = true
			}
		}
		s.Require().False(found)
	})
}

func (s *sentryZapCoreTest) TestWithErrorLog() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
	})

	s.Require().NoError(err)

	s.NotNil(sentry.CurrentHub().Client())

	s.Run("without stacktrace", func() {
		fakeId := gofakeit.UUID()
		message := gofakeit.Sentence(10)
		logger := WithSentry(zaptest.NewLogger(s.T()))
		logger.Error(message, zap.String("id", fakeId), zap.String("func", "test"), zap.Error(errors.New("error")))
		time.Sleep(30 * time.Millisecond)
		found := false
		for _, event := range s.transport.Events() {
			if event.Message == message {
				found = true
				s.Require().Equal(fakeId, event.Extra["id"])
				s.Require().Equal("test", event.Extra["func"])
				s.Require().Equal("error", event.Extra["error"])
				s.Require().Equal(sentry.LevelError, event.Level)
				s.Require().Equal("test", event.Environment)
				s.Require().NotEmpty(event.EventID)
				s.Require().Empty(event.Exception)
				s.Require().NotEmpty(event.Contexts["trace"])
			}
		}
		s.Require().True(found)
	})
	s.Run("with stacktrace", func() {
		err := sentry.Init(sentry.ClientOptions{
			Transport:   s.transport,
			Environment: "test",
		})

		s.Require().NoError(err)

		fakeId := gofakeit.UUID()
		message := gofakeit.Sentence(10)
		logger := WithSentry(zaptest.NewLogger(s.T()), WithStackTrace())
		logger.Error(message, zap.String("id", fakeId), zap.String("func", "test"), zap.Error(errors.New("error")))
		time.Sleep(30 * time.Millisecond)
		found := false
		for _, event := range s.transport.Events() {
			if event.Message == message {
				found = true
				s.Require().NotEmpty(event.EventID)
				s.Require().Equal(1, len(event.Exception))
				s.Require().Equal("*errors.errorString", event.Exception[0].Type)
				s.Require().Equal(message, event.Exception[0].Value)
				s.Require().NotEmpty(event.Exception[0].Stacktrace)
				s.Require().NotEmpty(event.Contexts["trace"])
			}
		}
		s.Require().True(found)
	})
}

func (s *sentryZapCoreTest) TestWithInfoLog() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
	})

	s.Require().NoError(err)

	s.NotNil(sentry.CurrentHub().Client())

	s.Run("without min level", func() {
		logger := WithSentry(zaptest.NewLogger(s.T()))
		message := gofakeit.Sentence(10)
		logger.Info(message)
		time.Sleep(30 * time.Millisecond)
		found := false
		for _, event := range s.transport.Events() {
			if event.Message == message {
				found = true
			}
		}
		s.Require().False(found)
	})
	s.Run("with min level info", func() {
		logger := WithSentry(zaptest.NewLogger(s.T()), WithMinLevel(zapcore.InfoLevel))
		message := gofakeit.Sentence(10)
		logger.Info(message)
		time.Sleep(30 * time.Millisecond)
		found := false
		for _, event := range s.transport.Events() {
			if event.Message == message {
				found = true
			}
		}
		s.Require().True(found)
	})
}

func (s *sentryZapCoreTest) TestWithSpanContext() {
	err := sentry.Init(sentry.ClientOptions{
		Transport:   s.transport,
		Environment: "test",
	})

	s.Require().NoError(err)

	s.NotNil(sentry.CurrentHub().Client())

	opName := gofakeit.Word()
	rootSpan := sentry.StartSpan(context.Background(), opName+"_root")
	defer rootSpan.Finish()
	span := sentry.StartSpan(rootSpan.Context(), opName)
	defer span.Finish()

	fakeId := gofakeit.UUID()
	message := gofakeit.Sentence(10)
	ctxField := zap.Field{
		Key:       "ctx",
		Type:      zapcore.SkipType,
		Interface: span.Context(),
	}

	logger := WithSentry(zaptest.NewLogger(s.T()))
	logger.Error(message, zap.String("id", fakeId), zap.String("func", "test"), ctxField, zap.Error(errors.New("error")))
	time.Sleep(30 * time.Millisecond)
	found := false
	for _, event := range s.transport.Events() {
		if event.Message == message {
			found = true
			s.Require().Equal(fakeId, event.Extra["id"])
			s.Require().Equal("test", event.Extra["func"])
			s.Require().Equal("error", event.Extra["error"])
			s.Require().NotContains(event.Extra, "ctx")
			s.Require().Equal(sentry.LevelError, event.Level)
			s.Require().Equal("test", event.Environment)
			s.Require().NotEmpty(event.EventID)
			s.Require().Empty(event.Exception)
			s.Require().NotEmpty(event.Contexts["trace"])
			s.Require().EqualValues(event.Contexts["trace"]["op"], opName)
			s.Require().EqualValues(event.Contexts["trace"]["span_id"], span.SpanID)
			s.Require().EqualValues(event.Contexts["trace"]["trace_id"], span.TraceID)
		}
	}
	s.Require().True(found)
}

func TestSentryZapCore(t *testing.T) {
	suite.Run(t, new(sentryZapCoreTest))
}
