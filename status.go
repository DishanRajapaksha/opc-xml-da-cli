package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"opc-xml-da-cli/xmlda"
)

func fetchServerStatus(ctx context.Context, service xmlda.OpcXmlDASoap, locale, clientHandle string) (*xmlda.GetStatusResponse, error) {
	req := &xmlda.GetStatus{
		LocaleID:            locale,
		ClientRequestHandle: clientHandle,
	}
	return service.GetStatusContext(ctx, req)
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
