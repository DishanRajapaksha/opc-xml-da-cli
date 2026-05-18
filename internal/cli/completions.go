package cli

import (
	"fmt"
	"io"
)

func writeCompletion(w io.Writer, shell string) error {
	switch shell {
	case "bash":
		_, err := fmt.Fprint(w, bashCompletion)
		return err
	case "zsh":
		_, err := fmt.Fprint(w, zshCompletion)
		return err
	default:
		return fmt.Errorf("unsupported shell %q; expected bash or zsh", shell)
	}
}

const bashCompletion = `# bash completion for opc-xml-da-cli
_opc_xml_da_cli()
{
    local cur prev commands common_flags item_flags
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="help version status browse read watch test-connection validate-config init-config completions"
    common_flags="--config --profile --format --endpoint --verbose --debug --dump-http --locale --client-handle --http-timeout --timeout --username --password"
    item_flags="--item-name --item-path --items"
    case "$prev" in
        opc-xml-da-cli)
            COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
            return 0
            ;;
        completions)
            COMPREPLY=( $(compgen -W "bash zsh" -- "$cur") )
            return 0
            ;;
    esac
    case "${COMP_WORDS[1]}" in
        browse)
            COMPREPLY=( $(compgen -W "$common_flags --item-name --item-path --depth" -- "$cur") )
            ;;
        read)
            COMPREPLY=( $(compgen -W "$common_flags $item_flags" -- "$cur") )
            ;;
        watch)
            COMPREPLY=( $(compgen -W "$common_flags $item_flags --interval --duration" -- "$cur") )
            ;;
        status|test-connection)
            COMPREPLY=( $(compgen -W "$common_flags" -- "$cur") )
            ;;
        validate-config)
            COMPREPLY=( $(compgen -W "--config --profile" -- "$cur") )
            ;;
        init-config)
            COMPREPLY=( $(compgen -W "--output --force" -- "$cur") )
            ;;
    esac
}
complete -F _opc_xml_da_cli opc-xml-da-cli
`

const zshCompletion = `#compdef opc-xml-da-cli
_opc_xml_da_cli() {
  local -a commands
  commands=(
    'help:show help'
    'version:show version'
    'status:get server status'
    'browse:browse items'
    'read:read item values'
    'watch:poll item values'
    'test-connection:run connection diagnostics'
    'validate-config:validate local config'
    'init-config:write starter config'
    'completions:generate shell completions'
  )
  _describe 'command' commands
}
_opc_xml_da_cli "$@"
`
