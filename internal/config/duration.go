package config

import (
	"fmt"
	"time"
)

func (c *ClientConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawClientConfig struct {
		Endpoint       string `yaml:"endpoint"`
		Username       string `yaml:"username,omitempty"`
		Password       string `yaml:"password,omitempty"`
		Locale         string `yaml:"locale,omitempty"`
		ClientHandle   string `yaml:"client_handle,omitempty"`
		HTTPTimeout    string `yaml:"http_timeout"`
		RequestTimeout string `yaml:"request_timeout"`
	}
	var raw rawClientConfig
	if err := unmarshal(&raw); err != nil {
		return err
	}
	c.Endpoint = raw.Endpoint
	c.Username = raw.Username
	c.Password = raw.Password
	c.Locale = raw.Locale
	c.ClientHandle = raw.ClientHandle
	var err error
	c.HTTPTimeout, err = parseOptionalDuration("http_timeout", raw.HTTPTimeout)
	if err != nil {
		return err
	}
	c.RequestTimeout, err = parseOptionalDuration("request_timeout", raw.RequestTimeout)
	if err != nil {
		return err
	}
	return nil
}

func (f *FileConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawFileConfig struct {
		Endpoint       string                  `yaml:"endpoint"`
		Username       string                  `yaml:"username,omitempty"`
		Password       string                  `yaml:"password,omitempty"`
		Locale         string                  `yaml:"locale,omitempty"`
		ClientHandle   string                  `yaml:"client_handle,omitempty"`
		HTTPTimeout    string                  `yaml:"http_timeout"`
		RequestTimeout string                  `yaml:"request_timeout"`
		DefaultProfile string                  `yaml:"default_profile,omitempty"`
		Profiles       map[string]ClientConfig `yaml:"profiles,omitempty"`
	}
	var raw rawFileConfig
	if err := unmarshal(&raw); err != nil {
		return err
	}
	httpTimeout, err := parseOptionalDuration("http_timeout", raw.HTTPTimeout)
	if err != nil {
		return err
	}
	requestTimeout, err := parseOptionalDuration("request_timeout", raw.RequestTimeout)
	if err != nil {
		return err
	}
	f.ClientConfig = ClientConfig{
		Endpoint:       raw.Endpoint,
		Username:       raw.Username,
		Password:       raw.Password,
		Locale:         raw.Locale,
		ClientHandle:   raw.ClientHandle,
		HTTPTimeout:    httpTimeout,
		RequestTimeout: requestTimeout,
	}
	f.DefaultProfile = raw.DefaultProfile
	f.Profiles = raw.Profiles
	return nil
}

func parseOptionalDuration(name, value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return duration, nil
}
