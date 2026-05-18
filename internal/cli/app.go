package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/hooklift/gowsdl/soap"

	"opc-xml-da-cli/service"
)

const (
	appName               = "opc-xml-da-cli"
	defaultBrowseDepth    = 1
	defaultHTTPTimeout    = 30 * time.Second
	defaultRequestTimeout = 90 * time.Second
	defaultLogLevel       = "info"
	exitSuccess           = 0
	exitGeneralError      = 1
)

type App struct {
	out io.Writer
	err io.Writer
}

type commandOptions struct {
	Endpoint       string
	BrowsePath     string
	BrowseItemPath string
	BrowseDepth    int
	ReadPath       string
	ReadItemPath   string
	NetDebug       bool
	LogLevel       string
	Locale         string
	ClientHandle   string
	HTTPTimeout    time.Duration
	RequestTimeout time.Duration
	Username       string
	Password       string
}

func defaultCommandOptions() commandOptions {
	return commandOptions{
		BrowseDepth:    defaultBrowseDepth,
		LogLevel:       defaultLogLevel,
		HTTPTimeout:    defaultHTTPTimeout,
		RequestTimeout: defaultRequestTimeout,
	}
}

func NewApp(out io.Writer, err io.Writer) *App {
	return &App{out: out, err: err}
}

func Main() {
	code := NewApp(os.Stdout, os.Stderr).Run(os.Args[1:])
	if code != 0 {
		os.Exit(code)
	}
}

func (a *App) Run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		a.printUsage()
		return exitSuccess
	}

	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		fmt.Fprintf(a.out, "%s development\n", appName)
		return exitSuccess
	}

	var err error
	switch args[0] {
	case "status":
		err = a.status(args[1:])
	case "browse":
		err = a.browse(args[1:])
	case "read":
		err = a.read(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			err = a.runLegacy(args)
			break
		}
		a.printUsage()
		fmt.Fprintf(a.err, "unknown command %q\n", args[0])
		return exitGeneralError
	}

	if err != nil {
		fmt.Fprintln(a.err, err)
		return exitGeneralError
	}
	return exitSuccess
}

func (a *App) status(args []string) error {
	opts := defaultCommandOptions()
	fs := a.newFlagSet("status")
	addCommonFlags(fs, &opts)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return a.runStatus(opts)
}

func (a *App) browse(args []string) error {
	opts := defaultCommandOptions()
	fs := a.newFlagSet("browse")
	addCommonFlags(fs, &opts)
	fs.StringVar(&opts.BrowsePath, "browse-path", "", "OPC browse path (maps to ItemName)")
	fs.StringVar(&opts.BrowseItemPath, "browse-item-path", "", "OPC browse item path (maps to ItemPath)")
	fs.IntVar(&opts.BrowseDepth, "browse-depth", opts.BrowseDepth, "max browse depth (1 = direct children only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return a.runBrowse(opts)
}

func (a *App) read(args []string) error {
	opts := defaultCommandOptions()
	fs := a.newFlagSet("read")
	addCommonFlags(fs, &opts)
	fs.StringVar(&opts.ReadPath, "read-path", "", "OPC read item name (maps to ItemName)")
	fs.StringVar(&opts.ReadItemPath, "read-item-path", "", "OPC read item path (maps to ItemPath)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return a.runRead(opts)
}

func (a *App) runLegacy(args []string) error {
	opts := defaultCommandOptions()
	fs := a.newFlagSet(appName)
	addCommonFlags(fs, &opts)
	fs.StringVar(&opts.BrowsePath, "browse-path", "", "OPC browse path (maps to ItemName)")
	fs.StringVar(&opts.BrowseItemPath, "browse-item-path", "", "OPC browse item path (maps to ItemPath)")
	fs.IntVar(&opts.BrowseDepth, "browse-depth", opts.BrowseDepth, "max browse depth (1 = direct children only)")
	fs.StringVar(&opts.ReadPath, "read-path", "", "OPC read item name (maps to ItemName)")
	fs.StringVar(&opts.ReadItemPath, "read-item-path", "", "OPC read item path (maps to ItemPath)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	browseRequested := opts.BrowsePath != "" || opts.BrowseItemPath != ""
	readRequested := opts.ReadPath != "" || opts.ReadItemPath != ""
	if browseRequested && readRequested {
		return fmt.Errorf("choose either browse or read options, not both")
	}
	if browseRequested {
		return a.runBrowse(opts)
	}
	if readRequested {
		return a.runRead(opts)
	}
	return a.runStatus(opts)
}

func (a *App) runStatus(opts commandOptions) error {
	ctx, opcService, err := a.newService(opts)
	if err != nil {
		return err
	}
	slog.Info("get status requested")
	resp, err := FetchServerStatus(ctx, opcService, opts.Locale, opts.ClientHandle)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}
	if err := PrintStatus(a.out, resp); err != nil {
		return fmt.Errorf("print status: %w", err)
	}
	return nil
}

func (a *App) runBrowse(opts commandOptions) error {
	if opts.BrowseDepth < 1 {
		return fmt.Errorf("browse-depth must be >= 1")
	}
	ctx, opcService, err := a.newService(opts)
	if err != nil {
		return err
	}
	slog.Info("browse requested", "item_path", opts.BrowseItemPath, "item_name", opts.BrowsePath, "max_depth", opts.BrowseDepth)
	return BrowseOpcTree(ctx, a.out, opcService, opts.Locale, opts.ClientHandle, opts.BrowseItemPath, opts.BrowsePath, opts.BrowseDepth)
}

func (a *App) runRead(opts commandOptions) error {
	ctx, opcService, err := a.newService(opts)
	if err != nil {
		return err
	}
	slog.Info("read requested", "item_path", opts.ReadItemPath, "item_name", opts.ReadPath)
	resp, err := FetchNodeValue(ctx, opcService, opts.Locale, opts.ClientHandle, opts.ReadItemPath, opts.ReadPath)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	if err := PrintRead(a.out, resp); err != nil {
		return fmt.Errorf("print read: %w", err)
	}
	return nil
}

func (a *App) newService(opts commandOptions) (context.Context, service.OpcXmlDASoap, error) {
	if err := configureLogging(opts.LogLevel, opts.NetDebug); err != nil {
		return nil, nil, err
	}
	if opts.Endpoint == "" {
		return nil, nil, fmt.Errorf("endpoint is required")
	}

	var soapOpts []soap.Option
	if opts.NetDebug {
		soapOpts = append(soapOpts, soap.WithHTTPClient(NewDebugHTTPClient(opts.HTTPTimeout, opts.RequestTimeout)))
	} else {
		soapOpts = append(soapOpts, soap.WithTimeout(opts.HTTPTimeout), soap.WithRequestTimeout(opts.RequestTimeout))
	}
	if opts.Username != "" {
		soapOpts = append(soapOpts, soap.WithBasicAuth(opts.Username, opts.Password))
	}

	ctx := context.Background()
	if opts.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.RequestTimeout)
		_ = cancel
	}

	slog.Info("opc xml-da cli start", "endpoint", opts.Endpoint)
	slog.Debug("soap timeouts configured", "http_timeout", opts.HTTPTimeout, "request_timeout", opts.RequestTimeout)
	client := soap.NewClient(opts.Endpoint, soapOpts...)
	return ctx, service.NewOpcXmlDASoap(client), nil
}

func (a *App) newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(a.err)
	return fs
}

func addCommonFlags(fs *flag.FlagSet, opts *commandOptions) {
	fs.StringVar(&opts.Endpoint, "endpoint", opts.Endpoint, "OPC XML-DA endpoint URL")
	fs.BoolVar(&opts.NetDebug, "net-debug", opts.NetDebug, "enable HTTP request/response debug logging")
	fs.StringVar(&opts.LogLevel, "log-level", opts.LogLevel, "log level: debug, info, warn, error")
	fs.StringVar(&opts.Locale, "locale", opts.Locale, "locale ID")
	fs.StringVar(&opts.ClientHandle, "client-handle", opts.ClientHandle, "client request handle")
	fs.DurationVar(&opts.HTTPTimeout, "http-timeout", opts.HTTPTimeout, "HTTP dial timeout")
	fs.DurationVar(&opts.RequestTimeout, "request-timeout", opts.RequestTimeout, "end-to-end request timeout")
	fs.StringVar(&opts.Username, "username", opts.Username, "Basic auth username")
	fs.StringVar(&opts.Password, "password", opts.Password, "Basic auth password")
}

func configureLogging(logLevel string, netDebug bool) error {
	level, err := parseLogLevel(logLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	if netDebug {
		slog.Info("network debug enabled", "max_body_bytes", MaxDebugBodyBytes)
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

func (a *App) printUsage() {
	fmt.Fprintln(a.out, `opc-xml-da-cli is a small OPC XML-DA command-line client.

Usage:
  opc-xml-da-cli status --endpoint URL
  opc-xml-da-cli browse --endpoint URL --browse-path PATH --browse-depth 1
  opc-xml-da-cli read --endpoint URL --read-path PATH
  opc-xml-da-cli version

Commands:
  status    Fetch OPC XML-DA GetStatus
  browse    Browse OPC XML-DA items
  read      Read an OPC XML-DA item
  version   Print version information

Common flags:
  --endpoint          OPC XML-DA endpoint URL
  --locale            Locale ID
  --client-handle     Client request handle
  --http-timeout      HTTP dial timeout
  --request-timeout   End-to-end request timeout
  --username          Basic auth username
  --password          Basic auth password
  --log-level         debug, info, warn, or error
  --net-debug         Enable HTTP request/response debug logging

Legacy top-level flags such as -endpoint, -browse-path, and -read-path are still accepted.`)
}
