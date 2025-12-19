package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"opc-xml-da-cli/service"
)

// FetchServerStatus requests the OPC server status.
func FetchServerStatus(ctx context.Context, svc service.OpcXmlDASoap, locale, clientHandle string) (*service.GetStatusResponse, error) {
	req := &service.GetStatus{
		LocaleID:            locale,
		ClientRequestHandle: clientHandle,
	}
	return svc.GetStatusContext(ctx, req)
}

// PrintStatus writes the response fields to out in a readable format.
func PrintStatus(out io.Writer, resp *service.GetStatusResponse) error {
	if out == nil {
		return errors.New("output is nil")
	}
	if resp == nil {
		_, err := fmt.Fprintln(out, "no response")
		return err
	}

	if resp.GetStatusResult != nil {
		if _, err := fmt.Fprintln(out, "GetStatusResult:"); err != nil {
			return err
		}
		if resp.GetStatusResult.ServerState != nil {
			if _, err := fmt.Fprintf(out, "  ServerState: %s\n", *resp.GetStatusResult.ServerState); err != nil {
				return err
			}
		}
		if resp.GetStatusResult.RevisedLocaleID != "" {
			if _, err := fmt.Fprintf(out, "  RevisedLocaleID: %s\n", resp.GetStatusResult.RevisedLocaleID); err != nil {
				return err
			}
		}
		if resp.GetStatusResult.ClientRequestHandle != "" {
			if _, err := fmt.Fprintf(out, "  ClientRequestHandle: %s\n", resp.GetStatusResult.ClientRequestHandle); err != nil {
				return err
			}
		}
		if t := formatXsdDateTime(resp.GetStatusResult.ReplyTime); t != "" {
			if _, err := fmt.Fprintf(out, "  ReplyTime: %s\n", t); err != nil {
				return err
			}
		}
		if t := formatXsdDateTime(resp.GetStatusResult.RcvTime); t != "" {
			if _, err := fmt.Fprintf(out, "  ReceiveTime: %s\n", t); err != nil {
				return err
			}
		}
	}

	if resp.Status != nil {
		if _, err := fmt.Fprintln(out, "Status:"); err != nil {
			return err
		}
		if resp.Status.StatusInfo != "" {
			if _, err := fmt.Fprintf(out, "  StatusInfo: %s\n", resp.Status.StatusInfo); err != nil {
				return err
			}
		}
		if resp.Status.VendorInfo != "" {
			if _, err := fmt.Fprintf(out, "  VendorInfo: %s\n", resp.Status.VendorInfo); err != nil {
				return err
			}
		}
		if resp.Status.ProductVersion != "" {
			if _, err := fmt.Fprintf(out, "  ProductVersion: %s\n", resp.Status.ProductVersion); err != nil {
				return err
			}
		}
		if t := formatXsdDateTime(resp.Status.StartTime); t != "" {
			if _, err := fmt.Fprintf(out, "  StartTime: %s\n", t); err != nil {
				return err
			}
		}
		if len(resp.Status.SupportedLocaleIDs) > 0 {
			if _, err := fmt.Fprintf(out, "  SupportedLocaleIDs: %s\n", strings.Join(resp.Status.SupportedLocaleIDs, ", ")); err != nil {
				return err
			}
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
				if _, err := fmt.Fprintf(out, "  SupportedInterfaceVersions: %s\n", strings.Join(versions, ", ")); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// formatXsdDateTime converts the SOAP datetime to RFC3339 when set.
func formatXsdDateTime(dt service.XSDDateTime) string {
	t := dt.ToGoTime()
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}
