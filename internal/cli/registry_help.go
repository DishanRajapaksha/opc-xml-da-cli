package cli

import sharedhelp "github.com/DishanRajapaksha/industrial-cli-kit/help"

func (a *App) writeRegistryUsage() {
	_ = sharedhelp.Write(a.out, cliRegistry, sharedhelp.Options{
		Description: "opc-xml-da-cli is a script-friendly OPC XML-DA command-line client.",
		Usage: []string{"opc-xml-da-cli [global flags] <command> [flags]"},
		Examples: []string{
			"opc-xml-da-cli status --profile local",
			"opc-xml-da-cli browse --profile local --item-name Plant --depth 2",
			"opc-xml-da-cli tui --profile local --item-name Plant --interval 1s",
			"opc-xml-da-cli read --profile local --item-name Plant.Temperature --format json",
			"opc-xml-da-cli watch --profile local --item-name Plant.Temperature --interval 1s --format jsonl",
			"opc-xml-da-cli test-connection --profile local",
			"opc-xml-da-cli validate-config --profile local",
			"opc-xml-da-cli init-config --output site.yaml",
			"opc-xml-da-cli completions zsh",
		},
	})
}
