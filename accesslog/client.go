package accesslog

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

const defaultMaxResponseBytesConfig = 65536

// ActorExtractor reads actor fields from a context (optional override per service).
type ActorExtractor func(ctx context.Context) Actor

// ClientConfig configures the activity log client used by hub-*-services.
type ClientConfig struct {
	IngestionURL     string
	ServiceName      string
	IngestAPIKey     string
	MaxResponseBytes int
	HTTPClient       *http.Client
	SkipMethods      func(fullMethod string) bool
	RequestCodeField bool
	ActorExtractor   ActorExtractor
}

// Client captures activity logs and sends them to hub-ingestion-service.
type Client struct {
	ingestionURL     string
	serviceName      string
	ingestAPIKey     string
	maxResponseBytes int
	httpClient       *http.Client
	skipMethods      func(fullMethod string) bool
	requestCodeField bool
	actorExtractor   ActorExtractor
}

// NewClient creates an activity log client for producer services.
func NewClient(cfg ClientConfig) *Client {
	maxBytes := cfg.MaxResponseBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxResponseBytesConfig
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	skipFn := cfg.SkipMethods
	if skipFn == nil {
		skipFn = defaultSkipMethods
	}

	return &Client{
		ingestionURL:     strings.TrimRight(cfg.IngestionURL, "/"),
		serviceName:      cfg.ServiceName,
		ingestAPIKey:     cfg.IngestAPIKey,
		maxResponseBytes: maxBytes,
		httpClient:       client,
		skipMethods:      skipFn,
		requestCodeField: true,
		actorExtractor:   cfg.ActorExtractor,
	}
}

func defaultSkipMethods(fullMethod string) bool {
	switch fullMethod {
	case "/grpc.health.v1.Health/Check",
		"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo":
		return true
	default:
		return false
	}
}

func (c *Client) shouldSkip(fullMethod string) bool {
	if c.skipMethods == nil {
		return false
	}
	return c.skipMethods(fullMethod)
}

// GroupWrapper returns a function that wraps handlers with a fixed module tag.
func (c *Client) GroupWrapper(module string) func(echo.HandlerFunc) echo.HandlerFunc {
	return func(h echo.HandlerFunc) echo.HandlerFunc {
		return c.Wrap(module, h)
	}
}

// Wrap wraps a single Echo handler with activity log capture and async send.
func (c *Client) Wrap(module string, h echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		start := time.Now()
		cacheRequestBody(ctx)

		buf := newResponseBuffer(ctx.Response().Writer, c.maxResponseBytes)
		ctx.Response().Writer = buf

		handlerErr := h(ctx)

		statusCode := statusCodeFromContext(ctx, handlerErr)
		responseBody := buf.Body(c.maxResponseBytes)

		event := AccessLogEvent{
			Service:       c.serviceName,
			Module:        module,
			Handler:       handlerName(h),
			Method:        ctx.Request().Method,
			Path:          ctx.Request().URL.Path,
			Route:         ctx.Path(),
			Transport:     TransportHTTP,
			StatusCode:    statusCode,
			RequestParams: buildRequestParams(ctx),
			ResponseBody:  responseBodyIfSuccess(handlerErr, statusCode, responseBody),
			Error:         buildError(ctx, handlerErr, responseBody, statusCode),
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}
		setDurationFromStart(&event, start)

		actor := captureActorFromRequest(ctx)
		if handlerErr == nil {
			actor = enrichActorFromResponse(actor, event.ResponseBody)
		}
		event.Actor = actor

		c.sendAsync(event)
		return handlerErr
	}
}
