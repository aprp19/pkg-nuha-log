package accesslog

import "time"

const (
	TransportHTTP = "http"
	TransportGRPC = "grpc"
)

// Actor holds user/context metadata for an activity log event.
type Actor struct {
	UserID       *int    `json:"user_id,omitempty"`
	UserEmail    *string `json:"user_email,omitempty"`
	UserUUID     *string `json:"user_uuid,omitempty"`
	UserName     *string `json:"user_name,omitempty"`
	CodeHospital *string `json:"code_hospital,omitempty"`
	ClientKey    *string `json:"client_key,omitempty"`
	TenantHubID  *string `json:"tenant_hub_id,omitempty"`
}

// AccessLogError holds structured failure details for an activity log event.
type AccessLogError struct {
	Message  string                 `json:"message,omitempty"`
	Cause    string                 `json:"cause,omitempty"`
	Context  map[string]interface{} `json:"context,omitempty"`
	Response interface{}            `json:"response,omitempty"`
}

// AccessLogEvent is the shared contract between producer services, the ingestion API,
// Redpanda messages, and downstream consumers.
type AccessLogEvent struct {
	Service       string                 `json:"service"`
	Module        string                 `json:"module"`
	Handler       string                 `json:"handler,omitempty"`
	Method        string                 `json:"method"`
	Path          string                 `json:"path"`
	Route         string                 `json:"route,omitempty"`
	Transport     string                 `json:"transport,omitempty"`
	RequestCode   string                 `json:"request_code,omitempty"`
	StatusCode    int                    `json:"status_code"`
	DurationMs    *int64                 `json:"duration_ms,omitempty"`
	DurationUs    *int64                 `json:"duration_us,omitempty"`
	RequestParams map[string]interface{} `json:"request_params,omitempty"`
	ResponseBody  interface{}     `json:"response_body,omitempty"`
	Actor         Actor           `json:"actor,omitempty"`
	Error         *AccessLogError `json:"error,omitempty"`
	TraceID       string          `json:"trace_id,omitempty" bson:"trace_id,omitempty"`
	SpanID        string          `json:"span_id,omitempty" bson:"span_id,omitempty"`
	ParentSpanID  string          `json:"parent_span_id,omitempty" bson:"parent_span_id,omitempty"`
	Timestamp     string                 `json:"timestamp"`
}

func (e *AccessLogEvent) EnsureTimestamp() {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
}

func (e *AccessLogEvent) Validate() error {
	if e.Service == "" {
		return errRequired("service")
	}
	if e.Module == "" {
		return errRequired("module")
	}
	if e.Method == "" {
		return errRequired("method")
	}
	if e.Path == "" {
		return errRequired("path")
	}
	return nil
}

type validationError struct {
	field string
}

func (e validationError) Error() string {
	return "missing required field: " + e.field
}

func errRequired(field string) error {
	return validationError{field: field}
}
