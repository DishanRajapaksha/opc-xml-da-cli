// Command opc-xml-da-cli calls an OPC XML-DA endpoint and prints GetStatus.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
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
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -endpoint URL [options]\n\n", os.Args[0])
		flag.PrintDefaults()
	}

	endpoint := flag.String("endpoint", "", "OPC XML-DA endpoint URL")
	locale := flag.String("locale", "", "Locale ID (optional)")
	clientHandle := flag.String("client-handle", "", "Client request handle (optional)")
	httpTimeout := flag.Duration("http-timeout", 30*time.Second, "HTTP dial timeout")
	requestTimeout := flag.Duration("request-timeout", 90*time.Second, "End-to-end request timeout")
	username := flag.String("username", "", "Basic auth username (optional)")
	password := flag.String("password", "", "Basic auth password (optional)")
	flag.Parse()

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

	resp, err := service.GetStatusContext(ctx, req)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	printStatus(resp)
	return nil
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
func formatXsdDateTime(dt soap.XSDDateTime) string {
	t := dt.ToGoTime()
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}
