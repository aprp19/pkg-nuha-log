package accesslog

import (
	"bytes"
	"context"
	"net/http"

	"10.100.2.133/nuha-hub/pkg-nuha-log/logger"
)

func (c *Client) sendAsync(event AccessLogEvent) {
	if c.ingestionURL == "" {
		logger.Warn().Msg("activity log ingestion URL not configured; skipping send")
		return
	}

	go func(ev AccessLogEvent) {
		if err := c.send(context.Background(), ev); err != nil {
			logger.Error().Err(err).Str("service", ev.Service).Str("path", ev.Path).Msg("failed to send activity log")
		}
	}(event)
}

func (c *Client) send(ctx context.Context, event AccessLogEvent) error {
	payload, err := jsonMarshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ingestionURL+"/api/logs", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.ingestAPIKey != "" {
		req.Header.Set("X-Ingest-Key", c.ingestAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		logger.Warn().Int("status", resp.StatusCode).Str("path", event.Path).Msg("activity log ingest returned non-success status")
	}
	return nil
}
