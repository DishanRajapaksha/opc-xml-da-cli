package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"opc-xml-da-cli/xmlda"
)

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
