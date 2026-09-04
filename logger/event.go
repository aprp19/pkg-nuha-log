package logger

import (
	"github.com/aprp19/pkg-nuha-log/internal/activityctx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Event wraps zerolog.Event and captures Error-level logs for access log events.
type Event struct {
	inner  *zerolog.Event
	fields map[string]interface{}
}

// Error starts an error-level log event wired for access log auto-capture.
func Error() *Event {
	return &Event{
		inner:  log.Error(),
		fields: make(map[string]interface{}),
	}
}

func (e *Event) Str(key, val string) *Event {
	e.inner = e.inner.Str(key, val)
	if key != "" {
		e.fields[key] = val
	}
	return e
}

func (e *Event) Int(key string, val int) *Event {
	e.inner = e.inner.Int(key, val)
	if key != "" {
		e.fields[key] = val
	}
	return e
}

func (e *Event) Int64(key string, val int64) *Event {
	e.inner = e.inner.Int64(key, val)
	if key != "" {
		e.fields[key] = val
	}
	return e
}

func (e *Event) Bool(key string, val bool) *Event {
	e.inner = e.inner.Bool(key, val)
	if key != "" {
		e.fields[key] = val
	}
	return e
}

func (e *Event) Err(err error) *Event {
	e.inner = e.inner.Err(err)
	return e
}

func (e *Event) Interface(key string, val interface{}) *Event {
	e.inner = e.inner.Interface(key, val)
	if key != "" {
		e.fields[key] = val
	}
	return e
}

func (e *Event) Msg(msg string) {
	activityctx.CaptureErrorLog(msg, e.fields)
	e.inner.Msg(msg)
}
