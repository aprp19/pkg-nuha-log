package accesslog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aprp19/pkg-nuha-log/internal/activityctx"
	"google.golang.org/grpc/metadata"
)

func TestParseTraceparentValid(t *testing.T) {
	traceID, spanID, ok := ParseTraceparent("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	if !ok {
		t.Fatal("expected valid traceparent")
	}
	if traceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected trace id: %s", traceID)
	}
	if spanID != "00f067aa0ba902b7" {
		t.Fatalf("unexpected span id: %s", spanID)
	}
}

func TestParseTraceparentInvalid(t *testing.T) {
	cases := []string{
		"",
		"01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-short-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",
	}
	for _, value := range cases {
		if _, _, ok := ParseTraceparent(value); ok {
			t.Fatalf("expected invalid traceparent for %q", value)
		}
	}
}

func TestFormatTraceparentRoundTrip(t *testing.T) {
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"
	formatted := FormatTraceparent(traceID, spanID)
	parsedTraceID, parsedSpanID, ok := ParseTraceparent(formatted)
	if !ok {
		t.Fatal("expected formatted traceparent to parse")
	}
	if parsedTraceID != traceID || parsedSpanID != spanID {
		t.Fatalf("round trip mismatch: %s %s", parsedTraceID, parsedSpanID)
	}
}

func TestExtractOrCreateFromIncoming(t *testing.T) {
	incoming := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	traceID, spanID, parentSpanID, err := ExtractOrCreate(incoming)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if traceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected trace id: %s", traceID)
	}
	if parentSpanID != "00f067aa0ba902b7" {
		t.Fatalf("unexpected parent span id: %s", parentSpanID)
	}
	if spanID == "" || spanID == parentSpanID {
		t.Fatalf("expected new span id, got %s", spanID)
	}
}

func TestExtractOrCreateWithoutIncoming(t *testing.T) {
	traceID, spanID, parentSpanID, err := ExtractOrCreate("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if traceID == "" || spanID == "" {
		t.Fatal("expected generated trace and span ids")
	}
	if parentSpanID != "" {
		t.Fatalf("expected empty parent span id, got %s", parentSpanID)
	}
}

func TestInjectGRPCOutgoingUsesCurrentHop(t *testing.T) {
	ctx := activityctx.WithTraceContext(context.Background(), activityctx.TraceContext{
		TraceID:      "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:       "1111111111111111",
		ParentSpanID: "00f067aa0ba902b7",
	})
	activityctx.Enter(ctx)
	defer activityctx.Leave()

	outCtx := InjectGRPCOutgoing(context.Background())
	md, ok := metadata.FromOutgoingContext(outCtx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}
	values := md.Get(traceparentHeader)
	if len(values) != 1 {
		t.Fatalf("expected one traceparent value, got %v", values)
	}
	expected := FormatTraceparent("4bf92f3577b34da6a3ce929d0e0e4736", "1111111111111111")
	if values[0] != expected {
		t.Fatalf("unexpected traceparent: %s", values[0])
	}
}

func TestInjectHTTPOutgoingUsesContextTrace(t *testing.T) {
	ctx := activityctx.WithTraceContext(context.Background(), activityctx.TraceContext{
		TraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:  "2222222222222222",
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil).WithContext(ctx)
	InjectHTTPOutgoing(req)

	got := req.Header.Get(traceparentHeader)
	expected := FormatTraceparent("4bf92f3577b34da6a3ce929d0e0e4736", "2222222222222222")
	if got != expected {
		t.Fatalf("unexpected traceparent header: %s", got)
	}
}

func TestAttachTraceContextFromGRPCMetadata(t *testing.T) {
	incoming := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(traceparentHeader, incoming))
	fields := traceFieldsFromContext(attachTraceContext(context.Background(), incomingTraceparentFromGRPC(ctx)))
	if fields.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected trace id: %s", fields.TraceID)
	}
	if fields.ParentSpanID != "00f067aa0ba902b7" {
		t.Fatalf("unexpected parent span id: %s", fields.ParentSpanID)
	}
	if fields.SpanID == "" {
		t.Fatal("expected generated span id")
	}
}
