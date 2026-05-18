package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadClientConfigForProfileUsesDefaults(t *testing.T) {
	path := writeConfig(t, `endpoint: http://localhost/opc`)
	cfg, err := LoadClientConfigForProfile(path, "")
	if err != nil {
		t.Fatalf("LoadClientConfigForProfile returned error: %v", err)
	}
	if cfg.Endpoint != "http://localhost/opc" {
		t.Fatalf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Fatalf("HTTPTimeout = %s", cfg.HTTPTimeout)
	}
	if cfg.RequestTimeout != 90*time.Second {
		t.Fatalf("RequestTimeout = %s", cfg.RequestTimeout)
	}
}

func TestLoadClientConfigForProfileAppliesProfile(t *testing.T) {
	path := writeConfig(t, `
endpoint: http://base/opc
http_timeout: 5s
default_profile: site-a
profiles:
  site-a:
    endpoint: http://site-a/opc
    username: user
    password: secret
    locale: en-US
    client_handle: cli
    request_timeout: 15s
`)
	cfg, err := LoadClientConfigForProfile(path, "")
	if err != nil {
		t.Fatalf("LoadClientConfigForProfile returned error: %v", err)
	}
	if cfg.Endpoint != "http://site-a/opc" {
		t.Fatalf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.HTTPTimeout != 5*time.Second {
		t.Fatalf("HTTPTimeout = %s", cfg.HTTPTimeout)
	}
	if cfg.RequestTimeout != 15*time.Second {
		t.Fatalf("RequestTimeout = %s", cfg.RequestTimeout)
	}
	if cfg.Username != "user" || cfg.Password != "secret" || cfg.Locale != "en-US" || cfg.ClientHandle != "cli" {
		t.Fatalf("profile fields not applied: %+v", cfg)
	}
}

func TestLoadClientConfigForProfileMissingProfile(t *testing.T) {
	path := writeConfig(t, `endpoint: http://localhost/opc`)
	_, err := LoadClientConfigForProfile(path, "missing")
	if err == nil {
		t.Fatal("LoadClientConfigForProfile returned nil error for missing profile")
	}
}

func TestValidateClientConfig(t *testing.T) {
	if err := ValidateClientConfig(ClientConfig{}); err == nil {
		t.Fatal("ValidateClientConfig returned nil error for missing endpoint")
	}
	if err := ValidateClientConfig(ClientConfig{Endpoint: "http://localhost/opc", HTTPTimeout: -time.Second}); err == nil {
		t.Fatal("ValidateClientConfig returned nil error for negative timeout")
	}
	if err := ValidateClientConfig(ClientConfig{Endpoint: "http://localhost/opc"}); err != nil {
		t.Fatalf("ValidateClientConfig returned error: %v", err)
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
