# accesslog — Activity logging client

> Module root: [../README.md](../README.md) · AI integration: [../AGENTS.md](../AGENTS.md)

Client library for capturing controller-level **activity logs** and sending them to **hub-ingestion-service**. Supports **HTTP (Echo)** and **gRPC (unary + streaming)**.

## Installation

```bash
go get github.com/aprp19/pkg-nuha-log@v0.1.0
```

```go
import "github.com/aprp19/pkg-nuha-log/accesslog"
```

## How it works

```
Your Echo handler
      │
      ▼
 accesslog.Wrap / GroupWrapper
      │  captures request + response
      ▼
 POST hub-ingestion-service/api/logs  (async, fire-and-forget)
      │
      ▼
 Redpanda → consumer-service
```

Logging never blocks your HTTP response. Send failures are logged locally and ignored.

## Configuration

Add to your service `.env`:

```env
HUB_INGESTION_URL=http://hub-ingestion-service:8080
INGEST_API_KEY=your-shared-secret          # optional, must match hub-ingestion-service
ACCESS_LOG_MAX_RESPONSE_BYTES=65536        # optional, default 64KB
```

| Variable | Required | Description |
|----------|----------|-------------|
| `HUB_INGESTION_URL` | yes | Base URL of hub-ingestion-service |
| `INGEST_API_KEY` | no | Sent as `X-Ingest-Key` header |
| `ACCESS_LOG_MAX_RESPONSE_BYTES` | no | Max response body captured (default `65536`) |

## Quick start

### 1. Initialize once at startup

```go
package main

import (
    "os"
    "github.com/aprp19/pkg-nuha-log/accesslog"
)

var accessLog *accesslog.Client

func initAccessLog() {
    accessLog = accesslog.NewClient(accesslog.ClientConfig{
        IngestionURL:     os.Getenv("HUB_INGESTION_URL"),
        ServiceName:      "hub-user-service",   // your service name
        IngestAPIKey:     os.Getenv("INGEST_API_KEY"),
        MaxResponseBytes: 65536,
    })
}
```

### 2. Wrap routes at controller/module level

Use `GroupWrapper` once per controller — same idea as `@OpenApiAccessLog('user')`:

```go
package routes

import (
    "your-service/internal/app/user/controller"
    "github.com/aprp19/pkg-nuha-log/accesslog"
    "github.com/labstack/echo/v4"
)

func RegisterRoutes(api *echo.Group, ctrl *controller.Controller, logClient *accesslog.Client) {
    wrap := logClient.GroupWrapper("user")

    users := api.Group("/users")
    users.GET("/:id", wrap(ctrl.GetByID))
    users.POST("", wrap(ctrl.Create))
    users.PUT("/:id", wrap(ctrl.Put))
    users.DELETE("/:id", wrap(ctrl.Delete))
}
```

### 3. Single handler (optional)

```go
api.GET("/profile", accessLog.Wrap("user", ctrl.GetProfile))
```

No changes needed inside your controller — wrap at the route registration layer.

## gRPC integration

See **[AGENTS.md](../AGENTS.md)** for the full AI integration playbook. Summary:

### 1. Register interceptors at gRPC server bootstrap

Auth outer, activity log inner (matches crm-gateway / crm-organization):

```go
grpcServer := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        middleware.GRPCAuthInterceptor(cfg),
        logClient.UnaryServerInterceptor("organization"),
    ),
    grpc.ChainStreamInterceptor(
        logClient.StreamServerInterceptor("organization"),
    ),
)
```

### 2. What gRPC events look like

| Field | Value |
|-------|-------|
| `method` | `"gRPC"` |
| `transport` | `"grpc"` |
| `path` / `route` | Full method e.g. `/organization.OrganizationService/ConnectOrganizationCrm` |
| `handler` | Short method name, or `Method/REQUEST_CODE` when proto has `GetRequestCode()` |
| `request_code` | e.g. `CREATE_ORGANIZATION` (organization-style dispatch) |
| `status_code` | HTTP-like code mapped from gRPC status |
| `request_params` | `{ "body": <protojson>, "metadata": {...} }` |

### 3. Streaming RPCs

For client-streaming uploads (e.g. `UploadFile`):

- First message metadata is captured (filename, content_type, size)
- Chunk count and total bytes are logged
- Raw chunk bytes are **not** logged

### 4. Skipped methods (default)

- `/grpc.health.v1.Health/Check`
- `/grpc.reflection.v1.ServerReflection/ServerReflectionInfo`
- `/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo`

Override with `ClientConfig.SkipMethods`.

## What gets captured

| Field | Source |
|-------|--------|
| `service` | `ClientConfig.ServiceName` |
| `module` | First arg to `GroupWrapper` / `Wrap` |
| `handler` | Go method name (e.g. `GetByID`) |
| `method` | HTTP method |
| `path` | Request URL path |
| `route` | Echo route template (e.g. `/api/users/:id`) |
| `transport` | `"http"` or `"grpc"` |
| `request_code` | gRPC dispatch code when present |
| `status_code` | Response status |
| `duration_us` | Handler wall-clock time when under 1ms (microseconds) |
| `duration_ms` | Handler wall-clock time when 1ms or longer (milliseconds) |
| `request_params` | Sanitized query string, path params, and body |
| `response_body` | Sanitized JSON response on **success** (max 64KB) |
| `actor` | User context from Echo + response enrichment |
| `error` | Structured failure details on **error** responses (see below) |
| `trace_id` | W3C trace ID (32 hex chars); shared across hops in a distributed trace (v0.3.0+) |
| `span_id` | Span ID for this hop (16 hex chars) (v0.3.0+) |
| `parent_span_id` | Parent span ID from inbound `traceparent` (empty on trace root) (v0.3.0+) |
| `timestamp` | RFC3339 UTC |

### Error object (`error`)

On failed requests (`status_code >= 400` or handler returned an error), the event includes a nested `error` object instead of top-level `response_body`:

| Field | Source |
|-------|--------|
| `message` | Auto-captured from `logger.Error().Msg()` (v0.2.1+) or explicit `SetErrorMessage` / `LogError` |
| `cause` | Underlying returned error (`err.Error()` or gRPC status message) |
| `context` | Auto-captured from `logger.Error().Str()` fields (v0.2.1+) or explicit `SetErrorContext` / `LogError` |
| `response` | JSON error payload sent to the client (buffered body, `SetErrorResponse`, or `echo.HTTPError` map message) |

Import **`github.com/aprp19/pkg-nuha-log/logger`** (not raw `zerolog`) so Error logs inside a request are auto-captured by the accesslog interceptor/wrapper.

**Recommended handler pattern (no return wrapping):**

```go
logger.Error().
    Str("nik_pegawai", request.NIK).
    Str("document_path", path).
    Err(err).
    Msg("merge document file fetch failed")
return err
```

**When Echo's global HTTPErrorHandler writes the JSON after the wrapper** (common on 500):

```go
accesslog.SetErrorResponse(c, map[string]interface{}{
    "success": false,
    "message": "Internal Server Error",
    "meta":    meta,
})
```

**Example error event:**

```json
{
  "status_code": 500,
  "error": {
    "message": "merge document file fetch failed",
    "cause": "failed to stat file: The specified key does not exist.",
    "context": {
      "document_path": "/asset/file/doc.pdf",
      "nik_pegawai": "P-2024-01"
    },
    "response": {
      "success": false,
      "message": "Internal Server Error",
      "meta": {
        "status": 500,
        "service": "gateway-service"
      }
    }
  }
}
```

### Sensitive data redaction

These keys are replaced with `[REDACTED]` in both request and response:

`password`, `token`, `access_token`, `refresh_token`, `authorization`, `access_code`, `document_token`

### Response size limit

Responses larger than 64KB are truncated. The payload includes `"_truncated": true` so downstream consumers know data was cut.

## Actor fields

Actor identity is aligned with **nuha-auth** JWT claims (`userID`, `email`, `name`) and the context keys set by auth middleware in hub/CRM services.

| Actor JSON field | Primary source | Fallback |
|------------------|----------------|----------|
| `user_id` | Echo/gRPC context `userID` | JWT `claims` (`userID`, `sub`, `id`, `user_id`) |
| `user_email` | Context `email` | JWT `claims["email"]`, request body |
| `user_name` | Context `name` | JWT `claims["name"]` / `username`, response `data.user` |
| `user_uuid` | Response `data.user.uid` or `.uuid` | Not in JWT |
| `tenant_hub_id` | gRPC metadata `tenant-hub-id` / `x-tenant-hub-id` | — |
| `code_hospital` | Request body | Response `data.user.code_hospital` |
| `client_key` | Request body | — |

Auth middleware should set context before activity log capture:

```go
// nuha-auth HTTP middleware sets:
c.Set("claims", claims)
c.Set("userID", claims["userID"])
c.Set("email", claims["email"])

// CRM gateway ValidateSession sets:
c.Set("userID", resp.User.Id)   // string ID
c.Set("email", resp.User.Email)
```

The client also reads from request body when present:

- `email`, `client_key`, `code_hospital`, `identifier`

On success, actor fields can be enriched from the response shape:

```json
{ "data": { "user": { "email": "...", "name": "...", "uid": "...", "uuid": "..." } } }
```

## Example event payload

```json
{
  "service": "hub-user-service",
  "module": "user",
  "handler": "GetByID",
  "method": "GET",
  "path": "/api/users/42",
  "route": "/api/users/:id",
  "status_code": 200,
  "duration_ms": 38,
  "request_params": {
    "query": {
      "page": "1"
    },
    "path_params": {
      "id": "42"
    }
  },
  "response_body": {
    "success": true,
    "data": { "id": 42, "name": "Jane Doe" }
  },
  "actor": {
    "user_email": "admin@example.com"
  },
  "timestamp": "2026-09-02T10:00:00Z"
}
```

## Multiple modules in one service

Use a different module tag per controller:

```go
userWrap := logClient.GroupWrapper("user")
orgWrap  := logClient.GroupWrapper("organization")

api.Group("/users").GET("/:id", userWrap(ctrl.GetByID))
api.Group("/orgs").GET("/:id", orgWrap(orgCtrl.GetByID))
```

## Public vs protected routes

Wrap only the routes you want logged:

```go
// Public — no auth, still logged
api.GET("/health", wrap(ctrl.Health))

// Protected — auth middleware runs first, then logging wrapper
protected := api.Group("", authMiddleware)
protected.POST("/users", wrap(ctrl.Create))
```

Recommended middleware order:

```go
protected := api.Group("")
protected.Use(authMiddleware)       // sets user context first
protected.GET("/users/:id", wrap(ctrl.GetByID))  // wrap at route level
```

## Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| No logs in Redpanda | Check `HUB_INGESTION_URL`, hub-ingestion-service running, `INGEST_API_KEY` match |
| `activity log ingestion URL not configured` | `HUB_INGESTION_URL` is empty |
| `failed to send activity log` | Network error or hub-ingestion-service down — check service logs |
| Missing request body | Body must be readable; client caches body before handler runs |
| Missing actor fields | Auth middleware must set Echo context keys before handler |
| Missing error message in access log | Use `github.com/aprp19/pkg-nuha-log/logger` (not raw zerolog) and ensure accesslog interceptor/wrapper is registered |
| Missing 500 response JSON in access log | Echo HTTPErrorHandler may run after wrapper — use `SetErrorResponse` or write JSON before returning |

## API reference

```go
// Error capture (HTTP)
accesslog.LogError(c, err, "human message", "key", "value") // logs + sets access log error
accesslog.SetErrorMessage(c, "human message")
accesslog.SetErrorContext(c, "nik_pegawai", request.NIK)
accesslog.SetErrorResponse(c, errorPayload) // when global error handler writes outside wrapper

// Error capture (gRPC)
ctx = accesslog.SetErrorMessageContext(ctx, "human message")
ctx = accesslog.SetErrorContextContext(ctx, "key", "value")
```
// Create client
client := accesslog.NewClient(accesslog.ClientConfig{
    IngestionURL:     string  // required
    ServiceName:      string  // required — e.g. "hub-user-service"
    IngestAPIKey:     string  // optional
    MaxResponseBytes: int     // optional, default 65536
    HTTPClient:       *http.Client // optional, default 5s timeout
})

// Wrap all handlers in a module
wrap := client.GroupWrapper("module-name")
router.GET("/path", wrap(handler))

// Wrap a single handler
router.GET("/path", client.Wrap("module-name", handler))

// gRPC interceptors
unaryInterceptor := client.UnaryServerInterceptor("organization")
streamInterceptor := client.StreamServerInterceptor("organization")

// Distributed trace propagation (v0.3.0+)
traceClientInterceptor := accesslog.UnaryClientInterceptor() // attach on outbound gRPC dials
accesslog.InjectHTTPOutgoing(req)                          // attach on outbound HTTP requests
```

## Related

- **[AGENTS.md](../AGENTS.md)** — AI integration playbook
- **hub-ingestion-service** — receives events at `POST /api/logs`, publishes to Redpanda
- **nuha-backend reference** — `src/open-api/common/open-api-access-log.*` (PostgreSQL version)
