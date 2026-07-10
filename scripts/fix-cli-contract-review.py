from pathlib import Path

app = Path("internal/cli/app.go")
text = app.read_text()
old = '''\tcase output.FormatTable:
\t\treturn output.WriteTable(a.out, []string{"Name", "ItemPath", "ItemName", "IsItem", "HasChildren"}, rows)'''
new = '''\tcase output.FormatTable, output.FormatText:
\t\treturn output.WriteTable(a.out, []string{"Name", "ItemPath", "ItemName", "IsItem", "HasChildren"}, rows)'''
if text.count(old) != 1:
    raise SystemExit(f"browse table case count: {text.count(old)}")
app.write_text(text.replace(old, new, 1))

readme = Path("README.md")
text = readme.read_text()
old = '''- `table` (default)
- `text`
- `json`

`read` supports the snapshot formats `table`, `text`, `json`, and `csv`.'''
new = '''- `table` (default)
- `text`
- `json`
- `csv`'''
if old not in text:
    raise SystemExit("snapshot format list not found")
readme.write_text(text.replace(old, new, 1))
