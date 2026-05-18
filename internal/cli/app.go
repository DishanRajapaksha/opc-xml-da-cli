package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/hooklift/gowsdl/soap"

	"opc-xml-da-cli/internal/config"
	"opc-xml-da-cli/internal/output"
	"opc-xml-da-cli/service"
)

const (
	appName               = "opc-xml-da-cli"
	defaultBrowseDepth    = 1
	defaultHTTPTimeout    = 30 * time.Second
	defaultRequestTimeout = 90 * time.Second
	defaultLogLevel       = "warn"
	exitSuccess           = 0
	exitGeneralError      = 1
)

type App struct {
	out io.Writer
	err io.Writer
}

type commandOptions struct {
	ConfigPath     string
	Profile        string
	Format         string
	Endpoint       string
	BrowsePath     string
	BrowseItemPath string
	BrowseDepth    int
	ReadPath       string
	ReadItemPath   string
	ReadItems      []itemRef
	DumpHTTP       bool
	LogLevel       string
	Verbose        bool
	Debug          bool
	Locale         string
	ClientHandle   string
	HTTPTimeout    time.Duration
	RequestTimeout time.Duration
	Username       string
	Password       string
}

func defaultCommandOptions() commandOptions {
	return commandOptions{
		ConfigPath:     config.DefaultConfigPath,
		Format:         "text",
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
	case "watch":
		err = a.watch(args[1:])
	case "test-connection":
		err = a.testConnection(args[1:])
	case "validate-config":
		err = a.validateConfig(args[1:])
	case "init-config":
		err = a.initConfig(args[1:])
	case "completions":
		err = a.completions(args[1:])
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

func errNotImplemented(command string) error {
	return fmt.Errorf("%s is not implemented yet", command)
}

func (a *App) completions(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: opc-xml-da-cli completions bash|zsh")
	}
	return writeCompletion(a.out, args[0])
}

func (a *App) initConfig(args []string) error {
	outputPath := config.DefaultConfigPath
	force := false
	fs := a.newFlagSet("init-config")
	fs.StringVar(&outputPath, "output", outputPath, "output YAML config file")
	fs.BoolVar(&force, "force", false, "overwrite output file if it exists")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !force {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("refusing to overwrite existing file %q; use --force to overwrite", outputPath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %q: %w", outputPath, err)
		}
	}
	if err := os.WriteFile(outputPath, config.StarterConfigYAML(), 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", outputPath, err)
	}
	fmt.Fprintf(a.out, "wrote starter config to %s\n", outputPath)
	return nil
}

func (a *App) validateConfig(args []string) error {
	configPath := config.DefaultConfigPath
	profile := ""
	fs := a.newFlagSet("validate-config")
	fs.StringVar(&configPath, "config", configPath, "YAML config file")
	fs.StringVar(&profile, "profile", profile, "config profile name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.LoadClientConfigForProfile(configPath, profile)
	if err != nil {
		return err
	}
	if err := config.ValidateClientConfig(cfg); err != nil {
		return err
	}
	fmt.Fprintln(a.out, "config validation: PASS")
	return nil
}

func (a *App) status(args []string) error {
	opts := defaultCommandOptions()
	fs := a.newFlagSet("status")
	addCommonFlags(fs, &opts)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := opts.applyConfig(fs); err != nil {
		return err
	}
	if err := validateSnapshotFormat(opts.Format); err != nil {
		return err
	}
	return a.runStatus(opts)
}

func (a *App) browse(args []string) error {
	opts := defaultCommandOptions()
	fs := a.newFlagSet("browse")
	addCommonFlags(fs, &opts)
	fs.StringVar(&opts.BrowsePath, "item-name", "", "OPC browse item name")
	fs.StringVar(&opts.BrowseItemPath, "item-path", "", "OPC browse item path")
	fs.IntVar(&opts.BrowseDepth, "depth", opts.BrowseDepth, "max browse depth (1 = direct children only)")
	fs.StringVar(&opts.BrowsePath, "browse-path", "", "deprecated alias for --item-name")
	fs.StringVar(&opts.BrowseItemPath, "browse-item-path", "", "deprecated alias for --item-path")
	fs.IntVar(&opts.BrowseDepth, "browse-depth", opts.BrowseDepth, "deprecated alias for --depth")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := opts.applyConfig(fs); err != nil {
		return err
	}
	if err := validateSnapshotFormat(opts.Format); err != nil {
		return err
	}
	return a.runBrowse(opts)
}

func (a *App) read(args []string) error {
	opts := defaultCommandOptions()
	var itemNames stringList
	var itemPaths stringList
	itemsFile := ""
	fs := a.newFlagSet("read")
	addCommonFlags(fs, &opts)
	fs.Var(&itemNames, "item-name", "OPC read item name; repeat for multiple items")
	fs.Var(&itemPaths, "item-path", "OPC read item path; repeat for multiple items")
	fs.StringVar(&itemsFile, "items", "", "path to file with one item name per line")
	fs.StringVar(&opts.ReadPath, "read-path", "", "deprecated alias for --item-name")
	fs.StringVar(&opts.ReadItemPath, "read-item-path", "", "deprecated alias for --item-path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := opts.applyConfig(fs); err != nil {
		return err
	}
	if err := validateSnapshotFormat(opts.Format); err != nil {
		return err
	}
	items, err := readItemRefs(itemNames, itemPaths, itemsFile)
	if err != nil {
		return err
	}
	if opts.ReadPath != "" || opts.ReadItemPath != "" {
		items = append(items, itemRef{ItemPath: opts.ReadItemPath, ItemName: opts.ReadPath})
	}
	opts.ReadItems = items
	return a.runRead(opts)
}

func (a *App) watch(args []string) error {
	opts := defaultCommandOptions()
	var itemNames stringList
	var itemPaths stringList
	itemsFile := ""
	interval := time.Second
	duration := time.Duration(0)
	fs := a.newFlagSet("watch")
	addCommonFlags(fs, &opts)
	fs.Var(&itemNames, "item-name", "OPC read item name; repeat for multiple items")
	fs.Var(&itemPaths, "item-path", "OPC read item path; repeat for multiple items")
	fs.StringVar(&itemsFile, "items", "", "path to file with one item name per line")
	fs.DurationVar(&interval, "interval", interval, "poll interval")
	fs.DurationVar(&duration, "duration", duration, "stop after this duration; zero runs until interrupted")
	fs.StringVar(&opts.ReadPath, "read-path", "", "deprecated alias for --item-name")
	fs.StringVar(&opts.ReadItemPath, "read-item-path", "", "deprecated alias for --item-path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if interval <= 0 {
		return fmt.Errorf("--interval must be greater than zero")
	}
	if err := opts.applyConfig(fs); err != nil {
		return err
	}
	if err := validateWatchFormat(opts.Format); err != nil {
		return err
	}
	items, err := readItemRefs(itemNames, itemPaths, itemsFile)
	if err != nil {
		return err
	}
	if opts.ReadPath != "" || opts.ReadItemPath != "" {
		items = append(items, itemRef{ItemPath: opts.ReadItemPath, ItemName: opts.ReadPath})
	}
	opts.ReadItems = items
	return a.runWatch(opts, interval, duration)
}

func (a *App) testConnection(args []string) error {
	opts := defaultCommandOptions()
	fs := a.newFlagSet("test-connection")
	addCommonFlags(fs, &opts)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := opts.applyConfig(fs); err != nil {
		return err
	}
	ctx, opcService, err := a.newService(opts)
	if err != nil {
		return err
	}
	resp, err := FetchServerStatus(ctx, opcService, opts.Locale, opts.ClientHandle)
	if err != nil {
		return fmt.Errorf("test connection: FAIL: %w", err)
	}
	state := ""
	if resp != nil && resp.GetStatusResult != nil && resp.GetStatusResult.ServerState != nil {
		state = string(*resp.GetStatusResult.ServerState)
	}
	if state == "" {
		fmt.Fprintln(a.out, "test connection: PASS")
		return nil
	}
	fmt.Fprintf(a.out, "test connection: PASS server_state=%s\n", state)
	return nil
}

func (a *App) runLegacy(args []string) error {
	fmt.Fprintln(a.err, "warning: top-level flags are deprecated; use status, browse, or read subcommands")
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
	if err := opts.applyConfig(fs); err != nil {
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
	if err := a.renderStatus(opts.Format, resp); err != nil {
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
	if output.NormaliseFormat(opts.Format) != output.FormatText {
		elements, err := fetchBrowseElements(ctx, opcService, opts.Locale, opts.ClientHandle, opts.BrowseItemPath, opts.BrowsePath)
		if err != nil {
			return err
		}
		return a.renderBrowse(opts.Format, elements)
	}
	return BrowseOpcTree(ctx, a.out, opcService, opts.Locale, opts.ClientHandle, opts.BrowseItemPath, opts.BrowsePath, opts.BrowseDepth)
}

func (a *App) runRead(opts commandOptions) error {
	if len(opts.ReadItems) == 0 && (opts.ReadPath != "" || opts.ReadItemPath != "") {
		opts.ReadItems = []itemRef{{ItemPath: opts.ReadItemPath, ItemName: opts.ReadPath}}
	}
	if len(opts.ReadItems) == 0 {
		return fmt.Errorf("at least one --item-name or --item-path is required")
	}
	ctx, opcService, err := a.newService(opts)
	if err != nil {
		return err
	}
	for _, item := range opts.ReadItems {
		slog.Info("read requested", "item_path", item.ItemPath, "item_name", item.ItemName)
		resp, err := FetchNodeValue(ctx, opcService, opts.Locale, opts.ClientHandle, item.ItemPath, item.ItemName)
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		if err := a.renderRead(opts.Format, resp); err != nil {
			return fmt.Errorf("print read: %w", err)
		}
	}
	return nil
}

func (a *App) runWatch(opts commandOptions, interval, duration time.Duration) error {
	if len(opts.ReadItems) == 0 {
		return fmt.Errorf("at least one --item-name or --item-path is required")
	}
	ctx, opcService, err := a.newService(opts)
	if err != nil {
		return err
	}
	runCtx := ctx
	var cancel context.CancelFunc
	if duration > 0 {
		runCtx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		for _, item := range opts.ReadItems {
			resp, err := FetchNodeValue(runCtx, opcService, opts.Locale, opts.ClientHandle, item.ItemPath, item.ItemName)
			if err != nil {
				return fmt.Errorf("watch: %w", err)
			}
			if err := a.renderWatch(opts.Format, item, resp); err != nil {
				return fmt.Errorf("print watch: %w", err)
			}
		}
		select {
		case <-runCtx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

type itemRef struct {
	ItemPath string
	ItemName string
}

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func readItemRefs(itemNames, itemPaths stringList, itemsFile string) ([]itemRef, error) {
	items := make([]itemRef, 0, len(itemNames)+len(itemPaths))
	for _, itemName := range itemNames {
		items = append(items, itemRef{ItemName: itemName})
	}
	for _, itemPath := range itemPaths {
		items = append(items, itemRef{ItemPath: itemPath})
	}
	if itemsFile == "" {
		return items, nil
	}
	fromFile, err := readItemsFile(itemsFile)
	if err != nil {
		return nil, err
	}
	return append(items, fromFile...), nil
}

func readItemsFile(path string) ([]itemRef, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read items file %q: %w", path, err)
	}
	defer f.Close()
	var items []itemRef
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		items = append(items, itemRef{ItemName: line})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read items file %q: %w", path, err)
	}
	return items, nil
}

func (a *App) newService(opts commandOptions) (context.Context, service.OpcXmlDASoap, error) {
	if err := configureLogging(opts); err != nil {
		return nil, nil, err
	}
	if opts.Endpoint == "" {
		return nil, nil, fmt.Errorf("endpoint is required")
	}

	var soapOpts []soap.Option
	if opts.DumpHTTP {
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
	fs.StringVar(&opts.ConfigPath, "config", opts.ConfigPath, "YAML config file")
	fs.StringVar(&opts.Profile, "profile", opts.Profile, "config profile name")
	fs.StringVar(&opts.Format, "format", opts.Format, "output format: table, text, json, or jsonl where supported")
	fs.StringVar(&opts.Endpoint, "endpoint", opts.Endpoint, "OPC XML-DA endpoint URL")
	fs.BoolVar(&opts.Verbose, "verbose", opts.Verbose, "print high-level connection decisions")
	fs.BoolVar(&opts.Debug, "debug", opts.Debug, "enable lower-level client debug logging")
	fs.BoolVar(&opts.DumpHTTP, "dump-http", opts.DumpHTTP, "dump HTTP request/response details to stderr")
	fs.BoolVar(&opts.DumpHTTP, "net-debug", opts.DumpHTTP, "deprecated alias for --dump-http")
	fs.StringVar(&opts.LogLevel, "log-level", opts.LogLevel, "deprecated log level override: debug, info, warn, error")
	fs.StringVar(&opts.Locale, "locale", opts.Locale, "locale ID")
	fs.StringVar(&opts.ClientHandle, "client-handle", opts.ClientHandle, "client request handle")
	fs.DurationVar(&opts.HTTPTimeout, "http-timeout", opts.HTTPTimeout, "HTTP dial timeout")
	fs.DurationVar(&opts.RequestTimeout, "timeout", opts.RequestTimeout, "end-to-end request timeout")
	fs.DurationVar(&opts.RequestTimeout, "request-timeout", opts.RequestTimeout, "deprecated alias for --timeout")
	fs.StringVar(&opts.Username, "username", opts.Username, "Basic auth username")
	fs.StringVar(&opts.Password, "password", opts.Password, "Basic auth password")
}

func (opts *commandOptions) applyConfig(fs *flag.FlagSet) error {
	visited := visitedFlags(fs)
	if !shouldLoadConfig(opts.ConfigPath, visited) {
		return nil
	}
	fileCfg, err := config.LoadClientConfigForProfile(opts.ConfigPath, opts.Profile)
	if err != nil {
		return err
	}
	if !visited["endpoint"] {
		opts.Endpoint = fileCfg.Endpoint
	}
	if !visited["username"] {
		opts.Username = fileCfg.Username
	}
	if !visited["password"] {
		opts.Password = fileCfg.Password
	}
	if !visited["locale"] {
		opts.Locale = fileCfg.Locale
	}
	if !visited["client-handle"] {
		opts.ClientHandle = fileCfg.ClientHandle
	}
	if !visited["http-timeout"] {
		opts.HTTPTimeout = fileCfg.HTTPTimeout
	}
	if !visited["timeout"] && !visited["request-timeout"] {
		opts.RequestTimeout = fileCfg.RequestTimeout
	}
	return nil
}

func (a *App) renderStatus(format string, resp *service.GetStatusResponse) error {
	switch output.NormaliseFormat(format) {
	case output.FormatText:
		return PrintStatus(a.out, resp)
	case output.FormatJSON:
		return output.WriteJSON(a.out, resp)
	case output.FormatTable:
		rows := [][]string{}
		if resp != nil && resp.GetStatusResult != nil && resp.GetStatusResult.ServerState != nil {
			rows = append(rows, []string{"server_state", string(*resp.GetStatusResult.ServerState)})
		}
		if resp != nil && resp.Status != nil {
			if resp.Status.StatusInfo != "" {
				rows = append(rows, []string{"status_info", resp.Status.StatusInfo})
			}
			if resp.Status.VendorInfo != "" {
				rows = append(rows, []string{"vendor_info", resp.Status.VendorInfo})
			}
			if resp.Status.ProductVersion != "" {
				rows = append(rows, []string{"product_version", resp.Status.ProductVersion})
			}
		}
		return output.WriteTable(a.out, []string{"Field", "Value"}, rows)
	default:
		return invalidSnapshotFormat(format)
	}
}

func (a *App) renderBrowse(format string, elements []*service.BrowseElement) error {
	switch output.NormaliseFormat(format) {
	case output.FormatJSON:
		return output.WriteJSON(a.out, elements)
	case output.FormatTable:
		rows := make([][]string, 0, len(elements))
		for _, el := range elements {
			if el == nil {
				continue
			}
			rows = append(rows, []string{browseElementName(el), el.ItemPath, el.ItemName, fmt.Sprint(el.IsItem), fmt.Sprint(el.HasChildren)})
		}
		return output.WriteTable(a.out, []string{"Name", "ItemPath", "ItemName", "IsItem", "HasChildren"}, rows)
	default:
		return invalidSnapshotFormat(format)
	}
}

func (a *App) renderRead(format string, resp *service.ReadResponse) error {
	switch output.NormaliseFormat(format) {
	case output.FormatText:
		return PrintRead(a.out, resp)
	case output.FormatJSON:
		return output.WriteJSON(a.out, resp)
	case output.FormatTable:
		rows := [][]string{}
		if resp != nil && resp.RItemList != nil {
			for _, item := range resp.RItemList.Items {
				if item == nil {
					continue
				}
				rows = append(rows, []string{
					item.ItemPath,
					item.ItemName,
					formatOPCQuality(item.Quality),
					formatXsdDateTime(item.Timestamp),
					item.DiagnosticInfo,
				})
			}
		}
		return output.WriteTable(a.out, []string{"ItemPath", "ItemName", "Quality", "Timestamp", "DiagnosticInfo"}, rows)
	default:
		return invalidSnapshotFormat(format)
	}
}

func (a *App) renderWatch(format string, item itemRef, resp *service.ReadResponse) error {
	switch output.NormaliseFormat(format) {
	case output.FormatText:
		return PrintRead(a.out, resp)
	case output.FormatJSONL:
		return output.WriteJSONLine(a.out, map[string]interface{}{
			"item_path": item.ItemPath,
			"item_name": item.ItemName,
			"response":  resp,
		})
	default:
		return invalidWatchFormat(format)
	}
}

func validateSnapshotFormat(format string) error {
	switch output.NormaliseFormat(format) {
	case output.FormatText, output.FormatTable, output.FormatJSON:
		return nil
	default:
		return invalidSnapshotFormat(format)
	}
}

func invalidSnapshotFormat(format string) error {
	return fmt.Errorf("invalid output format %q; expected table, text, or json", format)
}

func validateWatchFormat(format string) error {
	switch output.NormaliseFormat(format) {
	case output.FormatText, output.FormatJSONL:
		return nil
	default:
		return invalidWatchFormat(format)
	}
}

func invalidWatchFormat(format string) error {
	return fmt.Errorf("invalid output format %q; expected text or jsonl", format)
}

func shouldLoadConfig(path string, visited map[string]bool) bool {
	if visited["config"] || visited["profile"] {
		return true
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	return visited
}

func configureLogging(opts commandOptions) error {
	logLevel := opts.LogLevel
	if opts.Debug {
		logLevel = "debug"
	} else if opts.Verbose {
		logLevel = "info"
	}
	level, err := parseLogLevel(logLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	if opts.DumpHTTP {
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
  opc-xml-da-cli browse --endpoint URL --item-name PATH --depth 1
  opc-xml-da-cli read --endpoint URL --item-name PATH
  opc-xml-da-cli watch --endpoint URL --item-name PATH --interval 1s
  opc-xml-da-cli test-connection --endpoint URL
  opc-xml-da-cli validate-config --config config.yaml
  opc-xml-da-cli init-config
  opc-xml-da-cli completions zsh
  opc-xml-da-cli version

Commands:
  status           Fetch OPC XML-DA GetStatus
  browse           Browse OPC XML-DA items
  read             Read an OPC XML-DA item
  watch            Poll item values
  test-connection  Run connection diagnostics
  validate-config  Validate local config
  init-config      Write starter config
  completions      Generate shell completions
  version          Print version information

Common flags:
  --endpoint          OPC XML-DA endpoint URL
  --config            YAML config file, defaults to config.yaml
  --profile           Config profile name
  --format            Output format: table, text, json, or jsonl where supported
  --locale            Locale ID
  --client-handle     Client request handle
  --http-timeout      HTTP dial timeout
  --timeout           End-to-end request timeout
  --username          Basic auth username
  --password          Basic auth password
  --verbose           Print high-level connection decisions
  --debug             Enable lower-level client debug logging
  --dump-http         Dump HTTP request/response details to stderr

Legacy top-level flags such as -endpoint, -browse-path, and -read-path are still accepted.`)
}
