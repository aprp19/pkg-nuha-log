package accesslog

import (
	"context"
	"net/http"

	"github.com/aprp19/pkg-nuha-log/internal/activityctx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const traceparentHeader = "traceparent"

// InjectGRPCOutgoing attaches the current hop traceparent to outgoing gRPC metadata.
func InjectGRPCOutgoing(ctx context.Context) context.Context {
	tc, ok := activityctx.TraceFromContext(ctx)
	if !ok {
		if current, currentOK := activityctx.CurrentTrace(); currentOK {
			tc = current
		} else {
			return ctx
		}
	}

	value := FormatTraceparent(tc.TraceID, tc.SpanID)
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return metadata.AppendToOutgoingContext(ctx, traceparentHeader, value)
	}

	md = md.Copy()
	md.Set(traceparentHeader, value)
	return metadata.NewOutgoingContext(ctx, md)
}

// InjectHTTPOutgoing sets traceparent on an outgoing HTTP request from the current scope.
func InjectHTTPOutgoing(req *http.Request) {
	if req == nil {
		return
	}

	ctx := req.Context()
	tc, ok := activityctx.TraceFromContext(ctx)
	if !ok {
		if current, currentOK := activityctx.CurrentTrace(); currentOK {
			tc = current
		} else {
			return
		}
	}

	req.Header.Set(traceparentHeader, FormatTraceparent(tc.TraceID, tc.SpanID))
}

// UnaryClientInterceptor injects traceparent on all outbound unary gRPC calls.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		return invoker(InjectGRPCOutgoing(ctx), method, req, reply, cc, opts...)
	}
}

func incomingTraceparentFromGRPC(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(traceparentHeader); len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func attachTraceContext(ctx context.Context, incoming string) context.Context {
	fields, err := traceContextFromIncoming(incoming)
	if err != nil {
		return ctx
	}
	return activityctx.WithTraceContext(ctx, activityctx.TraceContext{
		TraceID:      fields.TraceID,
		SpanID:       fields.SpanID,
		ParentSpanID: fields.ParentSpanID,
	})
}

func traceFieldsFromContext(ctx context.Context) TraceFields {
	if tc, ok := activityctx.TraceFromContext(ctx); ok {
		return TraceFields{
			TraceID:      tc.TraceID,
			SpanID:       tc.SpanID,
			ParentSpanID: tc.ParentSpanID,
		}
	}
	return TraceFields{}
}
