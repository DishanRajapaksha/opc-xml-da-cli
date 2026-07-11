package cli

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"opc-xml-da-cli/service"
)

func TestRunHelp(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"help"})
	if code != exitSuccess {
		t.Fatalf("Run(help) = %d, want %d", code, exitSuccess)
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Fatalf("help output missing Commands section:\n%s", out.String())
	}
	if err.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", err.String())
	}
}

func TestRunSubcommandHelpSucceeds(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"read", "--help"})
	if code != exitSuccess {
		t.Fatalf("Run(read --help) = %d, want %d; stderr=%q", code, exitSuccess, err.String())
	}
	if !strings.Contains(err.String(), "Usage of read:") {
		t.Fatalf("stderr missing read usage: %q", err.String())
	}
}

func TestRunTUIHelpSucceeds(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"tui", "--help"})
	if code != exitSuccess {
		t.Fatalf("Run(tui --help) = %d, want %d; stderr=%q", code, exitSuccess, err.String())
	}
	if !strings.Contains(err.String(), "Usage of tui:") {
		t.Fatalf("stderr missing tui usage: %q", err.String())
	}
}

func TestRunTUIRejectsInvalidInterval(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"tui", "--interval", "0s"})
	if code != exitConfigError {
		t.Fatalf("Run(tui --interval 0s) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), "--interval must be greater than zero") {
		t.Fatalf("stderr = %q", err.String())
	}
}

func TestNormaliseGlobalFlagsSupportsTUI(t *testing.T) {
	got, err := normaliseGlobalFlags([]string{"--endpoint", "http://example.test/OPC/DA", "tui", "--item-name", "Plant"})
	if err != nil {
		t.Fatalf("normaliseGlobalFlags returned error: %v", err)
	}
	want := []string{"tui", "--endpoint", "http://example.test/OPC/DA", "--item-name", "Plant"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("normalised args = %#v, want %#v", got, want)
	}
}

func TestCompletionsIncludeTUI(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"completions", "bash"})
	if code != exitSuccess {
		t.Fatalf("Run(completions bash) = %d, want %d; stderr=%q", code, exitSuccess, err.String())
	}
	if !strings.Contains(out.String(), "tui") {
		t.Fatalf("bash completion missing tui: %q", out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"nope"})
	if code != exitGeneralError {
		t.Fatalf("Run(nope) = %d, want %d", code, exitGeneralError)
	}
	if !strings.Contains(err.String(), `unknown command "nope"`) {
		t.Fatalf("stderr missing unknown command: %q", err.String())
	}
}

func TestRunBadFlag(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"status", "--bad-flag"})
	if code != exitConfigError {
		t.Fatalf("Run(status --bad-flag) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), "flag provided but not defined") {
		t.Fatalf("stderr missing bad flag error: %q", err.String())
	}
}

func TestRunMissingEndpoint(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"status"})
	if code != exitConfigError {
		t.Fatalf("Run(status) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), "endpoint is required") {
		t.Fatalf("stderr missing endpoint error: %q", err.String())
	}
}

func TestRunRejectsUnsupportedFormat(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"status", "--format", "jsonl"})
	if code != exitConfigError {
		t.Fatalf("Run(status --format jsonl) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), `invalid output format "jsonl"`) {
		t.Fatalf("stderr missing format error: %q", err.String())
	}
}

func TestRunLegacyFlagsWarns(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"-endpoint", ""})
	if code != exitConfigError {
		t.Fatalf("Run(legacy missing endpoint) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), "top-level flags are deprecated") {
		t.Fatalf("stderr missing legacy warning: %q", err.String())
	}
}

func TestCompletionsRequiresShell(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"completions"})
	if code != exitConfigError {
		t.Fatalf("Run(completions) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), "usage: opc-xml-da-cli completions bash|zsh") {
		t.Fatalf("stderr missing completions usage: %q", err.String())
	}
}

func TestCompletionsHelpSucceeds(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"completions", "--help"})
	if code != exitSuccess {
		t.Fatalf("Run(completions --help) = %d, want %d; stderr=%q", code, exitSuccess, err.String())
	}
	if !strings.Contains(err.String(), "opc-xml-da-cli completions bash|zsh") {
		t.Fatalf("stderr missing completions usage: %q", err.String())
	}
}

func TestTestConnectionRejectsFormatFlag(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"test-connection", "--format", "json"})
	if code != exitConfigError {
		t.Fatalf("Run(test-connection --format json) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), "flag provided but not defined") {
		t.Fatalf("stderr missing flag error: %q", err.String())
	}
}

func TestCompletionsBash(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"completions", "bash"})
	if code != exitSuccess {
		t.Fatalf("Run(completions bash) = %d, want %d; stderr=%q", code, exitSuccess, err.String())
	}
	if !strings.Contains(out.String(), "complete -F _opc_xml_da_cli opc-xml-da-cli") {
		t.Fatalf("bash completion missing function registration")
	}
}

func TestWatchRejectsInvalidInterval(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"watch", "--item-name", "A", "--interval", "0s"})
	if code != exitConfigError {
		t.Fatalf("Run(watch --interval 0s) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), "--interval must be greater than zero") {
		t.Fatalf("stderr missing interval error: %q", err.String())
	}
}

func TestInitConfigWritesStarterConfig(t *testing.T) {
	var out, err bytes.Buffer
	outputPath := filepath.Join(t.TempDir(), "site-a.yaml")
	code := NewApp(&out, &err).Run([]string{"init-config", "--output", outputPath})
	if code != exitSuccess {
		t.Fatalf("Run(init-config) = %d, want %d; stderr=%q", code, exitSuccess, err.String())
	}
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("read starter config: %v", readErr)
	}
	if !strings.Contains(string(data), "endpoint:") {
		t.Fatalf("starter config missing endpoint:\n%s", string(data))
	}
	if !strings.Contains(out.String(), "wrote starter config") {
		t.Fatalf("stdout missing success message: %q", out.String())
	}
}

func TestInitConfigRefusesOverwriteWithoutForce(t *testing.T) {
	var out, err bytes.Buffer
	outputPath := filepath.Join(t.TempDir(), "site-a.yaml")
	if writeErr := os.WriteFile(outputPath, []byte("existing"), 0o600); writeErr != nil {
		t.Fatalf("write existing config: %v", writeErr)
	}
	code := NewApp(&out, &err).Run([]string{"init-config", "--output", outputPath})
	if code != exitConfigError {
		t.Fatalf("Run(init-config existing) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), "refusing to overwrite") {
		t.Fatalf("stderr missing overwrite refusal: %q", err.String())
	}
}

func TestInitConfigForceOverwrites(t *testing.T) {
	var out, err bytes.Buffer
	outputPath := filepath.Join(t.TempDir(), "site-a.yaml")
	if writeErr := os.WriteFile(outputPath, []byte("existing"), 0o600); writeErr != nil {
		t.Fatalf("write existing config: %v", writeErr)
	}
	code := NewApp(&out, &err).Run([]string{"init-config", "--output", outputPath, "--force"})
	if code != exitSuccess {
		t.Fatalf("Run(init-config --force) = %d, want %d; stderr=%q", code, exitSuccess, err.String())
	}
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("read starter config: %v", readErr)
	}
	if string(data) == "existing" {
		t.Fatal("starter config was not overwritten")
	}
}

func TestValidateConfigPasses(t *testing.T) {
	var out, err bytes.Buffer
	path := writeCLIConfig(t, `endpoint: http://localhost/opc`)
	code := NewApp(&out, &err).Run([]string{"validate-config", "--config", path})
	if code != exitSuccess {
		t.Fatalf("Run(validate-config) = %d, want %d; stderr=%q", code, exitSuccess, err.String())
	}
	if !strings.Contains(out.String(), "config validation: PASS") {
		t.Fatalf("stdout missing validation pass: %q", out.String())
	}
}

func TestValidateConfigAcceptsGlobalConfigFlag(t *testing.T) {
	var out, err bytes.Buffer
	path := writeCLIConfig(t, `endpoint: http://localhost/opc`)
	code := NewApp(&out, &err).Run([]string{"--config", path, "validate-config"})
	if code != exitSuccess {
		t.Fatalf("Run(global config validate-config) = %d, want %d; stderr=%q", code, exitSuccess, err.String())
	}
	if !strings.Contains(out.String(), "config validation: PASS") {
		t.Fatalf("stdout missing validation pass: %q", out.String())
	}
}

func TestValidateConfigFailsForInvalidConfig(t *testing.T) {
	var out, err bytes.Buffer
	path := writeCLIConfig(t, `locale: en-US`)
	code := NewApp(&out, &err).Run([]string{"validate-config", "--config", path})
	if code != exitConfigError {
		t.Fatalf("Run(validate-config invalid) = %d, want %d", code, exitConfigError)
	}
	if !strings.Contains(err.String(), "endpoint is required") {
		t.Fatalf("stderr missing validation error: %q", err.String())
	}
}

func TestCommandOptionsApplyConfig(t *testing.T) {
	path := writeCLIConfig(t, `
endpoint: http://from-config/opc
username: user
password: secret
locale: en-US
client_handle: cli
http_timeout: 2s
request_timeout: 3s
`)
	var errOut bytes.Buffer
	opts := defaultCommandOptions()
	fs := NewApp(&bytes.Buffer{}, &errOut).newFlagSet("status")
	addCommonFlags(fs, &opts, "output format: table, text, or json")
	if err := fs.Parse([]string{"--config", path}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := opts.applyConfig(fs); err != nil {
		t.Fatalf("applyConfig returned error: %v", err)
	}
	if opts.Endpoint != "http://from-config/opc" {
		t.Fatalf("Endpoint = %q", opts.Endpoint)
	}
	if opts.Username != "user" || opts.Password != "secret" || opts.Locale != "en-US" || opts.ClientHandle != "cli" {
		t.Fatalf("config fields not applied: %+v", opts)
	}
	if opts.HTTPTimeout != 2*time.Second || opts.RequestTimeout != 3*time.Second {
		t.Fatalf("timeouts not applied: http=%s request=%s", opts.HTTPTimeout, opts.RequestTimeout)
	}
}

func TestMapRunError(t *testing.T) {
	cases := map[error]int{
		nil:                                          exitSuccess,
		errors.New("endpoint is required"):           exitConfigError,
		errors.New("print read: short write"):        exitOutputError,
		errors.New("read: soap fault"):               exitRequestError,
		errors.New("test connection: FAIL: refused"): exitConnectionError,
		errors.New("unknown failure"):                exitGeneralError,
	}
	for err, want := range cases {
		if got := mapRunError(err); got != want {
			t.Fatalf("mapRunError(%v) = %d, want %d", err, got, want)
		}
	}
}

func TestCommandOptionsApplyConfigKeepsCLIOverrides(t *testing.T) {
	path := writeCLIConfig(t, `
endpoint: http://from-config/opc
http_timeout: 2s
`)
	var errOut bytes.Buffer
	opts := defaultCommandOptions()
	fs := NewApp(&bytes.Buffer{}, &errOut).newFlagSet("status")
	addCommonFlags(fs, &opts, "output format: table, text, or json")
	if err := fs.Parse([]string{"--config", path, "--endpoint", "http://override/opc"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := opts.applyConfig(fs); err != nil {
		t.Fatalf("applyConfig returned error: %v", err)
	}
	if opts.Endpoint != "http://override/opc" {
		t.Fatalf("Endpoint = %q", opts.Endpoint)
	}
	if opts.HTTPTimeout != 2*time.Second {
		t.Fatalf("HTTPTimeout = %s", opts.HTTPTimeout)
	}
}

func TestNormaliseGlobalFlagsPreservesCommandOverride(t *testing.T) {
	got, err := normaliseGlobalFlags([]string{"--format", "json", "read", "--format", "table", "--item-name", "A"})
	if err != nil {
		t.Fatalf("normaliseGlobalFlags returned error: %v", err)
	}
	want := []string{"read", "--format", "json", "--format", "table", "--item-name", "A"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("normalised args = %#v, want %#v", got, want)
	}
}

func TestNormaliseGlobalFlagsKeepsLegacySingleDashFlags(t *testing.T) {
	got, err := normaliseGlobalFlags([]string{"-endpoint", "http://localhost/opc"})
	if err != nil {
		t.Fatalf("normaliseGlobalFlags returned error: %v", err)
	}
	want := []string{"-endpoint", "http://localhost/opc"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("normalised args = %#v, want %#v", got, want)
	}
}

func TestCommandOptionsApplyConfigIgnoresMissingDefaultConfig(t *testing.T) {
	var errOut bytes.Buffer
	opts := defaultCommandOptions()
	opts.ConfigPath = filepath.Join(t.TempDir(), "missing.yaml")
	fs := NewApp(&bytes.Buffer{}, &errOut).newFlagSet("status")
	addCommonFlags(fs, &opts, "output format: table, text, or json")
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := opts.applyConfig(fs); err != nil {
		t.Fatalf("applyConfig returned error: %v", err)
	}
}

func TestReadItemRefs(t *testing.T) {
	itemsPath := filepath.Join(t.TempDir(), "items.txt")
	if err := os.WriteFile(itemsPath, []byte("# comment\nFile.Item\n\n"), 0o600); err != nil {
		t.Fatalf("write items file: %v", err)
	}
	items, err := readItemRefs(
		stringList{"Name.A"},
		stringList{"Path.B"},
		itemsPath,
	)
	if err != nil {
		t.Fatalf("readItemRefs returned error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3: %+v", len(items), items)
	}
	if items[0].ItemName != "Name.A" || items[1].ItemPath != "Path.B" || items[2].ItemName != "File.Item" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestRenderStatusJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	app := NewApp(&out, &errOut)
	resp := &service.GetStatusResponse{Status: &service.ServerStatus{StatusInfo: "ok"}}
	if err := app.renderStatus("json", resp); err != nil {
		t.Fatalf("renderStatus returned error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}

func TestRenderReadTable(t *testing.T) {
	var out, errOut bytes.Buffer
	app := NewApp(&out, &errOut)
	resp := &service.ReadResponse{
		RItemList: &service.ReplyItemList{
			Items: []*service.ItemValue{{ItemName: "A", DiagnosticInfo: "ok"}},
		},
	}
	if err := app.renderRead("table", resp); err != nil {
		t.Fatalf("renderRead returned error: %v", err)
	}
	if !strings.Contains(out.String(), "ItemName") || !strings.Contains(out.String(), "A") {
		t.Fatalf("table output missing fields: %q", out.String())
	}
}

func TestRenderReadCSV(t *testing.T) {
	var out, errOut bytes.Buffer
	app := NewApp(&out, &errOut)
	resp := &service.ReadResponse{
		RItemList: &service.ReplyItemList{
			Items: []*service.ItemValue{{ItemName: "A"}},
		},
	}
	if err := app.renderRead("csv", resp); err != nil {
		t.Fatalf("renderRead returned error: %v", err)
	}
	if !strings.Contains(out.String(), "A") {
		t.Fatalf("CSV output missing item: %q", out.String())
	}
}

func TestRenderWatchJSONL(t *testing.T) {
	var out, errOut bytes.Buffer
	app := NewApp(&out, &errOut)
	resp := &service.ReadResponse{
		RItemList: &service.ReplyItemList{
			Items: []*service.ItemValue{{ItemName: "A"}},
		},
	}
	if err := app.renderWatch("jsonl", itemRef{ItemName: "A"}, resp); err != nil {
		t.Fatalf("renderWatch returned error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid jsonl: %v", err)
	}
	if decoded["item_name"] != "A" {
		t.Fatalf("decoded item_name = %v", decoded["item_name"])
	}
}

func TestFormatXMLDAValueScalar(t *testing.T) {
	var item service.ItemValue
	raw := `<Items xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><Value xsi:type="xsd:double">24.1</Value></Items>`
	if err := xml.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatalf("unmarshal item value: %v", err)
	}
	if got := formatXMLDAValue(item.Value); got != "24.1" {
		t.Fatalf("formatXMLDAValue = %q, want 24.1", got)
	}
}

func TestFormatXMLDAValueComplexFallback(t *testing.T) {
	value := service.AnyType{InnerXML: "\n  <A> 1 </A>\n  <B>2</B>\n"}
	if got := formatXMLDAValue(value); got != "<A>1</A><B>2</B>" {
		t.Fatalf("formatXMLDAValue = %q", got)
	}
}

func writeCLIConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
