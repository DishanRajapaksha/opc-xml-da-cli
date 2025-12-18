// Command opc-xml-da-cli calls an OPC XML-DA endpoint and prints GetStatus.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hooklift/gowsdl/soap"

	"opc-xml-da-cli/xmlda"
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
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -endpoint URL [options]\n       %s -endpoint URL -browse-path PATH [options]\n\n", os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}

	endpoint := flag.String("endpoint", "", "OPC XML-DA endpoint URL")
	browsePath := flag.String("browse-path", "", "OPC browse path (maps to ItemName)")
	browseItemPath := flag.String("browse-item-path", "", "OPC browse item path (maps to ItemPath)")
	browseDepth := flag.Int("browse-depth", 1, "Max browse depth (1 = direct children only)")
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

	if *endpoint == "" {
		flag.Usage()
		return fmt.Errorf("endpoint is required")
	}

	// Configure SOAP timeouts and optional basic auth.
	opts := []soap.Option{
		soap.WithTimeout(*httpTimeout),
		soap.WithRequestTimeout(*requestTimeout),
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
