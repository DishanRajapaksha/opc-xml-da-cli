package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = "config.yaml"

type ClientConfig struct {
	Endpoint       string        `yaml:"endpoint"`
	Username       string        `yaml:"username,omitempty"`
	Password       string        `yaml:"password,omitempty"`
	Locale         string        `yaml:"locale,omitempty"`
	ClientHandle   string        `yaml:"client_handle,omitempty"`
	HTTPTimeout    time.Duration `yaml:"http_timeout"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

type FileConfig struct {
	ClientConfig   `yaml:",inline"`
	DefaultProfile string                  `yaml:"default_profile,omitempty"`
	Profiles       map[string]ClientConfig `yaml:"profiles,omitempty"`
}

func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		HTTPTimeout:    30 * time.Second,
		RequestTimeout: 90 * time.Second,
	}
}

func LoadClientConfigForProfile(path, profile string) (ClientConfig, error) {
	cfg, err := LoadFile(path)
	if err != nil {
		return ClientConfig{}, err
	}
	selected := cfg.ClientConfig
	if selected.HTTPTimeout == 0 {
		selected.HTTPTimeout = DefaultClientConfig().HTTPTimeout
	}
	if selected.RequestTimeout == 0 {
		selected.RequestTimeout = DefaultClientConfig().RequestTimeout
	}

	if profile == "" {
		profile = cfg.DefaultProfile
	}
	if profile != "" {
		profileCfg, ok := cfg.Profiles[profile]
		if !ok {
			return ClientConfig{}, fmt.Errorf("profile %q not found", profile)
		}
		selected = mergeClientConfig(selected, profileCfg)
	}
	return selected, nil
}

func LoadFile(path string) (FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, fmt.Errorf("read config %q: %w", path, err)
	}
	cfg := FileConfig{ClientConfig: DefaultClientConfig()}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return FileConfig{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}

func ValidateClientConfig(cfg ClientConfig) error {
	if cfg.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if cfg.HTTPTimeout < 0 {
		return errors.New("http_timeout must be zero or greater")
	}
	if cfg.RequestTimeout < 0 {
		return errors.New("request_timeout must be zero or greater")
	}
	return nil
}

func mergeClientConfig(base, override ClientConfig) ClientConfig {
	if override.Endpoint != "" {
		base.Endpoint = override.Endpoint
	}
	if override.Username != "" {
		base.Username = override.Username
	}
	if override.Password != "" {
		base.Password = override.Password
	}
	if override.Locale != "" {
		base.Locale = override.Locale
	}
	if override.ClientHandle != "" {
		base.ClientHandle = override.ClientHandle
	}
	if override.HTTPTimeout != 0 {
		base.HTTPTimeout = override.HTTPTimeout
	}
	if override.RequestTimeout != 0 {
		base.RequestTimeout = override.RequestTimeout
	}
	return base
}
