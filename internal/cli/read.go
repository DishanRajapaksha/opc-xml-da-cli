package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"opc-xml-da-cli/service"
)

// FetchNodeValue requests the current value of a single OPC item.
func FetchNodeValue(ctx context.Context, svc service.OpcXmlDASoap, locale, clientHandle, itemPath, itemName string) (*service.ReadResponse, error) {
	if itemPath == "" && itemName == "" {
		return nil, errors.New("read requires an item path or item name")
	}
	options := &service.RequestOptions{
		ReturnErrorText:      true,
		ReturnDiagnosticInfo: true,
		ReturnItemTime:       true,
		ReturnItemPath:       true,
		ReturnItemName:       true,
		ClientRequestHandle:  clientHandle,
		LocaleID:             locale,
	}
	req := &service.Read{
		Options: options,
		ItemList: &service.ReadRequestItemList{
			Items: []*service.ReadRequestItem{
				{
					ItemPath: itemPath,
					ItemName: itemName,
				},
			},
		},
	}
	return svc.ReadContext(ctx, req)
}

// PrintRead writes the response fields to out in a readable format.
func PrintRead(out io.Writer, resp *service.ReadResponse) error {
	if out == nil {
		return errors.New("output is nil")
	}
	if resp == nil {
		_, err := fmt.Fprintln(out, "no response")
		return err
	}

	if resp.ReadResult != nil {
		if err := printReplyBase(out, "ReadResult", resp.ReadResult); err != nil {
			return err
		}
	}

	if resp.RItemList != nil {
		if _, err := fmt.Fprintln(out, "Items:"); err != nil {
			return err
		}
		for _, item := range resp.RItemList.Items {
			if item == nil {
				continue
			}
			if err := printItemValue(out, item); err != nil {
				return err
			}
		}
	}

	if len(resp.Errors) > 0 {
		if _, err := fmt.Fprintf(out, "Errors: %s\n", formatOPCErrors(resp.Errors)); err != nil {
			return err
		}
	}

	return nil
}

func printReplyBase(out io.Writer, label string, base *service.ReplyBase) error {
	if _, err := fmt.Fprintf(out, "%s:\n", label); err != nil {
		return err
	}
	if base == nil {
		_, err := fmt.Fprintln(out, "  <empty>")
		return err
	}
	if base.ServerState != nil {
		if _, err := fmt.Fprintf(out, "  ServerState: %s\n", *base.ServerState); err != nil {
			return err
		}
	}
	if base.RevisedLocaleID != "" {
		if _, err := fmt.Fprintf(out, "  RevisedLocaleID: %s\n", base.RevisedLocaleID); err != nil {
			return err
		}
	}
	if base.ClientRequestHandle != "" {
		if _, err := fmt.Fprintf(out, "  ClientRequestHandle: %s\n", base.ClientRequestHandle); err != nil {
			return err
		}
	}
	if t := formatXsdDateTime(base.ReplyTime); t != "" {
		if _, err := fmt.Fprintf(out, "  ReplyTime: %s\n", t); err != nil {
			return err
		}
	}
	if t := formatXsdDateTime(base.RcvTime); t != "" {
		if _, err := fmt.Fprintf(out, "  ReceiveTime: %s\n", t); err != nil {
			return err
		}
	}
	return nil
}

func printItemValue(out io.Writer, item *service.ItemValue) error {
	if item == nil {
		return nil
	}
	if _, err := fmt.Fprintln(out, "  - Item"); err != nil {
		return err
	}
	if item.ItemName != "" {
		if _, err := fmt.Fprintf(out, "    ItemName: %s\n", item.ItemName); err != nil {
			return err
		}
	}
	if item.ItemPath != "" {
		if _, err := fmt.Fprintf(out, "    ItemPath: %s\n", item.ItemPath); err != nil {
			return err
		}
	}
	if item.ClientItemHandle != "" {
		if _, err := fmt.Fprintf(out, "    ClientItemHandle: %s\n", item.ClientItemHandle); err != nil {
			return err
		}
	}
	if item.ResultID != nil {
		if _, err := fmt.Fprintf(out, "    ResultID: %s\n", *item.ResultID); err != nil {
			return err
		}
	}
	if item.ValueTypeQualifier != nil {
		if _, err := fmt.Fprintf(out, "    ValueTypeQualifier: %s\n", *item.ValueTypeQualifier); err != nil {
			return err
		}
	}
	if t := formatXsdDateTime(item.Timestamp); t != "" {
		if _, err := fmt.Fprintf(out, "    Timestamp: %s\n", t); err != nil {
			return err
		}
	}
	if quality := formatOPCQuality(item.Quality); quality != "" {
		if _, err := fmt.Fprintf(out, "    Quality: %s\n", quality); err != nil {
			return err
		}
	}
	if item.DiagnosticInfo != "" {
		if _, err := fmt.Fprintf(out, "    DiagnosticInfo: %s\n", item.DiagnosticInfo); err != nil {
			return err
		}
	}
	return nil
}

func formatOPCQuality(quality *service.OPCQuality) string {
	if quality == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if quality.QualityField != nil {
		parts = append(parts, fmt.Sprintf("quality=%s", *quality.QualityField))
	}
	if quality.LimitField != nil {
		parts = append(parts, fmt.Sprintf("limit=%s", *quality.LimitField))
	}
	if quality.VendorField != 0 {
		parts = append(parts, fmt.Sprintf("vendor=%d", quality.VendorField))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}
