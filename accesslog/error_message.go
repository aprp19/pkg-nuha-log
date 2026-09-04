package accesslog

import (
	"context"

	"github.com/aprp19/pkg-nuha-log/internal/activityctx"
	"github.com/aprp19/pkg-nuha-log/logger"
	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/status"
)

const (
	accessLogErrorMessageKey = "_accesslog_error_message"
	accessLogErrorContextKey = "_accesslog_error_context"
	accessLogErrorResponseKey = "_accesslog_error_response"
)

type grpcContextKey string

const (
	grpcErrorMessageKey grpcContextKey = "_accesslog_error_message"
	grpcErrorContextKey grpcContextKey = "_accesslog_error_context"
)

// SetErrorMessage stores a human-readable failure reason for the access log event.
func SetErrorMessage(c echo.Context, msg string) {
	if msg == "" {
		return
	}
	if bag, ok := activityctx.BagFromContext(c.Request().Context()); ok {
		bag.Message = msg
		return
	}
	c.Set(accessLogErrorMessageKey, msg)
}

// SetErrorContext stores one structured error context field for the access log event.
func SetErrorContext(c echo.Context, key string, value interface{}) {
	if key == "" {
		return
	}
	if bag, ok := activityctx.BagFromContext(c.Request().Context()); ok {
		if bag.Context == nil {
			bag.Context = make(map[string]interface{})
		}
		bag.Context[key] = value
		return
	}
	existing, _ := c.Get(accessLogErrorContextKey).(map[string]interface{})
	if existing == nil {
		existing = make(map[string]interface{})
	}
	existing[key] = value
	c.Set(accessLogErrorContextKey, existing)
}

// SetErrorResponse stores the JSON error payload when it is written outside the access log wrapper.
func SetErrorResponse(c echo.Context, body interface{}) {
	if body != nil {
		c.Set(accessLogErrorResponseKey, body)
	}
}

// SetErrorMessageContext stores a human-readable failure reason on a gRPC context.
func SetErrorMessageContext(ctx context.Context, msg string) context.Context {
	if msg == "" {
		return ctx
	}
	if bag, ok := activityctx.BagFromContext(ctx); ok {
		bag.Message = msg
		return ctx
	}
	return context.WithValue(ctx, grpcErrorMessageKey, msg)
}

// SetErrorContextContext stores one structured error context field on a gRPC context.
func SetErrorContextContext(ctx context.Context, key string, value interface{}) context.Context {
	if key == "" {
		return ctx
	}

	if bag, ok := activityctx.BagFromContext(ctx); ok {
		if bag.Context == nil {
			bag.Context = make(map[string]interface{})
		}
		bag.Context[key] = value
		return ctx
	}

	existing, _ := ctx.Value(grpcErrorContextKey).(map[string]interface{})
	if existing == nil {
		existing = make(map[string]interface{})
	} else {
		cloned := make(map[string]interface{}, len(existing))
		for k, v := range existing {
			cloned[k] = v
		}
		existing = cloned
	}
	existing[key] = value
	return context.WithValue(ctx, grpcErrorContextKey, existing)
}

// LogError logs with zerolog, stores message and context fields for the access log, and returns err unchanged.
func LogError(c echo.Context, err error, msg string, fields ...string) error {
	if err == nil {
		return nil
	}

	SetErrorMessage(c, msg)
	for i := 0; i+1 < len(fields); i += 2 {
		SetErrorContext(c, fields[i], fields[i+1])
	}

	event := logger.Error().Err(err)
	for i := 0; i+1 < len(fields); i += 2 {
		event = event.Str(fields[i], fields[i+1])
	}
	event.Msg(msg)
	return err
}

func buildError(c echo.Context, err error, responseBody interface{}, statusCode int) *AccessLogError {
	if !isErrorResponse(err, statusCode) {
		return nil
	}
	return assembleAccessLogError(
		resolveErrorMessage(errorMessageFromEcho(c), c.Request().Context()),
		resolveErrorContext(errorContextFromEcho(c), c.Request().Context()),
		underlyingErrorCause(err),
		errorResponseFromSources(c, err, responseBody),
	)
}

func buildErrorFromGRPCContext(ctx context.Context, err error, responseBody interface{}, statusCode int) *AccessLogError {
	if !isErrorResponse(err, statusCode) {
		return nil
	}

	var response interface{}
	if !isResponseBodyEmpty(responseBody) {
		response = responseBody
	}

	return assembleAccessLogError(
		resolveErrorMessage(errorMessageFromGRPCContext(ctx), ctx),
		resolveErrorContext(errorContextFromGRPCContext(ctx), ctx),
		grpcUnderlyingCause(err),
		response,
	)
}

func resolveErrorMessage(explicit string, ctx context.Context) string {
	if explicit != "" {
		return explicit
	}
	if bag, ok := activityctx.BagFromContext(ctx); ok {
		return bag.Message
	}
	return ""
}

func resolveErrorContext(explicit map[string]interface{}, ctx context.Context) map[string]interface{} {
	bag, ok := activityctx.BagFromContext(ctx)
	if !ok || len(bag.Context) == 0 {
		return explicit
	}

	merged := sanitizeObject(bag.Context)
	if len(explicit) == 0 {
		return merged
	}

	for key, value := range explicit {
		merged[key] = value
	}
	return merged
}

func assembleAccessLogError(message string, ctxMap map[string]interface{}, cause string, response interface{}) *AccessLogError {
	accessErr := &AccessLogError{
		Message:  message,
		Context:  ctxMap,
		Cause:    cause,
		Response: response,
	}

	if accessErr.Message == "" && accessErr.Cause == "" && len(accessErr.Context) == 0 && accessErr.Response == nil {
		return nil
	}

	if len(accessErr.Context) == 0 {
		accessErr.Context = nil
	}

	return accessErr
}

func isErrorResponse(err error, statusCode int) bool {
	return err != nil || statusCode >= 400
}

func responseBodyIfSuccess(err error, statusCode int, body interface{}) interface{} {
	if isErrorResponse(err, statusCode) {
		return nil
	}
	return body
}

func errorResponseFromSources(c echo.Context, err error, responseBody interface{}) interface{} {
	if !isResponseBodyEmpty(responseBody) {
		return responseBody
	}

	if resp := c.Get(accessLogErrorResponseKey); resp != nil {
		return sanitizeErrorResponseValue(resp)
	}

	if he, ok := err.(*echo.HTTPError); ok {
		if structured := structuredHTTPErrorMessage(he.Message); structured != nil {
			return structured
		}
	}

	return nil
}

func errorMessageFromEcho(c echo.Context) string {
	msg, _ := stringFromValue(c.Get(accessLogErrorMessageKey))
	return msg
}

func errorContextFromEcho(c echo.Context) map[string]interface{} {
	raw, ok := c.Get(accessLogErrorContextKey).(map[string]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	return sanitizeObject(raw)
}

func errorMessageFromGRPCContext(ctx context.Context) string {
	msg, _ := stringFromValue(ctx.Value(grpcErrorMessageKey))
	return msg
}

func errorContextFromGRPCContext(ctx context.Context) map[string]interface{} {
	raw, ok := ctx.Value(grpcErrorContextKey).(map[string]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	return sanitizeObject(raw)
}

func underlyingErrorCause(err error) string {
	if err == nil {
		return ""
	}
	if he, ok := err.(*echo.HTTPError); ok {
		if msg, ok := he.Message.(string); ok {
			return msg
		}
	}
	return err.Error()
}

func grpcUnderlyingCause(err error) string {
	if err == nil {
		return ""
	}
	return status.Convert(err).Message()
}

func structuredHTTPErrorMessage(msg interface{}) interface{} {
	if msg == nil {
		return nil
	}

	switch v := msg.(type) {
	case map[string]interface{}:
		return sanitizeObject(v)
	case string:
		return nil
	default:
		data, err := jsonMarshal(v)
		if err != nil {
			return nil
		}
		var parsed map[string]interface{}
		if err := jsonUnmarshal(data, &parsed); err != nil {
			return nil
		}
		return sanitizeObject(parsed)
	}
}

func sanitizeErrorResponseValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return sanitizeObject(val)
	default:
		data, err := jsonMarshal(val)
		if err != nil {
			return val
		}
		var parsed interface{}
		if err := jsonUnmarshal(data, &parsed); err != nil {
			return val
		}
		return sanitizeValue(parsed)
	}
}

func isResponseBodyEmpty(body interface{}) bool {
	if body == nil {
		return true
	}
	if s, ok := body.(string); ok {
		return s == ""
	}
	if m, ok := body.(map[string]interface{}); ok {
		return len(m) == 0
	}
	return false
}
