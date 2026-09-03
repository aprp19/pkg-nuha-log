package accesslog

import (
	"testing"
	"time"
)

func TestSetDurationFromStart_subMillisecond(t *testing.T) {
	start := time.Now().Add(-500 * time.Microsecond)
	event := &AccessLogEvent{}
	setDurationFromStart(event, start)

	if event.DurationMs != nil {
		t.Fatalf("expected DurationMs nil, got %v", *event.DurationMs)
	}
	if event.DurationUs == nil {
		t.Fatal("expected DurationUs set")
	}
	if *event.DurationUs < 400 || *event.DurationUs > 600 {
		t.Fatalf("expected ~500µs, got %d", *event.DurationUs)
	}
}

func TestSetDurationFromStart_millisecondOrMore(t *testing.T) {
	start := time.Now().Add(-5 * time.Millisecond)
	event := &AccessLogEvent{}
	setDurationFromStart(event, start)

	if event.DurationUs != nil {
		t.Fatalf("expected DurationUs nil, got %v", *event.DurationUs)
	}
	if event.DurationMs == nil {
		t.Fatal("expected DurationMs set")
	}
	if *event.DurationMs < 4 || *event.DurationMs > 10 {
		t.Fatalf("expected ~5ms, got %d", *event.DurationMs)
	}
}

func TestSetDurationFromStart_exactlyOneMillisecond(t *testing.T) {
	start := time.Now().Add(-time.Millisecond)
	event := &AccessLogEvent{}
	setDurationFromStart(event, start)

	if event.DurationUs != nil {
		t.Fatalf("expected DurationUs nil at 1ms boundary, got %v", *event.DurationUs)
	}
	if event.DurationMs == nil || *event.DurationMs < 1 {
		t.Fatal("expected DurationMs >= 1 at 1ms boundary")
	}
}
