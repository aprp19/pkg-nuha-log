package accesslog

import (
	"bytes"
	"net/http"
)

const defaultMaxResponseBytes = 65536

type responseBuffer struct {
	http.ResponseWriter
	buf        bytes.Buffer
	maxBytes   int
	statusCode int
}

func newResponseBuffer(w http.ResponseWriter, maxBytes int) *responseBuffer {
	if maxBytes <= 0 {
		maxBytes = defaultMaxResponseBytes
	}
	return &responseBuffer{
		ResponseWriter: w,
		maxBytes:       maxBytes,
		statusCode:     http.StatusOK,
	}
}

func (b *responseBuffer) Write(p []byte) (int, error) {
	if b.buf.Len() < b.maxBytes {
		remaining := b.maxBytes - b.buf.Len()
		if len(p) > remaining {
			_, _ = b.buf.Write(p[:remaining])
		} else {
			_, _ = b.buf.Write(p)
		}
	}
	return b.ResponseWriter.Write(p)
}

func (b *responseBuffer) WriteHeader(statusCode int) {
	b.statusCode = statusCode
	b.ResponseWriter.WriteHeader(statusCode)
}

func (b *responseBuffer) Body(maxBytes int) interface{} {
	return sanitizeResponseBody(b.buf.Bytes(), maxBytes)
}
