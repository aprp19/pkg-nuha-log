package accesslog

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// UnaryServerInterceptor returns a gRPC unary interceptor that logs access events.
func (c *Client) UnaryServerInterceptor(module string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if c.shouldSkip(info.FullMethod) {
			return handler(ctx, req)
		}

		start := time.Now()
		resp, err := handler(ctx, req)

		requestCode := ""
		if c.requestCodeField {
			requestCode = extractRequestCode(req)
		}

		event := AccessLogEvent{
			Service:       c.serviceName,
			Module:        module,
			Handler:       buildHandlerName(info.FullMethod, requestCode),
			Method:        "gRPC",
			Path:          info.FullMethod,
			Route:         info.FullMethod,
			Transport:     TransportGRPC,
			RequestCode:   requestCode,
			StatusCode:    grpcStatusCode(err),
			RequestParams: buildGRPCRequestParams(ctx, req, c.maxResponseBytes),
			ResponseBody:  protoToSanitizedValue(resp, c.maxResponseBytes),
			ErrorMessage:  grpcErrorMessage(err),
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}
		setDurationFromStart(&event, start)

		actor := captureActorFromGRPCContext(ctx, c)
		if err == nil {
			actor = enrichActorFromResponse(actor, event.ResponseBody)
		}
		event.Actor = actor

		c.sendAsync(event)
		return resp, err
	}
}
