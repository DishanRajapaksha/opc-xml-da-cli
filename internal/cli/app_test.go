package cli

import (
	"bytes"
	"strings"
	"testing"
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
	code := NewApp(&out, &err).Run([]string{"init-config"})
	if code != exitGeneralError {
		t.Fatalf("Run(init-config) = %d, want %d", code, exitGeneralError)
	}
	if !strings.Contains(err.String(), "init-config is not implemented yet") {
		t.Fatalf("stderr missing placeholder error: %q", err.String())
	}
}
