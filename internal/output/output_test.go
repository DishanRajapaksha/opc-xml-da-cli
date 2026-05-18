package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteJSONLine(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSONLine(&out, map[string]string{"a": "b"}); err != nil {
		t.Fatalf("WriteJSONLine returned error: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json line: %v", err)
	}
	if decoded["a"] != "b" {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestWriteTable(t *testing.T) {
	var out bytes.Buffer
	if err := WriteTable(&out, []string{"Name", "Value"}, [][]string{{"A", "1"}}); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Name") || !strings.Contains(out.String(), "A") {
		t.Fatalf("table output missing fields: %q", out.String())
	}
}
