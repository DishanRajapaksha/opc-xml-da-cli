package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/DishanRajapaksha/industrial-cli-kit/command"
	"github.com/DishanRajapaksha/industrial-cli-kit/completion"
)

func TestRegistryMatchesDispatcher(t *testing.T) {
	dispatched := []string{
		"status", "browse", "tui", "read", "watch", "test-connection",
		"validate-config", "init-config", "completions", "help", "version",
	}
	registered := map[string]bool{}
	for _, registeredCommand := range cliRegistry.Commands {
		if registered[registeredCommand.Name] {
			t.Fatalf("duplicate registry command %q", registeredCommand.Name)
		}
		registered[registeredCommand.Name] = true
	}
	for _, name := range dispatched {
		if !registered[name] {
			t.Errorf("dispatcher command %q is not registered", name)
		}
	}
	for name := range registered {
		found := false
		for _, candidate := range dispatched {
			if candidate == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("registered command %q is not dispatched", name)
		}
	}
}

func TestRegistryGlobalFlagsMatchNormalizer(t *testing.T) {
	for _, global := range cliRegistry.GlobalFlags {
		args := []string{"--" + global.Name}
		if global.TakesValue {
			args = append(args, "value")
		}
		args = append(args, "status")
		normalised, err := normaliseGlobalFlags(args)
		if err != nil {
			t.Errorf("registered global flag --%s is rejected: %v", global.Name, err)
			continue
		}
		if len(normalised) == 0 || normalised[0] != "status" {
			t.Errorf("normalising --%s produced %v", global.Name, normalised)
		}
	}
}

func TestRegistryAppliesCommandGlobalPolicies(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "diagnostics drops output format",
			args: []string{
				"--config", "site.yaml", "--endpoint", "http://host/opc", "--format", "json",
				"--dump-http", "--timeout", "5s", "test-connection",
			},
			want: []string{
				"test-connection", "--config", "site.yaml", "--endpoint", "http://host/opc",
				"--dump-http", "--timeout", "5s",
			},
		},
		{
			name: "validation keeps config selection only",
			args: []string{
				"--config", "site.yaml", "--profile", "local", "--endpoint", "http://host/opc",
				"--verbose", "validate-config",
			},
			want: []string{"validate-config", "--config", "site.yaml", "--profile", "local"},
		},
		{
			name: "init config drops inherited globals",
			args: []string{"--config", "site.yaml", "--verbose", "init-config", "--output", "new.yaml"},
			want: []string{"init-config", "--output", "new.yaml"},
		},
		{
			name: "completion shell remains positional",
			args: []string{"--profile", "local", "completions", "bash"},
			want: []string{"completions", "bash"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normaliseGlobalFlags(test.args)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("normaliseGlobalFlags() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestRegistryLocalCommandsRejectGlobals(t *testing.T) {
	for _, name := range []string{"init-config", "completions", "help", "version"} {
		registered := registryCommand(t, name)
		if registered.GlobalFlags == nil || len(registered.GlobalFlags) != 0 {
			t.Fatalf("%s must explicitly reject global flags: %#v", name, registered.GlobalFlags)
		}
	}
	if got := registryCommand(t, "completions").LeadingArgs; got != 1 {
		t.Fatalf("completions LeadingArgs=%d, want 1", got)
	}
}

func TestGeneratedCompletionsApplyPolicies(t *testing.T) {
	var out bytes.Buffer
	if err := completion.Write(&out, "bash", cliRegistry); err != nil {
		t.Fatal(err)
	}
	script := out.String()
	for _, want := range []string{"browse", "watch", "--item-name", "--item-path", "--items", "--duration"} {
		if !strings.Contains(script, want) {
			t.Fatalf("completion output missing %q", want)
		}
	}
	assertCaseContains(t, script, "test-connection", "--endpoint", "--dump-http", "--timeout")
	assertCaseOmits(t, script, "test-connection", "--format")
	assertCaseContains(t, script, "validate-config", "--config", "--profile")
	assertCaseOmits(t, script, "validate-config", "--endpoint", "--verbose", "--format")
	assertCaseContains(t, script, "init-config", "--force", "--output")
	assertCaseOmits(t, script, "init-config", "--config", "--profile")
	if !strings.Contains(script, "complete -F _opc_xml_da_cli_completion opc-xml-da-cli") {
		t.Fatalf("completion is not registered for opc-xml-da-cli: %s", script)
	}
}

func registryCommand(t *testing.T, name string) command.Command {
	t.Helper()
	for _, registered := range cliRegistry.Commands {
		if registered.Name == name {
			return registered
		}
	}
	t.Fatalf("registry command %q not found", name)
	return command.Command{}
}

func assertCaseContains(t *testing.T, script, name string, values ...string) {
	t.Helper()
	line := bashCaseLine(t, script, name)
	for _, value := range values {
		if !strings.Contains(line, value) {
			t.Errorf("%s completion is missing %q: %s", name, value, line)
		}
	}
}

func assertCaseOmits(t *testing.T, script, name string, values ...string) {
	t.Helper()
	line := bashCaseLine(t, script, name)
	for _, value := range values {
		if strings.Contains(line, value) {
			t.Errorf("%s completion unexpectedly includes %q: %s", name, value, line)
		}
	}
}

func bashCaseLine(t *testing.T, script, name string) string {
	t.Helper()
	prefix := "    " + name + ") words="
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	t.Fatalf("completion case for %q not found", name)
	return ""
}
