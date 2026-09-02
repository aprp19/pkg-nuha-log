package accesslog

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var protoJSONMarshaler = protojson.MarshalOptions{
	UseProtoNames: true,
}

type requestCodeGetter interface {
	GetRequestCode() string
}

func extractRequestCode(req interface{}) string {
	if req == nil {
		return ""
	}
	if rc, ok := req.(requestCodeGetter); ok {
		return rc.GetRequestCode()
	}
	return ""
}

func shortMethodName(fullMethod string) string {
	if idx := strings.LastIndex(fullMethod, "/"); idx >= 0 && idx+1 < len(fullMethod) {
		return fullMethod[idx+1:]
	}
	return fullMethod
}

func buildHandlerName(fullMethod, requestCode string) string {
	method := shortMethodName(fullMethod)
	if requestCode == "" {
		return method
	}
	return method + "/" + requestCode
}

func protoToSanitizedValue(msg interface{}, maxBytes int) interface{} {
	if msg == nil {
		return nil
	}

	if pm, ok := msg.(proto.Message); ok {
		data, err := protoJSONMarshaler.Marshal(pm)
		if err != nil {
			return map[string]interface{}{"_marshal_error": err.Error()}
		}
		return sanitizeResponseBody(redactBinaryFields(data), maxBytes)
	}

	if b, ok := msg.([]byte); ok {
		return map[string]interface{}{
			"bytes_length": len(b),
		}
	}

	data, err := jsonMarshal(msg)
	if err != nil {
		return nil
	}
	return sanitizeResponseBody(data, maxBytes)
}

func redactBinaryFields(data []byte) []byte {
	var parsed interface{}
	if err := jsonUnmarshal(data, &parsed); err != nil {
		return data
	}
	redacted := redactBinaryInValue(parsed)
	out, err := jsonMarshal(redacted)
	if err != nil {
		return data
	}
	return out
}

func redactBinaryInValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for key, item := range v {
			if key == "chunk" || key == "file_content" || strings.HasSuffix(key, "_content") {
				if s, ok := item.(string); ok {
					out[key] = fmt.Sprintf("<%d bytes>", len(s))
					continue
				}
			}
			out[key] = redactBinaryInValue(item)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = redactBinaryInValue(item)
		}
		return out
	default:
		return v
	}
}

func buildGRPCRequestParams(ctx context.Context, req interface{}, maxBytes int) map[string]interface{} {
	params := map[string]interface{}{
		"body": protoToSanitizedValue(req, maxBytes),
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok && len(md) > 0 {
		meta := make(map[string]interface{})
		for key, values := range md {
			if isSensitiveKey(key) {
				meta[key] = "[REDACTED]"
				continue
			}
			if len(values) == 1 {
				meta[key] = values[0]
			} else {
				meta[key] = values
			}
		}
		params["metadata"] = sanitizeObject(meta)
	}

	return params
}

func captureActorFromGRPCContext(ctx context.Context, c *Client) Actor {
	if c.actorExtractor != nil {
		return c.actorExtractor(ctx)
	}
	return defaultGRPCActorExtractor(ctx)
}

func defaultGRPCActorExtractor(ctx context.Context) Actor {
	actor := mergeActor(Actor{}, actorFromContextKeys(ctx))
	actor = mergeActor(actor, actorFromClaims(claimsFromContext(ctx)))

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if tenant := tenantHubIDFromMetadata(md); tenant != "" {
			actor.TenantHubID = strPtr(tenant)
		}
	}

	return actor
}

func firstMetadata(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func grpcErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return status.Convert(err).Message()
}

func grpcStatusCode(err error) int {
	if err == nil {
		return grpcCodeToHTTPStatus(codes.OK)
	}
	return grpcCodeToHTTPStatus(status.Code(err))
}
