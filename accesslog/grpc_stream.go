package accesslog

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type streamCapture struct {
	firstRequest  interface{}
	response      interface{}
	chunkCount    int
	totalBytes    int64
	maxResponse   int
}

func newStreamCapture(maxResponse int) *streamCapture {
	return &streamCapture{maxResponse: maxResponse}
}

type wrappedServerStream struct {
	grpc.ServerStream
	capture *streamCapture
}

func (w *wrappedServerStream) RecvMsg(m interface{}) error {
	err := w.ServerStream.RecvMsg(m)
	if err != nil {
		return err
	}

	if w.capture.firstRequest == nil {
		w.capture.firstRequest = cloneProtoMessage(m)
	}

	if chunk, ok := extractByteChunk(m); ok {
		w.capture.chunkCount++
		w.capture.totalBytes += int64(len(chunk))
	}

	return nil
}

func (w *wrappedServerStream) SendMsg(m interface{}) error {
	w.capture.response = cloneProtoMessage(m)
	return w.ServerStream.SendMsg(m)
}

func cloneProtoMessage(m interface{}) interface{} {
	if pm, ok := m.(proto.Message); ok {
		return proto.Clone(pm)
	}
	return m
}

func extractByteChunk(m interface{}) ([]byte, bool) {
	type chunkGetter interface {
		GetChunk() []byte
	}
	if cg, ok := m.(chunkGetter); ok {
		chunk := cg.GetChunk()
		if len(chunk) > 0 {
			return chunk, true
		}
	}

	type fileContentGetter interface {
		GetFileContent() []byte
	}
	if fg, ok := m.(fileContentGetter); ok {
		content := fg.GetFileContent()
		if len(content) > 0 {
			return content, true
		}
	}

	return nil, false
}

func buildStreamRequestParams(capture *streamCapture, maxBytes int) map[string]interface{} {
	params := map[string]interface{}{}

	if capture.firstRequest != nil {
		params["body"] = protoToSanitizedValue(capture.firstRequest, maxBytes)
	}

	if capture.chunkCount > 0 || capture.totalBytes > 0 {
		params["stream"] = map[string]interface{}{
			"chunk_count": capture.chunkCount,
			"total_bytes": capture.totalBytes,
		}
	}

	return params
}

// StreamServerInterceptor returns a gRPC stream interceptor that logs access events.
func (c *Client) StreamServerInterceptor(module string) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if c.shouldSkip(info.FullMethod) {
			return handler(srv, ss)
		}

		start := time.Now()
		capture := newStreamCapture(c.maxResponseBytes)
		wrapped := &wrappedServerStream{
			ServerStream: ss,
			capture:      capture,
		}

		err := handler(srv, wrapped)

		event := AccessLogEvent{
			Service:       c.serviceName,
			Module:        module,
			Handler:       shortMethodName(info.FullMethod),
			Method:        "gRPC",
			Path:          info.FullMethod,
			Route:         info.FullMethod,
			Transport:     TransportGRPC,
			StatusCode:    grpcStatusCode(err),
			DurationMs:    time.Since(start).Milliseconds(),
			RequestParams: buildStreamRequestParams(capture, c.maxResponseBytes),
			ResponseBody:  protoToSanitizedValue(capture.response, c.maxResponseBytes),
			ErrorMessage:  grpcErrorMessage(err),
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}

		actor := captureActorFromGRPCContext(ss.Context(), c)
		if err == nil {
			actor = enrichActorFromResponse(actor, event.ResponseBody)
		}
		event.Actor = actor

		c.sendAsync(event)
		return err
	}
}
