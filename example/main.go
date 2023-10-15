package main

import (
	"errors"

	sentryzapcore "github.com/adlandh/sentry-zapcore"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

func main() {
	err := sentry.Init(sentry.ClientOptions{})
	if err != nil {
		panic(err)
	}

	logger, err := zap.NewDevelopment(sentryzapcore.WithSentryOption(sentryzapcore.WithStackTrace()))
	if err != nil {
		panic(err)
	}

	logger.Debug("debug message") // will not be sent to sentry
	logger.Info("info message")   // will not be sent to sentry
	logger.Warn("warn message")   // will not be sent to sentry
	logger.Error("error message with fake error", zap.String("just a test string", "something"),
		zap.Error(errors.New("fake error"))) // will be sent to sentry with additional data
	logger.Fatal("fatal message") // will be sent to sentry
}
