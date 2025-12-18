// Command opc-xml-da-cli calls an OPC XML-DA endpoint and prints GetStatus.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hooklift/gowsdl/soap"

	"opc-xml-da-cli/xmlda"
)

const maxDebugBodyBytes int64 = 64 * 1024

var netDebugSeq uint64

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run parses flags, performs the SOAP call, and prints the response.
func run() error {
	flag.CommandLine.SetOutput(os.Stderr)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -endpoint URL [options]\n       %s -endpoint URL -browse-path PATH [options]\n\n", os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}

	endpoint := flag.String("endpoint", "", "OPC XML-DA endpoint URL")
	browsePath := flag.String("browse-path", "", "OPC browse path (maps to ItemName)")
	browseItemPath := flag.String("browse-item-path", "", "OPC browse item path (maps to ItemPath)")
	browseDepth := flag.Int("browse-depth", 1, "Max browse depth (1 = direct children only)")
	netDebug := flag.Bool("net-debug", false, "Enable HTTP request/response debug logging")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	locale := flag.String("locale", "", "Locale ID (optional)")
	clientHandle := flag.String("client-handle", "", "Client request handle (optional)")
	httpTimeout := flag.Duration("http-timeout", 30*time.Second, "HTTP dial timeout")
	requestTimeout := flag.Duration("request-timeout", 90*time.Second, "End-to-end request timeout")
	username := flag.String("username", "", "Basic auth username (optional)")
	password := flag.String("password", "", "Basic auth password (optional)")
	flag.Parse()

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	if *netDebug {
		slog.Info("network debug enabled", "max_body_bytes", maxDebugBodyBytes)
	}

	if *endpoint == "" {
		flag.Usage()
		return fmt.Errorf("endpoint is required")
	}

	// Configure SOAP timeouts and optional basic auth.
	opts := []soap.Option{}
	if *netDebug {
		opts = append(opts, soap.WithHTTPClient(newDebugHTTPClient(*httpTimeout, *requestTimeout)))
	} else {
		opts = append(opts, soap.WithTimeout(*httpTimeout), soap.WithRequestTimeout(*requestTimeout))
	}
	if *username != "" {
		opts = append(opts, soap.WithBasicAuth(*username, *password))
	}

	mode := "status"
	if *browsePath != "" || *browseItemPath != "" {
		mode = "browse"
	}
	slog.Info("opc xml-da cli start", "mode", mode, "endpoint", *endpoint)
	slog.Debug("soap timeouts configured", "http_timeout", *httpTimeout, "request_timeout", *requestTimeout)

	client := soap.NewClient(*endpoint, opts...)
	service := xmlda.NewOpcXmlDASoap(client)

	req := &xmlda.GetStatus{
		LocaleID:            *locale,
		ClientRequestHandle: *clientHandle,
	}

	// Use a request-scoped context to bound the call.
	ctx := context.Background()
	if *requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *requestTimeout)
		defer cancel()
	}

	if *browsePath != "" || *browseItemPath != "" {
		if *browseDepth < 1 {
			return fmt.Errorf("browse-depth must be >= 1")
		}
		slog.Info("browse requested", "item_path", *browseItemPath, "item_name", *browsePath, "max_depth", *browseDepth)
		return browseOpcTree(ctx, service, *locale, *clientHandle, *browseItemPath, *browsePath, *browseDepth)
	}

	slog.Info("get status requested")
	resp, err := service.GetStatusContext(ctx, req)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	printStatus(resp)
	return nil
}

func browseOpcTree(ctx context.Context, service xmlda.OpcXmlDASoap, locale, clientHandle, itemPath, itemName string, maxDepth int) error {
	rootLabel := itemName
	if rootLabel == "" {
		rootLabel = itemPath
	}
	if rootLabel == "" {
		rootLabel = "<root>"
	}
	fmt.Println(rootLabel)
	if maxDepth <= 0 {
		return nil
	}

	visited := map[string]struct{}{
		makeBrowseKey(itemPath, itemName): {},
	}
	return browseOpcChildren(ctx, service, locale, clientHandle, itemPath, itemName, "  ", 1, maxDepth, visited)
}

func browseOpcChildren(ctx context.Context, service xmlda.OpcXmlDASoap, locale, clientHandle, itemPath, itemName, indent string, depth, maxDepth int, visited map[string]struct{}) error {
	elements, err := fetchBrowseElements(ctx, service, locale, clientHandle, itemPath, itemName)
	if err != nil {
		return err
	}

	sort.Slice(elements, func(i, j int) bool {
		return browseElementName(elements[i]) < browseElementName(elements[j])
	})

	for _, el := range elements {
		name := browseElementName(el)
		suffix := ""
		if el.HasChildren {
			suffix = "/"
		}
		fmt.Printf("%s%s%s\n", indent, name, suffix)

		if el.HasChildren && depth < maxDepth {
			key := makeBrowseKey(el.ItemPath, el.ItemName)
			if _, ok := visited[key]; ok {
				continue
			}
			visited[key] = struct{}{}
			if err := browseOpcChildren(ctx, service, locale, clientHandle, el.ItemPath, el.ItemName, indent+"  ", depth+1, maxDepth, visited); err != nil {
				return err
			}
		}
	}

	return nil
}

func fetchBrowseElements(ctx context.Context, service xmlda.OpcXmlDASoap, locale, clientHandle, itemPath, itemName string) ([]*xmlda.BrowseElement, error) {
	var all []*xmlda.BrowseElement
	continuation := ""
	filter := xmlda.BrowseFilterAll

	for {
		slog.Debug("browse page request", "item_path", itemPath, "item_name", itemName, "continuation", continuation)
		req := &xmlda.Browse{
			LocaleID:            locale,
			ClientRequestHandle: clientHandle,
			ItemPath:            itemPath,
			ItemName:            itemName,
			ContinuationPoint:   continuation,
			BrowseFilter:        &filter,
			ReturnErrorText:     true,
		}

		resp, err := service.BrowseContext(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("browse: %w", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("browse: empty response")
		}
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("browse: %s", formatOPCErrors(resp.Errors))
		}
		slog.Debug("browse page response", "elements", len(resp.Elements), "more_elements", resp.MoreElements)
		all = append(all, resp.Elements...)

		if !resp.MoreElements || resp.ContinuationPoint == "" {
			break
		}
		continuation = resp.ContinuationPoint
	}

	return all, nil
}

func browseElementName(el *xmlda.BrowseElement) string {
	if el == nil {
		return "<nil>"
	}
	if el.Name != "" {
		return el.Name
	}
	if el.ItemName != "" {
		return el.ItemName
	}
	if el.ItemPath != "" {
		return el.ItemPath
	}
	return "<unnamed>"
}

func makeBrowseKey(itemPath, itemName string) string {
	return itemPath + "\x00" + itemName
}

func formatOPCErrors(errors []*xmlda.OPCError) string {
	if len(errors) == 0 {
		return "unknown error"
	}

	parts := make([]string, 0, len(errors))
	for _, err := range errors {
		if err == nil {
			continue
		}
		if err.ID != nil && *err.ID != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", *err.ID, err.Text))
			continue
		}
		if err.Text != "" {
			parts = append(parts, err.Text)
		}
	}
	if len(parts) == 0 {
		return "unknown error"
	}
	return strings.Join(parts, "; ")
}

// printStatus renders the response fields in a readable format.
func printStatus(resp *xmlda.GetStatusResponse) {
	if resp == nil {
		fmt.Println("no response")
		return
	}

	if resp.GetStatusResult != nil {
		fmt.Println("GetStatusResult:")
		if resp.GetStatusResult.ServerState != nil {
			fmt.Printf("  ServerState: %s\n", *resp.GetStatusResult.ServerState)
		}
		if resp.GetStatusResult.RevisedLocaleID != "" {
			fmt.Printf("  RevisedLocaleID: %s\n", resp.GetStatusResult.RevisedLocaleID)
		}
		if resp.GetStatusResult.ClientRequestHandle != "" {
			fmt.Printf("  ClientRequestHandle: %s\n", resp.GetStatusResult.ClientRequestHandle)
		}
		if t := formatXsdDateTime(resp.GetStatusResult.ReplyTime); t != "" {
			fmt.Printf("  ReplyTime: %s\n", t)
		}
		if t := formatXsdDateTime(resp.GetStatusResult.RcvTime); t != "" {
			fmt.Printf("  ReceiveTime: %s\n", t)
		}
	}

	if resp.Status != nil {
		fmt.Println("Status:")
		if resp.Status.StatusInfo != "" {
			fmt.Printf("  StatusInfo: %s\n", resp.Status.StatusInfo)
		}
		if resp.Status.VendorInfo != "" {
			fmt.Printf("  VendorInfo: %s\n", resp.Status.VendorInfo)
		}
		if resp.Status.ProductVersion != "" {
			fmt.Printf("  ProductVersion: %s\n", resp.Status.ProductVersion)
		}
		if t := formatXsdDateTime(resp.Status.StartTime); t != "" {
			fmt.Printf("  StartTime: %s\n", t)
		}
		if len(resp.Status.SupportedLocaleIDs) > 0 {
			fmt.Printf("  SupportedLocaleIDs: %s\n", strings.Join(resp.Status.SupportedLocaleIDs, ", "))
		}
		if len(resp.Status.SupportedInterfaceVersions) > 0 {
			versions := make([]string, 0, len(resp.Status.SupportedInterfaceVersions))
			for _, v := range resp.Status.SupportedInterfaceVersions {
				if v == nil {
					continue
				}
				versions = append(versions, string(*v))
			}
			if len(versions) > 0 {
				fmt.Printf("  SupportedInterfaceVersions: %s\n", strings.Join(versions, ", "))
			}
		}
	}
}

// formatXsdDateTime converts the SOAP datetime to RFC3339 when set.
func formatXsdDateTime(dt xmlda.XSDDateTime) string {
	t := dt.ToGoTime()
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log-level: %q", value)
	}
}

func newDebugHTTPClient(httpTimeout, requestTimeout time.Duration) *http.Client {
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
			maxBodyBytes: maxDebugBodyBytes,
			logger:       slog.Default().With("component", "net"),
		},
		Timeout: requestTimeout,
	}
}

type loggingRoundTripper struct {
	base         http.RoundTripper
	maxBodyBytes int64
	logger       *slog.Logger
}

func (rt *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.base == nil {
		rt.base = http.DefaultTransport
	}
	id := atomic.AddUint64(&netDebugSeq, 1)
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
	resp, err := rt.base.RoundTrip(req)
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
