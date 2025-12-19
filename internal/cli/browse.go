package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

	"opc-xml-da-cli/service"
)

// BrowseOpcTree writes a hierarchical browse tree to out.
func BrowseOpcTree(ctx context.Context, out io.Writer, svc service.OpcXmlDASoap, locale, clientHandle, itemPath, itemName string, maxDepth int) error {
	if out == nil {
		return errors.New("output is nil")
	}

	rootLabel := itemName
	if rootLabel == "" {
		rootLabel = itemPath
	}
	if rootLabel == "" {
		rootLabel = "<root>"
	}
	if _, err := fmt.Fprintln(out, rootLabel); err != nil {
		return err
	}
	if maxDepth <= 0 {
		return nil
	}

	visited := map[string]struct{}{
		makeBrowseKey(itemPath, itemName): {},
	}
	return browseOpcChildren(ctx, out, svc, locale, clientHandle, itemPath, itemName, "  ", 1, maxDepth, visited)
}

func browseOpcChildren(ctx context.Context, out io.Writer, svc service.OpcXmlDASoap, locale, clientHandle, itemPath, itemName, indent string, depth, maxDepth int, visited map[string]struct{}) error {
	elements, err := fetchBrowseElements(ctx, svc, locale, clientHandle, itemPath, itemName)
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
		if _, err := fmt.Fprintf(out, "%s%s%s\n", indent, name, suffix); err != nil {
			return err
		}

		if el.HasChildren && depth < maxDepth {
			key := makeBrowseKey(el.ItemPath, el.ItemName)
			if _, ok := visited[key]; ok {
				continue
			}
			visited[key] = struct{}{}
			if err := browseOpcChildren(ctx, out, svc, locale, clientHandle, el.ItemPath, el.ItemName, indent+"  ", depth+1, maxDepth, visited); err != nil {
				return err
			}
		}
	}

	return nil
}

func fetchBrowseElements(ctx context.Context, svc service.OpcXmlDASoap, locale, clientHandle, itemPath, itemName string) ([]*service.BrowseElement, error) {
	var all []*service.BrowseElement
	continuation := ""
	filter := service.BrowseFilterAll

	for {
		slog.Debug("browse page request", "item_path", itemPath, "item_name", itemName, "continuation", continuation)
		req := &service.Browse{
			LocaleID:            locale,
			ClientRequestHandle: clientHandle,
			ItemPath:            itemPath,
			ItemName:            itemName,
			ContinuationPoint:   continuation,
			BrowseFilter:        &filter,
			ReturnErrorText:     true,
		}

		resp, err := svc.BrowseContext(ctx, req)
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

func browseElementName(el *service.BrowseElement) string {
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

func formatOPCErrors(opcErrors []*service.OPCError) string {
	if len(opcErrors) == 0 {
		return "unknown error"
	}

	parts := make([]string, 0, len(opcErrors))
	for _, err := range opcErrors {
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
