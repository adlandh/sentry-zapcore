package sentryzapcore

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

type transportMock struct {
	sync.Mutex
	events []*sentry.Event
}

func (*transportMock) Configure(_ sentry.ClientOptions) {}
func (t *transportMock) SendEvent(event *sentry.Event) {
	t.events = append(t.events, event)
}
func (*transportMock) Flush(_ time.Duration) bool {
	return true
}
func (t *transportMock) Events() []*sentry.Event {
	return t.events
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
		found := false
		for _, event := range s.transport.Events() {
			if event.Message == message {
				found = true
				s.Require().NotEmpty(event.EventID)
				s.Require().Equal(1, len(event.Exception))
				s.Require().Equal("*errors.errorString", event.Exception[0].Type)
				s.Require().Equal(message, event.Exception[0].Value)
				s.Require().NotEmpty(event.Exception[0].Stacktrace)
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
		found := false
		for _, event := range s.transport.Events() {
			if event.Message == message {
				found = true
			}
		}
		s.Require().True(found)
	})
}

func TestSentryZapCore(t *testing.T) {
	suite.Run(t, new(sentryZapCoreTest))
}
