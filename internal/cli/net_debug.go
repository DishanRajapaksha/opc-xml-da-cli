package cli

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
	"time"
)

// MaxDebugBodyBytes caps the number of bytes captured for HTTP debug logs.
const MaxDebugBodyBytes int64 = 64 * 1024

// NewDebugHTTPClient builds an HTTP client that logs request/response details.
func NewDebugHTTPClient(httpTimeout, requestTimeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if httpTimeout > 0 {
		transport.DialContext = (&net.Dialer{
			Timeout:   httpTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext
		transport.TLSHandshakeTimeout = httpTimeout
	}
	return &http.Client{
		Transport: &loggingRoundTripper{
			base:         transport,
			maxBodyBytes: MaxDebugBodyBytes,
			logger:       slog.Default().With("component", "net"),
		},
		Timeout: requestTimeout,
	}
}

type loggingRoundTripper struct {
	base         http.RoundTripper
	maxBodyBytes int64
	logger       *slog.Logger
	seq          uint64
}

func (rt *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	id := atomic.AddUint64(&rt.seq, 1)
	start := time.Now()
	logger := rt.logger
	if logger == nil {
		logger = slog.Default().With("component", "net")
	}

	reqHeaders := redactHeaders(req.Header)
	reqBodySize := 0
	reqBodyPreview := ""
	reqBodyTruncated := false
	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		reqBodySize = len(bodyBytes)
		reqBodyPreview, reqBodyTruncated = bodyPreview(bodyBytes, rt.maxBodyBytes)
	}

	logger.Info("http request",
		"id", id,
		"method", req.Method,
		"url", req.URL.String(),
		"headers", reqHeaders,
		"content_length", req.ContentLength,
		"body_size", reqBodySize,
		"body_truncated", reqBodyTruncated,
		"body_preview", reqBodyPreview,
	)

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			logger.Info("http trace dns start", "id", id, "host", info.Host)
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			logger.Info("http trace dns done", "id", id, "addrs", formatAddrs(info.Addrs), "coalesced", info.Coalesced, "err", info.Err)
		},
		ConnectStart: func(network, addr string) {
			logger.Info("http trace connect start", "id", id, "network", network, "addr", addr)
		},
		ConnectDone: func(network, addr string, err error) {
			logger.Info("http trace connect done", "id", id, "network", network, "addr", addr, "err", err)
		},
		TLSHandshakeStart: func() {
			logger.Info("http trace tls handshake start", "id", id)
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			logger.Info(
				"http trace tls handshake done",
				"id", id,
				"version", tlsVersion(state.Version),
				"server_name", state.ServerName,
				"negotiated_protocol", state.NegotiatedProtocol,
				"cipher_suite", tls.CipherSuiteName(state.CipherSuite),
				"err", err,
			)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			logger.Info("http trace got conn", "id", id, "reused", info.Reused, "was_idle", info.WasIdle, "idle_time", info.IdleTime)
		},
		WroteHeaders: func() {
			logger.Info("http trace wrote headers", "id", id)
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			logger.Info("http trace wrote request", "id", id, "err", info.Err)
		},
		GotFirstResponseByte: func() {
			logger.Info("http trace first response byte", "id", id, "elapsed", time.Since(start))
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := base.RoundTrip(req)
	if err != nil {
		logger.Info("http response error", "id", id, "err", err, "elapsed", time.Since(start))
		return nil, err
	}

	respHeaders := redactHeaders(resp.Header)
	logger.Info("http response",
		"id", id,
		"status", resp.Status,
		"status_code", resp.StatusCode,
		"headers", respHeaders,
		"content_length", resp.ContentLength,
		"elapsed", time.Since(start),
	)

	if resp.Body != nil && resp.Body != http.NoBody {
		resp.Body = &loggedBody{
			ReadCloser:   resp.Body,
			maxBodyBytes: rt.maxBodyBytes,
			logger:       logger,
			id:           id,
			start:        start,
		}
	}
	return resp, nil
}

type loggedBody struct {
	io.ReadCloser
	maxBodyBytes int64
	logger       *slog.Logger
	id           uint64
	start        time.Time
	buf          bytes.Buffer
	truncated    bool
	readBytes    int64
}

func (lb *loggedBody) Read(p []byte) (int, error) {
	n, err := lb.ReadCloser.Read(p)
	if n > 0 {
		lb.readBytes += int64(n)
		remaining := int(lb.maxBodyBytes) - lb.buf.Len()
		if remaining > 0 {
			if n > remaining {
				lb.buf.Write(p[:remaining])
				lb.truncated = true
			} else {
				lb.buf.Write(p[:n])
			}
		} else {
			lb.truncated = true
		}
	}
	return n, err
}

func (lb *loggedBody) Close() error {
	err := lb.ReadCloser.Close()
	if lb.logger == nil {
		lb.logger = slog.Default().With("component", "net")
	}
	lb.logger.Info(
		"http response body",
		"id", lb.id,
		"bytes_read", lb.readBytes,
		"body_truncated", lb.truncated,
		"body_preview", lb.buf.String(),
		"elapsed", time.Since(lb.start),
	)
	return err
}

func redactHeaders(headers http.Header) map[string][]string {
	redacted := make(map[string][]string, len(headers))
	for key, values := range headers {
		if isSensitiveHeader(key) {
			redacted[key] = []string{"<redacted>"}
			continue
		}
		copied := make([]string, len(values))
		copy(copied, values)
		redacted[key] = copied
	}
	return redacted
}

func isSensitiveHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie":
		return true
	default:
		return false
	}
}

func bodyPreview(body []byte, limit int64) (string, bool) {
	if limit <= 0 {
		return "", len(body) > 0
	}
	if int64(len(body)) <= limit {
		return string(body), false
	}
	return string(body[:limit]), true
}

func formatAddrs(addrs []net.IPAddr) []string {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.String())
	}
	return out
}

func tlsVersion(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS1.3"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS10:
		return "TLS1.0"
	default:
		return fmt.Sprintf("0x%x", version)
	}
}
