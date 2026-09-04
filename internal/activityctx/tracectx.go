package activityctx

import "context"

type traceContextKey struct{}

// TraceContext holds W3C trace identifiers for the current request hop.
type TraceContext struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
}

// WithTraceContext attaches trace identifiers to ctx.
func WithTraceContext(ctx context.Context, tc TraceContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, traceContextKey{}, tc)
}

// TraceFromContext returns trace identifiers attached to ctx.
func TraceFromContext(ctx context.Context) (TraceContext, bool) {
	if ctx == nil {
		return TraceContext{}, false
	}
	tc, ok := ctx.Value(traceContextKey{}).(TraceContext)
	if !ok || tc.TraceID == "" || tc.SpanID == "" {
		return TraceContext{}, false
	}
	return tc, true
}

// CurrentTrace returns trace identifiers for the active request scope, if any.
func CurrentTrace() (TraceContext, bool) {
	return TraceFromContext(Current())
}
