// Command opc-xml-da-cli calls an OPC XML-DA endpoint and prints GetStatus.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/hooklift/gowsdl/soap"

	"opc-xml-da-cli/internal/cli"
	"opc-xml-da-cli/service"
)

const (
	defaultBrowseDepth    = 1
	defaultHTTPTimeout    = 30 * time.Second
	defaultRequestTimeout = 90 * time.Second
	defaultLogLevel       = "info"
)

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
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -endpoint URL [options]\n       %s -endpoint URL -browse-path PATH [options]\n       %s -endpoint URL -read-path PATH [options]\n\n", os.Args[0], os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}

	endpoint := flag.String("endpoint", "", "OPC XML-DA endpoint URL")
	browsePath := flag.String("browse-path", "", "OPC browse path (maps to ItemName)")
	browseItemPath := flag.String("browse-item-path", "", "OPC browse item path (maps to ItemPath)")
	browseDepth := flag.Int("browse-depth", defaultBrowseDepth, "Max browse depth (1 = direct children only)")
	readPath := flag.String("read-path", "", "OPC read item name (maps to ItemName)")
	readItemPath := flag.String("read-item-path", "", "OPC read item path (maps to ItemPath)")
	netDebug := flag.Bool("net-debug", false, "Enable HTTP request/response debug logging")
	logLevel := flag.String("log-level", defaultLogLevel, "Log level (debug, info, warn, error)")
	locale := flag.String("locale", "", "Locale ID (optional)")
	clientHandle := flag.String("client-handle", "", "Client request handle (optional)")
	httpTimeout := flag.Duration("http-timeout", defaultHTTPTimeout, "HTTP dial timeout")
	requestTimeout := flag.Duration("request-timeout", defaultRequestTimeout, "End-to-end request timeout")
	username := flag.String("username", "", "Basic auth username (optional)")
	password := flag.String("password", "", "Basic auth password (optional)")
	flag.Parse()

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	if *netDebug {
		slog.Info("network debug enabled", "max_body_bytes", cli.MaxDebugBodyBytes)
	}

	if *endpoint == "" {
		flag.Usage()
		return fmt.Errorf("endpoint is required")
	}

	// Configure SOAP timeouts and optional basic auth.
	var opts []soap.Option
	if *netDebug {
		opts = append(opts, soap.WithHTTPClient(cli.NewDebugHTTPClient(*httpTimeout, *requestTimeout)))
	} else {
		opts = append(opts, soap.WithTimeout(*httpTimeout), soap.WithRequestTimeout(*requestTimeout))
	}
	if *username != "" {
		opts = append(opts, soap.WithBasicAuth(*username, *password))
	}

	browseRequested := *browsePath != "" || *browseItemPath != ""
	readRequested := *readPath != "" || *readItemPath != ""
	if browseRequested && readRequested {
		return fmt.Errorf("choose either browse or read options, not both")
	}
	mode := "status"
	if browseRequested {
		mode = "browse"
	}
	if readRequested {
		mode = "read"
	}
	slog.Info("opc xml-da cli start", "mode", mode, "endpoint", *endpoint)
	slog.Debug("soap timeouts configured", "http_timeout", *httpTimeout, "request_timeout", *requestTimeout)

	client := soap.NewClient(*endpoint, opts...)
	opcService := service.NewOpcXmlDASoap(client)

	// Use a request-scoped context to bound the call.
	ctx := context.Background()
	if *requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *requestTimeout)
		defer cancel()
	}

	if browseRequested {
		if *browseDepth < 1 {
			return fmt.Errorf("browse-depth must be >= 1")
		}
		slog.Info("browse requested", "item_path", *browseItemPath, "item_name", *browsePath, "max_depth", *browseDepth)
		return cli.BrowseOpcTree(ctx, os.Stdout, opcService, *locale, *clientHandle, *browseItemPath, *browsePath, *browseDepth)
	}

	if readRequested {
		slog.Info("read requested", "item_path", *readItemPath, "item_name", *readPath)
		resp, err := cli.FetchNodeValue(ctx, opcService, *locale, *clientHandle, *readItemPath, *readPath)
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		if err := cli.PrintRead(os.Stdout, resp); err != nil {
			return fmt.Errorf("print read: %w", err)
		}
		return nil
	}

	slog.Info("get status requested")
	resp, err := cli.FetchServerStatus(ctx, opcService, *locale, *clientHandle)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	if err := cli.PrintStatus(os.Stdout, resp); err != nil {
		return fmt.Errorf("print status: %w", err)
	}
	return nil
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
