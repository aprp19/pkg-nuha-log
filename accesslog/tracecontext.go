package accesslog

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	traceparentVersion = "00"
	traceparentFlags   = "01"
)

// ParseTraceparent parses a W3C traceparent header value.
// Returns trace ID, parent span ID, and whether parsing succeeded.
func ParseTraceparent(value string) (traceID, parentSpanID string, ok bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", false
	}

	parts := strings.Split(value, "-")
	if len(parts) != 4 {
		return "", "", false
	}
	if parts[0] != traceparentVersion {
		return "", "", false
	}
	if !isHex(parts[1], 32) || !isHex(parts[2], 16) || !isHex(parts[3], 2) {
		return "", "", false
	}

	return strings.ToLower(parts[1]), strings.ToLower(parts[2]), true
}

// FormatTraceparent builds a W3C traceparent value for outbound propagation.
func FormatTraceparent(traceID, spanID string) string {
	traceID = strings.ToLower(strings.TrimSpace(traceID))
	spanID = strings.ToLower(strings.TrimSpace(spanID))
	return fmt.Sprintf("%s-%s-%s-%s", traceparentVersion, traceID, spanID, traceparentFlags)
}

// GenerateTraceID returns a new 128-bit trace ID as 32 lowercase hex chars.
func GenerateTraceID() (string, error) {
	return randomHex(16)
}

// GenerateSpanID returns a new 64-bit span ID as 16 lowercase hex chars.
func GenerateSpanID() (string, error) {
	return randomHex(8)
}

// ExtractOrCreate parses inbound traceparent or starts a new trace for this hop.
func ExtractOrCreate(incoming string) (traceID, spanID, parentSpanID string, err error) {
	if incomingTraceID, incomingSpanID, ok := ParseTraceparent(incoming); ok {
		newSpanID, genErr := GenerateSpanID()
		if genErr != nil {
			return "", "", "", genErr
		}
		return incomingTraceID, newSpanID, incomingSpanID, nil
	}

	newTraceID, err := GenerateTraceID()
	if err != nil {
		return "", "", "", err
	}
	newSpanID, err := GenerateSpanID()
	if err != nil {
		return "", "", "", err
	}
	return newTraceID, newSpanID, "", nil
}

func randomHex(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func traceContextFromIncoming(incoming string) (TraceFields, error) {
	traceID, spanID, parentSpanID, err := ExtractOrCreate(incoming)
	if err != nil {
		return TraceFields{}, err
	}
	return TraceFields{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
	}, nil
}

// TraceFields holds trace identifiers stored on access log events.
type TraceFields struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
}

func applyTraceFields(event *AccessLogEvent, fields TraceFields) {
	if event == nil {
		return
	}
	event.TraceID = fields.TraceID
	event.SpanID = fields.SpanID
	event.ParentSpanID = fields.ParentSpanID
}
