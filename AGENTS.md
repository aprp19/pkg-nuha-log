# AGENTS.md — Activity log integration playbook

**Read this file when adding activity logging to another service** (crm-gateway-service, crm-organization-service, hub-*-services).

For human-readable reference, see [README.md](README.md) and [accesslog/README.md](accesslog/README.md).

## When to use this

Add this module to the target service via `go get`. Wire HTTP and/or gRPC interceptors. Changes to the client happen **in this repo** — tag releases and update consumers with `go get ...@vX.Y.Z`.

## Prerequisites

- Go 1.21+
- hub-ingestion-service running and reachable
- Env vars: `HUB_INGESTION_URL`, optional `INGEST_API_KEY`
- Target service has Echo (HTTP) and/or gRPC server
- Git access to `https://github.com/aprp19/pkg-nuha-log.git`

## Install checklist

1. Add the module to the target service:

   ```bash
   go get github.com/aprp19/pkg-nuha-log@v0.1.0
   ```

2. Import in Go code:

   ```go
   import "github.com/aprp19/pkg-nuha-log/accesslog"
   ```

3. Run `go mod tidy && go build ./...`

## HTTP integration (Echo)

### 1. Initialize client at startup

```go
logClient := accesslog.NewClient(accesslog.ClientConfig{
    IngestionURL: os.Getenv("HUB_INGESTION_URL"),
    ServiceName:  "crm-organization-service",
    IngestAPIKey: os.Getenv("INGEST_API_KEY"),
})
```

### 2. Wrap routes per module

```go
wrap := logClient.GroupWrapper("organization")
api.GET("/users/:id", wrap(ctrl.GetByID))
```

### 3. Middleware order

Auth middleware must run **before** the activity log wrapper so actor context is set:

```go
protected := api.Group("")
protected.Use(authMiddleware)  // sets userID, email, name in context
protected.GET("/users/:id", wrap(ctrl.GetByID))
```

## gRPC integration (unary + stream)

**Order matters:** auth outer, activity log inner.

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

### Module naming

| Service | Module tag |
|---------|------------|
| crm-gateway-service | `"gateway"` |
| crm-organization-service | `"organization"` |
| hub-user-service | `"user"` |

## CRM-specific patterns

### Gateway (crm-gateway-service)

Register in `internal/bootstrap/grpc.go` with module `"gateway"`.

### Organization dispatch RPC (request_code)

For messages implementing `GetRequestCode() string`:

- Event `request_code` is set automatically (e.g. `CREATE_ORGANIZATION`)
- Event `handler` becomes `ConnectOrganizationCrm/CREATE_ORGANIZATION`

### Streaming uploads (UploadFile)

- First message metadata captured; chunk count + total bytes logged
- Raw chunk bytes are **never** logged

## Event contract

Events POST to `{HUB_INGESTION_URL}/api/logs` as JSON (`AccessLogEvent`). See [accesslog/README.md](accesslog/README.md) for field reference.

Sensitive keys redacted: `password`, `token`, `access_token`, `refresh_token`, `authorization`, `access_code`, `document_token`.

## Structured error capture

Failed requests emit a nested `error` object with `message`, `cause`, `context`, and `response` instead of flat `error_message`. Success responses still use top-level `response_body`.

**Auto-capture (v0.2.1+):** use `github.com/aprp19/pkg-nuha-log/logger` — existing `logger.Error().Str(...).Msg(...)` before `return err` fills `error.message` and `error.context` automatically when the accesslog interceptor/wrapper is registered. No per-return helpers required.

```go
logger.Error().
    Str("nik_pegawai", request.NIK).
    Str("document_path", path).
    Err(err).
    Msg("merge document file fetch failed")
return err
```

Explicit opt-in still works: `accesslog.LogError`, `SetErrorMessage`, `SetErrorResponse` (HTTP), `SetErrorMessageContext` (gRPC).

When Echo's global HTTPErrorHandler formats 500 JSON outside the wrapper, call `accesslog.SetErrorResponse(c, payload)` from the error handler.

**Migration from v0.1.x:** `error_message` → `error.cause` / `error.message`; error JSON → `error.response`.

## Release checklist

On every **major** release (changes to `AccessLogEvent` / `AccessLogError` in `accesslog/event.go`, or semver minor/major bump):

1. Tag and push pkg-nuha-log (e.g. `v0.2.0`)
2. **Always bump hub-ingestion-service and hub-consumer-service** — both import `AccessLogEvent` directly; an older version silently drops new JSON fields at bind time
   ```bash
   cd hub-ingestion-service
   go get github.com/aprp19/pkg-nuha-log@vX.Y.Z
   go mod tidy && go build ./...

   cd ../hub-consumer-service
   go get github.com/aprp19/pkg-nuha-log@vX.Y.Z
   go mod tidy && go build ./...
   ```
3. Update hub-ingestion-service and hub-consumer-service README/AGENTS/setup scripts to match the new contract
4. Deploy hub-ingestion-service and hub-consumer-service **before or with** producer service upgrades
5. Verify `POST /api/logs` accepts a sample event with new fields → `202 Accepted`
6. Verify consumer persists nested `error` object to MongoDB (or run `go test ./...` in hub-consumer-service)

Patch releases that only fix producer-side behavior (no JSON shape change) may skip the ingestion/consumer bumps.

Producer services can use `go get github.com/aprp19/pkg-nuha-log@latest` after tagging.

## Do NOT

- Talk to Redpanda directly from producer services
- Block handlers waiting for ingest response (send is async fire-and-forget)
- Log raw file chunk content in streaming RPCs
- Put business logic inside interceptors

## Verification checklist

- [ ] `go build ./...` passes in target service
- [ ] `HUB_INGESTION_URL` points to running hub-ingestion-service
- [ ] HTTP and/or gRPC call produces `POST /api/logs` → `202 Accepted`
- [ ] Event has `service`, `module`, `handler`, `path`, `transport`

## Actor extraction (nuha-auth aligned)

Actor fields follow nuha-auth JWT claims (`userID`, `email`, `name`) and auth middleware context. UUID comes from response enrichment (`data.user.uid` or `.uuid`), not JWT.

Override with `ClientConfig.ActorExtractor` for custom context keys.
