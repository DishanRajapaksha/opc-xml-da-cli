package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if code != exitGeneralError {
		t.Fatalf("Run(status --bad-flag) = %d, want %d", code, exitGeneralError)
	}
	if !strings.Contains(err.String(), "flag provided but not defined") {
		t.Fatalf("stderr missing bad flag error: %q", err.String())
	}
}

func TestRunMissingEndpoint(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"status"})
	if code != exitGeneralError {
		t.Fatalf("Run(status) = %d, want %d", code, exitGeneralError)
	}
	if !strings.Contains(err.String(), "endpoint is required") {
		t.Fatalf("stderr missing endpoint error: %q", err.String())
	}
}

func TestRunLegacyFlagsWarns(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"-endpoint", ""})
	if code != exitGeneralError {
		t.Fatalf("Run(legacy missing endpoint) = %d, want %d", code, exitGeneralError)
	}
	if !strings.Contains(err.String(), "top-level flags are deprecated") {
		t.Fatalf("stderr missing legacy warning: %q", err.String())
	}
}

func TestRunPlaceholderCommand(t *testing.T) {
	var out, err bytes.Buffer
	code := NewApp(&out, &err).Run([]string{"watch"})
	if code != exitGeneralError {
		t.Fatalf("Run(watch) = %d, want %d", code, exitGeneralError)
	}
	if !strings.Contains(err.String(), "watch is not implemented yet") {
		t.Fatalf("stderr missing placeholder error: %q", err.String())
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
	if code != exitGeneralError {
		t.Fatalf("Run(init-config existing) = %d, want %d", code, exitGeneralError)
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

func TestValidateConfigFailsForInvalidConfig(t *testing.T) {
	var out, err bytes.Buffer
	path := writeCLIConfig(t, `locale: en-US`)
	code := NewApp(&out, &err).Run([]string{"validate-config", "--config", path})
	if code != exitGeneralError {
		t.Fatalf("Run(validate-config invalid) = %d, want %d", code, exitGeneralError)
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
	addCommonFlags(fs, &opts)
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

func TestCommandOptionsApplyConfigKeepsCLIOverrides(t *testing.T) {
	path := writeCLIConfig(t, `
endpoint: http://from-config/opc
http_timeout: 2s
`)
	var errOut bytes.Buffer
	opts := defaultCommandOptions()
	fs := NewApp(&bytes.Buffer{}, &errOut).newFlagSet("status")
	addCommonFlags(fs, &opts)
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

func TestCommandOptionsApplyConfigIgnoresMissingDefaultConfig(t *testing.T) {
	var errOut bytes.Buffer
	opts := defaultCommandOptions()
	opts.ConfigPath = filepath.Join(t.TempDir(), "missing.yaml")
	fs := NewApp(&bytes.Buffer{}, &errOut).newFlagSet("status")
	addCommonFlags(fs, &opts)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := opts.applyConfig(fs); err != nil {
		t.Fatalf("applyConfig returned error: %v", err)
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
