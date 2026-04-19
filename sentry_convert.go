package sentryzapcore

import (
	"fmt"
	"math"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
)

// valueSink abstracts writing a typed value to either an attribute.Builder
// or a sentry.LogEntry so that applyValue can be shared by both.
type valueSink interface {
	SetString(string)
	SetBool(bool)
	SetInt(int)
	SetInt64(int64)
	SetFloat64(float64)
}

// attributeSink populates an attribute.Builder (used during With()).
type attributeSink struct {
	result attribute.Builder
	key    string
}

func (sink *attributeSink) SetString(value string) { sink.result = attribute.String(sink.key, value) }
func (sink *attributeSink) SetBool(value bool)     { sink.result = attribute.Bool(sink.key, value) }
func (sink *attributeSink) SetInt(value int)       { sink.result = attribute.Int(sink.key, value) }
func (sink *attributeSink) SetInt64(value int64)   { sink.result = attribute.Int64(sink.key, value) }
func (sink *attributeSink) SetFloat64(value float64) {
	sink.result = attribute.Float64(sink.key, value)
}

// logEntrySink populates a sentry.LogEntry (used during Write()).
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

// attributeFromValue converts a single key/value pair to an attribute.Builder.
func attributeFromValue(key string, value interface{}) attribute.Builder {
	sink := &attributeSink{key: key}
	applyValue(value, sink)

	return sink.result
}

// applyValueToLogEntry writes a single key/value pair to a sentry.LogEntry.
func applyValueToLogEntry(entry sentry.LogEntry, key string, value interface{}) sentry.LogEntry {
	sink := &logEntrySink{key: key, entry: entry}
	applyValue(value, sink)

	return sink.entry
}

// attributesFromValues converts a map of key/value pairs to attribute.Builders.
func attributesFromValues(values map[string]interface{}) []attribute.Builder {
	attrs := make([]attribute.Builder, 0, len(values))

	for k, v := range values {
		attrs = append(attrs, attributeFromValue(k, v))
	}

	return attrs
}

// applyValue routes a value through type converters and writes the result
// to the provided sink. Unknown types are converted via fmt.Sprint.
func applyValue(value interface{}, sink valueSink) {
	if v, ok := timeStringValue(value); ok {
		sink.SetString(v)
		return
	}

	if v, ok := stringValue(value); ok {
		sink.SetString(v)
		return
	}

	if v, ok := boolValue(value); ok {
		sink.SetBool(v)
		return
	}

	if v, ok := signedInt64Value(value); ok {
		sink.SetInt64(v)
		return
	}

	if v, ok := unsignedInt64Value(value); ok {
		if v > math.MaxInt64 {
			sink.SetString(fmt.Sprint(value))
			return
		}

		sink.SetInt64(int64(v))

		return
	}

	if v, ok := float64Value(value); ok {
		sink.SetFloat64(v)
		return
	}

	sink.SetString(fmt.Sprint(value))
}

func timeStringValue(value interface{}) (string, bool) {
	switch v := value.(type) {
	case time.Time:
		return v.Format(time.RFC3339Nano), true
	case time.Duration:
		return v.String(), true
	default:
		return "", false
	}
}

func stringValue(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	case error:
		return v.Error(), true
	case fmt.Stringer:
		return v.String(), true
	default:
		return "", false
	}
}

func boolValue(value interface{}) (bool, bool) {
	v, ok := value.(bool)
	return v, ok
}

func signedInt64Value(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	default:
		return 0, false
	}
}

func unsignedInt64Value(value interface{}) (uint64, bool) {
	switch v := value.(type) {
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	default:
		return 0, false
	}
}

func float64Value(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}
