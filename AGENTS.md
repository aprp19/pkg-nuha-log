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
- Git access to `http://10.100.2.133/nuha-hub/pkg-nuha-log.git`

## Install checklist

1. Configure private module access (once per machine):

   ```bash
   go env -w GOPRIVATE=10.100.2.133
   go env -w GONOSUMDB=10.100.2.133
   ```

2. Add the module to the target service:

   ```bash
   go get 10.100.2.133/nuha-hub/pkg-nuha-log@v0.1.0
   ```

3. Import in Go code:

   ```go
   import "10.100.2.133/nuha-hub/pkg-nuha-log/accesslog"
   ```

4. Run `go mod tidy && go build ./...`

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
