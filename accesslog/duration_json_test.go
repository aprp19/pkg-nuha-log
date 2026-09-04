package accesslog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAccessLogEvent_durationJSON_subMillisecond(t *testing.T) {
	us := int64(450)
	event := AccessLogEvent{
		Service:    "nuha-auth",
		Module:     "auth",
		Method:     "GET",
		Path:       "/api/auth/validate-session",
		StatusCode: 200,
		DurationUs: &us,
		Timestamp:  "2026-09-03T08:00:00Z",
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	s := string(payload)
	if !strings.Contains(s, `"duration_us":450`) {
		t.Fatalf("expected duration_us in JSON, got %s", s)
	}
	if strings.Contains(s, "duration_ms") {
		t.Fatalf("expected no duration_ms in JSON, got %s", s)
	}
}

func TestAccessLogEvent_durationJSON_millisecondOrMore(t *testing.T) {
	ms := int64(12)
	event := AccessLogEvent{
		Service:    "nuha-auth",
		Module:     "auth",
		Method:     "POST",
		Path:       "/api/auth/login",
		StatusCode: 200,
		DurationMs: &ms,
		Timestamp:  "2026-09-03T08:00:00Z",
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	s := string(payload)
	if !strings.Contains(s, `"duration_ms":12`) {
		t.Fatalf("expected duration_ms in JSON, got %s", s)
	}
	if strings.Contains(s, "duration_us") {
		t.Fatalf("expected no duration_us in JSON, got %s", s)
	}
}
