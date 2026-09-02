# pkg-nuha-log

Shared Go module for **activity logging** in Nuha Hub / CRM services. Captures HTTP (Echo) and gRPC request/response metadata and sends events async to **hub-ingestion-service** (`POST /api/logs`).

> **For AI-assisted integration**, read **[AGENTS.md](AGENTS.md)** first.

## Install

This module is hosted on private GitLab. Configure Go once per machine:

```bash
go env -w GOPRIVATE=10.100.2.133
go env -w GONOSUMDB=10.100.2.133
```

If authentication is required:

```bash
git config --global url."http://<user>:<token>@10.100.2.133/".insteadOf "http://10.100.2.133/"
```

In your service:

```bash
go get 10.100.2.133/nuha-hub/pkg-nuha-log@v0.1.0
```

```go
import "10.100.2.133/nuha-hub/pkg-nuha-log/accesslog"
```

## Quick start

```go
logClient := accesslog.NewClient(accesslog.ClientConfig{
    IngestionURL: os.Getenv("HUB_INGESTION_URL"),
    ServiceName:  "hub-user-service",
    IngestAPIKey: os.Getenv("INGEST_API_KEY"),
})

wrap := logClient.GroupWrapper("user")
api.GET("/users/:id", wrap(ctrl.GetByID))
```

gRPC (auth outer, activity log inner):

```go
grpc.ChainUnaryInterceptor(
    middleware.GRPCAuthInterceptor(cfg),
    logClient.UnaryServerInterceptor("organization"),
)
```

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `HUB_INGESTION_URL` | yes | Base URL of hub-ingestion-service |
| `INGEST_API_KEY` | no | Sent as `X-Ingest-Key` header |

## Updating the library

When this repo is updated, tag a release and pull in consuming services:

```bash
go get 10.100.2.133/nuha-hub/pkg-nuha-log@v0.1.1
```

## Repository layout

```
accesslog/   Activity log client (HTTP + gRPC)
logger/      Internal zerolog wrapper (used by accesslog)
```

## Related

- **[AGENTS.md](AGENTS.md)** — integration playbook for hub/CRM services
- **hub-ingestion-service** — ingestion gateway → Redpanda
- Remote: `http://10.100.2.133/nuha-hub/pkg-nuha-log.git`

See [accesslog/README.md](accesslog/README.md) for the full human reference (config, actor fields, troubleshooting).
