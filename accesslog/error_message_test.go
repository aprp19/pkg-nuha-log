package accesslog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBuildError_full(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/documents/merge", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	SetErrorMessage(c, "merge document file fetch failed")
	SetErrorContext(c, "nik_pegawai", "P-2024-01")
	SetErrorContext(c, "document_path", "/asset/file/doc.pdf")

	err := errors.New("failed to stat file: The specified key does not exist.")
	responseBody := map[string]interface{}{
		"success": false,
		"message": "Internal Server Error",
		"meta": map[string]interface{}{
			"status":  500,
			"service": "gateway-service",
		},
	}

	accessErr := buildError(c, err, responseBody, http.StatusInternalServerError)
	if accessErr == nil {
		t.Fatal("expected error object")
	}
	if accessErr.Message != "merge document file fetch failed" {
		t.Fatalf("message = %q", accessErr.Message)
	}
	if accessErr.Cause != "failed to stat file: The specified key does not exist." {
		t.Fatalf("cause = %q", accessErr.Cause)
	}
	if accessErr.Context["nik_pegawai"] != "P-2024-01" {
		t.Fatalf("context = %#v", accessErr.Context)
	}
	resp, ok := accessErr.Response.(map[string]interface{})
	if !ok || resp["success"] != false {
		t.Fatalf("response = %#v", accessErr.Response)
	}
}

func TestBuildError_causeAndResponseOnly(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := errors.New("failed to stat file: The specified key does not exist.")
	responseBody := map[string]interface{}{
		"success": false,
		"message": "Internal Server Error",
	}

	accessErr := buildError(c, err, responseBody, http.StatusInternalServerError)
	if accessErr == nil {
		t.Fatal("expected error object")
	}
	if accessErr.Message != "" {
		t.Fatalf("message = %q, want empty", accessErr.Message)
	}
	if accessErr.Cause == "" {
		t.Fatal("expected cause")
	}
	if accessErr.Response == nil {
		t.Fatal("expected response")
	}
}

func TestBuildError_HTTPErrorStructuredMessage(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := echo.NewHTTPError(http.StatusInternalServerError, map[string]interface{}{
		"success": false,
		"message": "Internal Server Error",
	})

	accessErr := buildError(c, err, nil, http.StatusInternalServerError)
	if accessErr == nil {
		t.Fatal("expected error object")
	}
	resp, ok := accessErr.Response.(map[string]interface{})
	if !ok || resp["message"] != "Internal Server Error" {
		t.Fatalf("response = %#v", accessErr.Response)
	}
}

func TestBuildError_SetErrorResponseFallback(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	SetErrorResponse(c, map[string]interface{}{
		"success": false,
		"message": "Internal Server Error",
	})
	err := errors.New("boom")

	accessErr := buildError(c, err, nil, http.StatusInternalServerError)
	if accessErr == nil || accessErr.Response == nil {
		t.Fatalf("expected response fallback, got %#v", accessErr)
	}
}

func TestResponseBodyIfSuccess(t *testing.T) {
	body := map[string]interface{}{"success": true}
	if got := responseBodyIfSuccess(nil, http.StatusOK, body); got == nil {
		t.Fatal("expected success response body")
	}
	if got := responseBodyIfSuccess(nil, http.StatusBadRequest, body); got != nil {
		t.Fatal("expected nil response body on error status")
	}
	if got := responseBodyIfSuccess(errors.New("fail"), http.StatusOK, body); got != nil {
		t.Fatal("expected nil response body when handler returned error")
	}
}

func TestBuildError_successReturnsNil(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if accessErr := buildError(c, nil, map[string]interface{}{"ok": true}, http.StatusOK); accessErr != nil {
		t.Fatalf("expected nil error object, got %#v", accessErr)
	}
}

func TestBuildErrorFromGRPCContext(t *testing.T) {
	ctx := SetErrorMessageContext(context.Background(), "merge document file fetch failed")
	ctx = SetErrorContextContext(ctx, "nik_pegawai", "P-2024-01")

	err := status.Error(codes.Internal, "failed to stat file: The specified key does not exist.")
	responseBody := map[string]interface{}{"success": false}

	accessErr := buildErrorFromGRPCContext(ctx, err, responseBody, http.StatusInternalServerError)
	if accessErr == nil {
		t.Fatal("expected error object")
	}
	if accessErr.Message != "merge document file fetch failed" {
		t.Fatalf("message = %q", accessErr.Message)
	}
	if accessErr.Cause != "failed to stat file: The specified key does not exist." {
		t.Fatalf("cause = %q", accessErr.Cause)
	}
	if accessErr.Context["nik_pegawai"] != "P-2024-01" {
		t.Fatalf("context = %#v", accessErr.Context)
	}
}

func TestAccessLogEvent_errorJSON(t *testing.T) {
	event := AccessLogEvent{
		Service:    "gateway-service",
		Module:     "gateway",
		Method:     "GET",
		Path:       "/api/documents/merge",
		StatusCode: 500,
		Error: &AccessLogError{
			Message: "merge document file fetch failed",
			Cause:   "failed to stat file: The specified key does not exist.",
			Context: map[string]interface{}{
				"nik_pegawai": "P-2024-01",
			},
			Response: map[string]interface{}{
				"success": false,
				"message": "Internal Server Error",
			},
		},
		Timestamp: "2026-09-04T08:36:12Z",
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	s := string(payload)
	if strings.Contains(s, "error_message") {
		t.Fatalf("unexpected error_message field: %s", s)
	}
	if !strings.Contains(s, `"error":`) {
		t.Fatalf("expected nested error object: %s", s)
	}
	if !strings.Contains(s, `"message":"merge document file fetch failed"`) {
		t.Fatalf("expected error.message in JSON: %s", s)
	}
	if strings.Contains(s, "response_body") {
		t.Fatalf("expected no response_body on error event: %s", s)
	}
}

func TestAccessLogEvent_successKeepsResponseBody(t *testing.T) {
	event := AccessLogEvent{
		Service:      "gateway-service",
		Module:       "gateway",
		Method:       "GET",
		Path:         "/api/health",
		StatusCode:   200,
		ResponseBody: map[string]interface{}{"success": true},
		Timestamp:    "2026-09-04T08:36:12Z",
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	s := string(payload)
	if !strings.Contains(s, `"response_body"`) {
		t.Fatalf("expected response_body: %s", s)
	}
	if strings.Contains(s, `"error"`) {
		t.Fatalf("expected no error on success: %s", s)
	}
}

func TestSanitizeErrorContextSensitiveKey(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	SetErrorContext(c, "password", "secret")
	ctxMap := errorContextFromEcho(c)
	if ctxMap["password"] != "[REDACTED]" {
		t.Fatalf("password = %#v", ctxMap["password"])
	}
}
