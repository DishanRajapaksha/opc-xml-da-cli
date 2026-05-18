package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

const (
	FormatTable = "table"
	FormatText  = "text"
	FormatJSON  = "json"
	FormatJSONL = "jsonl"
)

func NormaliseFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", FormatText:
		return FormatText
	case FormatTable:
		return FormatTable
	case FormatJSON:
		return FormatJSON
	case FormatJSONL:
		return FormatJSONL
	default:
		return value
	}
}

func WriteJSON(w io.Writer, value interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func WriteJSONLine(w io.Writer, value interface{}) error {
	return json.NewEncoder(w).Encode(value)
}

func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}
