package cli

import "github.com/DishanRajapaksha/industrial-cli-kit/command"

var cliRegistry = command.Registry{
	Binary: appName,
	GlobalFlags: []command.Flag{
		{Name: "config", TakesValue: true, Summary: "YAML config file"},
		{Name: "profile", TakesValue: true, Summary: "config profile name"},
		{Name: "format", TakesValue: true, Summary: "output format"},
		{Name: "endpoint", TakesValue: true, Summary: "OPC XML-DA endpoint URL"},
		{Name: "verbose", Summary: "print request decisions"},
		{Name: "debug", Summary: "enable debug logging"},
		{Name: "dump-http", Summary: "dump HTTP requests and responses"},
		{Name: "locale", TakesValue: true, Summary: "requested locale"},
		{Name: "client-handle", TakesValue: true, Summary: "client handle"},
		{Name: "http-timeout", TakesValue: true, Summary: "HTTP transport timeout"},
		{Name: "timeout", TakesValue: true, Summary: "request timeout"},
		{Name: "username", TakesValue: true, Summary: "HTTP username"},
		{Name: "password", TakesValue: true, Summary: "HTTP password"},
	},
	Commands: []command.Command{
		{Name: "status", Summary: "Get server status"},
		{Name: "browse", Summary: "Browse items", Flags: registryFlags("item-name", "item-path", "depth")},
		{Name: "tui", Summary: "Browse items interactively", Flags: registryFlags("item-name", "item-path", "interval")},
		{Name: "read", Summary: "Read item values", Flags: registryFlags("item-name", "item-path", "items")},
		{Name: "watch", Summary: "Poll item values", Flags: registryFlags("item-name", "item-path", "items", "interval", "duration")},
		{Name: "test-connection", Summary: "Run connection diagnostics"},
		{Name: "validate-config", Summary: "Validate local config"},
		{Name: "init-config", Summary: "Write a starter YAML config", Flags: registryFlags("output", "force")},
		{Name: "completions", Summary: "Generate shell completion scripts"},
		{Name: "help", Summary: "Print help"},
		{Name: "version", Summary: "Print version information"},
	},
}

func registryFlags(names ...string) []command.Flag {
	flags := make([]command.Flag, 0, len(names))
	for _, name := range names {
		flags = append(flags, command.Flag{Name: name, TakesValue: name != "force"})
	}
	return flags
}
