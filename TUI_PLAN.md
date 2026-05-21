# TUI Node Browser Implementation Plan

## Summary

Add an interactive `tui` command while preserving the existing script-friendly commands.

The TUI provides:

- Address-space tree with lazy child loading.
- Selected item details and current value reads.
- Polling-based monitored values panel.
- Info/event log.
- Footer key hints.

## Command Shape

```bash
opc-xml-da-cli tui --item-name Plant.Area --interval 1s
```

The command supports the same connection, config, auth, timeout, verbose, debug, and HTTP diagnostic globals as existing runtime commands. It does not support `--format` or `--duration`.

## Behavior

- Arrows/Enter: navigate and expand tree.
- Tab: move focus across panes.
- `a`: refresh details for selected item.
- `r`: read selected item once.
- `m`: poll-monitor selected item.
- `u`: unmonitor selected item.
- `R`: reload selected branch.
- `c`: clear info log.
- `?`: show help.
- `q` or Ctrl-C: exit.

## Protocol Notes

- Browse uses existing SOAP `Browse` behavior and continuation handling.
- Reads use existing XML-DA `Read` behavior.
- Live values are polling-based at `--interval`.
- SOAP `<Value>` payloads are captured as inner XML, with simple scalar values unwrapped for display and complex values compacted as XML.

## Validation

- CLI tests cover help, invalid interval handling, global flag handling, and completions.
- Controller tests cover monitor restart ordering, read-error logging, and bounded logs.
- XML-DA value tests cover scalar unwrapping and complex XML fallback.
- Verified with `go test ./...` and `make build`.
