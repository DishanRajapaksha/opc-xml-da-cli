from pathlib import Path

path = Path("internal/cli/app_test.go")
text = path.read_text()
old = '''func TestRenderReadJSONL(t *testing.T) {
\tvar out, errOut bytes.Buffer
\tapp := NewApp(&out, &errOut)
\tresp := &service.ReadResponse{
\t\tRItemList: &service.ReplyItemList{
\t\t\tItems: []*service.ItemValue{{ItemName: "A"}},
\t\t},
\t}
\tif err := app.renderRead("jsonl", resp); err != nil {
\t\tt.Fatalf("renderRead returned error: %v", err)
\t}
\tvar decoded map[string]interface{}
\tif err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
\t\tt.Fatalf("invalid jsonl: %v", err)
\t}
}
'''
new = '''func TestRenderReadCSV(t *testing.T) {
\tvar out, errOut bytes.Buffer
\tapp := NewApp(&out, &errOut)
\tresp := &service.ReadResponse{
\t\tRItemList: &service.ReplyItemList{
\t\t\tItems: []*service.ItemValue{{ItemName: "A"}},
\t\t},
\t}
\tif err := app.renderRead("csv", resp); err != nil {
\t\tt.Fatalf("renderRead returned error: %v", err)
\t}
\tif !strings.Contains(out.String(), "A") {
\t\tt.Fatalf("CSV output missing item: %q", out.String())
\t}
}
'''
if old not in text:
    raise SystemExit("JSONL read test block not found")
path.write_text(text.replace(old, new, 1))
