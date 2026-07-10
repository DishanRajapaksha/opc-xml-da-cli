from pathlib import Path


def replace_function(text: str, name: str, replacement: str) -> str:
    start = text.find(f"func {name}(")
    if start < 0:
        raise SystemExit(f"function {name} not found")
    end = text.find("\nfunc ", start + 1)
    if end < 0:
        end = len(text)
    return text[:start] + replacement.rstrip() + "\n" + text[end:]


output = Path("internal/output/output.go")
text = output.read_text()
text = text.replace('import (\n\t"encoding/json"', 'import (\n\t"encoding/csv"\n\t"encoding/json"', 1)
text = text.replace('\tFormatJSONL = "jsonl"\n', '\tFormatJSONL = "jsonl"\n\tFormatCSV   = "csv"\n', 1)
text = text.replace('\tcase FormatJSONL:\n\t\treturn FormatJSONL', '\tcase FormatJSONL:\n\t\treturn FormatJSONL\n\tcase FormatCSV:\n\t\treturn FormatCSV', 1)
text += '''

func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
\tcw := csv.NewWriter(w)
\tif len(headers) > 0 {
\t\tif err := cw.Write(headers); err != nil {
\t\t\treturn err
\t\t}
\t}
\tif err := cw.WriteAll(rows); err != nil {
\t\treturn err
\t}
\treturn cw.Error()
}

func WriteCSVRows(w io.Writer, rows [][]string) error {
\treturn WriteCSV(w, nil, rows)
}
'''
output.write_text(text)

app = Path("internal/cli/app.go")
text = app.read_text()
text = text.replace(
    '\texitRequestError      = 4\n\texitOutputError       = 9',
    '\texitRequestError  = 4\n\texitWriteRejected = 7\n\texitTimeout       = 8\n\texitOutputError   = 9',
    1,
)
text = text.replace(
    '\tif isConfigError(err) {\n\t\treturn exitConfigError\n\t}\n\tif isOutputError(err) {',
    '\tif isConfigError(err) {\n\t\treturn exitConfigError\n\t}\n\tif isTimeoutError(err) {\n\t\treturn exitTimeout\n\t}\n\tif isOutputError(err) {',
    1,
)
marker = 'func isOutputError(err error) bool {'
helper = '''func isTimeoutError(err error) bool {
\treturn errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout")
}

'''
if marker not in text:
    raise SystemExit("isOutputError marker not found")
text = text.replace(marker, helper + marker, 1)

snapshot_token = "__SNAPSHOT_FORMAT_HELP__"
stream_token = "__STREAM_FORMAT_HELP__"
for old in (
    "output format: table, text, json, or jsonl",
    "output format: table, text, or json",
):
    text = text.replace(old, snapshot_token)
text = text.replace("output format: text or jsonl", stream_token)
text = text.replace(snapshot_token, "output format: table, text, json, or csv")
text = text.replace(stream_token, "output format: text, jsonl, or csv")

text = replace_function(text, "(a *App) renderStatus", '''func (a *App) renderStatus(format string, resp *service.GetStatusResponse) error {
\trows := [][]string{}
\tif resp != nil && resp.GetStatusResult != nil && resp.GetStatusResult.ServerState != nil {
\t\trows = append(rows, []string{"server_state", string(*resp.GetStatusResult.ServerState)})
\t}
\tif resp != nil && resp.Status != nil {
\t\tif resp.Status.StatusInfo != "" {
\t\t\trows = append(rows, []string{"status_info", resp.Status.StatusInfo})
\t\t}
\t\tif resp.Status.VendorInfo != "" {
\t\t\trows = append(rows, []string{"vendor_info", resp.Status.VendorInfo})
\t\t}
\t\tif resp.Status.ProductVersion != "" {
\t\t\trows = append(rows, []string{"product_version", resp.Status.ProductVersion})
\t\t}
\t}
\tswitch output.NormaliseFormat(format) {
\tcase output.FormatText:
\t\treturn PrintStatus(a.out, resp)
\tcase output.FormatJSON:
\t\treturn output.WriteJSON(a.out, resp)
\tcase output.FormatTable:
\t\treturn output.WriteTable(a.out, []string{"Field", "Value"}, rows)
\tcase output.FormatCSV:
\t\treturn output.WriteCSV(a.out, []string{"Field", "Value"}, rows)
\tdefault:
\t\treturn invalidSnapshotFormat(format)
\t}
}''')

text = replace_function(text, "(a *App) renderBrowse", '''func (a *App) renderBrowse(format string, elements []*service.BrowseElement) error {
\trows := make([][]string, 0, len(elements))
\tfor _, el := range elements {
\t\tif el == nil {
\t\t\tcontinue
\t\t}
\t\trows = append(rows, []string{browseElementName(el), el.ItemPath, el.ItemName, fmt.Sprint(el.IsItem), fmt.Sprint(el.HasChildren)})
\t}
\tswitch output.NormaliseFormat(format) {
\tcase output.FormatJSON:
\t\treturn output.WriteJSON(a.out, elements)
\tcase output.FormatTable:
\t\treturn output.WriteTable(a.out, []string{"Name", "ItemPath", "ItemName", "IsItem", "HasChildren"}, rows)
\tcase output.FormatCSV:
\t\treturn output.WriteCSV(a.out, []string{"Name", "ItemPath", "ItemName", "IsItem", "HasChildren"}, rows)
\tdefault:
\t\treturn invalidSnapshotFormat(format)
\t}
}''')

text = replace_function(text, "(a *App) renderRead", '''func (a *App) renderRead(format string, resp *service.ReadResponse) error {
\trows := readResponseRows(resp)
\tswitch output.NormaliseFormat(format) {
\tcase output.FormatText:
\t\treturn PrintRead(a.out, resp)
\tcase output.FormatJSON:
\t\treturn output.WriteJSON(a.out, resp)
\tcase output.FormatTable:
\t\treturn output.WriteTable(a.out, readHeaders(), rows)
\tcase output.FormatCSV:
\t\treturn output.WriteCSVRows(a.out, rows)
\tdefault:
\t\treturn invalidSnapshotFormat(format)
\t}
}''')

text = replace_function(text, "(a *App) renderWatch", '''func (a *App) renderWatch(format string, item itemRef, resp *service.ReadResponse) error {
\tswitch output.NormaliseFormat(format) {
\tcase output.FormatText:
\t\treturn PrintRead(a.out, resp)
\tcase output.FormatJSONL:
\t\treturn output.WriteJSONLine(a.out, map[string]interface{}{
\t\t\t"item_path": item.ItemPath,
\t\t\t"item_name": item.ItemName,
\t\t\t"response":  resp,
\t\t})
\tcase output.FormatCSV:
\t\treturn output.WriteCSVRows(a.out, readResponseRows(resp))
\tdefault:
\t\treturn invalidWatchFormat(format)
\t}
}''')

text = replace_function(text, "validateSnapshotFormat", '''func validateSnapshotFormat(format string) error {
\tswitch output.NormaliseFormat(format) {
\tcase output.FormatText, output.FormatTable, output.FormatJSON, output.FormatCSV:
\t\treturn nil
\tdefault:
\t\treturn invalidSnapshotFormat(format)
\t}
}''')
text = replace_function(text, "invalidSnapshotFormat", '''func invalidSnapshotFormat(format string) error {
\treturn fmt.Errorf("invalid output format %q; expected table, text, json, or csv", format)
}''')
text = replace_function(text, "validateWatchFormat", '''func validateWatchFormat(format string) error {
\tswitch output.NormaliseFormat(format) {
\tcase output.FormatText, output.FormatJSONL, output.FormatCSV:
\t\treturn nil
\tdefault:
\t\treturn invalidWatchFormat(format)
\t}
}''')
text = replace_function(text, "invalidWatchFormat", '''func invalidWatchFormat(format string) error {
\treturn fmt.Errorf("invalid output format %q; expected text, jsonl, or csv", format)
}''')
text = replace_function(text, "validateReadFormat", '''func validateReadFormat(format string) error {
\treturn validateSnapshotFormat(format)
}''')

insert_marker = 'func (a *App) renderStatus('
helpers = '''func readHeaders() []string {
\treturn []string{"ItemPath", "ItemName", "Value", "Quality", "Timestamp", "DiagnosticInfo"}
}

func readResponseRows(resp *service.ReadResponse) [][]string {
\trows := [][]string{}
\tif resp == nil || resp.RItemList == nil {
\t\treturn rows
\t}
\tfor _, item := range resp.RItemList.Items {
\t\tif item == nil {
\t\t\tcontinue
\t\t}
\t\trows = append(rows, []string{
\t\t\titem.ItemPath,
\t\t\titem.ItemName,
\t\t\tformatXMLDAValue(item.Value),
\t\t\tformatOPCQuality(item.Quality),
\t\t\tformatXsdDateTime(item.Timestamp),
\t\t\titem.DiagnosticInfo,
\t\t})
\t}
\treturn rows
}

'''
if insert_marker not in text:
    raise SystemExit("renderStatus marker not found")
text = text.replace(insert_marker, helpers + insert_marker, 1)

run_read_marker = '''\tctx, opcService, err := a.newService(opts)
\tif err != nil {
\t\treturn err
\t}
\tfor _, item := range opts.ReadItems {'''
run_read_replacement = '''\tctx, opcService, err := a.newService(opts)
\tif err != nil {
\t\treturn err
\t}
\tif output.NormaliseFormat(opts.Format) == output.FormatCSV {
\t\tif err := output.WriteCSV(a.out, readHeaders(), nil); err != nil {
\t\t\treturn fmt.Errorf("print read: %w", err)
\t\t}
\t}
\tfor _, item := range opts.ReadItems {'''
if run_read_marker not in text:
    raise SystemExit("runRead loop marker not found")
text = text.replace(run_read_marker, run_read_replacement, 1)

watch_marker = '''\tticker := time.NewTicker(interval)
\tdefer ticker.Stop()
\tfor {'''
watch_replacement = '''\tticker := time.NewTicker(interval)
\tdefer ticker.Stop()
\tif output.NormaliseFormat(opts.Format) == output.FormatCSV {
\t\tif err := output.WriteCSV(a.out, readHeaders(), nil); err != nil {
\t\t\treturn fmt.Errorf("print watch: %w", err)
\t\t}
\t}
\tfor {'''
if watch_marker not in text:
    raise SystemExit("watch loop marker not found")
text = text.replace(watch_marker, watch_replacement, 1)
app.write_text(text)

readme = Path("README.md")
text = readme.read_text()
text = text.replace('| JSON Lines read | `opc-xml-da-cli read --items items.txt --format jsonl` |', '| CSV read | `opc-xml-da-cli read --items items.txt --format csv` |')
text = text.replace('opc-xml-da-cli read --items items.txt --format jsonl', 'opc-xml-da-cli read --items items.txt --format csv')
text = text.replace('`read` also supports `jsonl`, with one read response per line.', '`read` supports the snapshot formats `table`, `text`, `json`, and `csv`.')
text = text.replace('- `json`\n\n`read` also supports', '- `json`\n- `csv`\n\n`read` also supports')
text = text.replace('- `text` (default)\n- `jsonl`', '- `text` (default)\n- `jsonl`\n- `csv`')
text = text.replace('- `4`: XML-DA request error\n- `9`', '- `4`: protocol or request error\n- `7`: write or control rejected (reserved)\n- `8`: operation timeout\n- `9`')
readme.write_text(text)

Path("internal/cli/contracts_test.go").write_text(r'''package cli

import (
    "context"
    "testing"
)

func TestSnapshotFormatContract(t *testing.T) {
    for _, format := range []string{"table", "text", "json", "csv"} {
        if err := validateSnapshotFormat(format); err != nil {
            t.Fatalf("snapshot format %q rejected: %v", format, err)
        }
    }
    if err := validateSnapshotFormat("jsonl"); err == nil {
        t.Fatal("snapshot commands must reject jsonl")
    }
}

func TestStreamFormatContract(t *testing.T) {
    for _, format := range []string{"text", "jsonl", "csv"} {
        if err := validateWatchFormat(format); err != nil {
            t.Fatalf("stream format %q rejected: %v", format, err)
        }
    }
    for _, format := range []string{"table", "json"} {
        if err := validateWatchFormat(format); err == nil {
            t.Fatalf("stream format %q must be rejected", format)
        }
    }
}

func TestSharedExitCodeContract(t *testing.T) {
    if got := mapRunError(context.DeadlineExceeded); got != exitTimeout {
        t.Fatalf("timeout exit code = %d, want %d", got, exitTimeout)
    }
    if exitRequestError != 4 || exitWriteRejected != 7 || exitTimeout != 8 || exitOutputError != 9 {
        t.Fatal("shared exit-code contract changed")
    }
}
''')
