# OPC XML-DA CLI

A script-friendly OPC XML Data Access command-line client written in Go.

## At a Glance

| Task | Command |
|---|---|
| Create starter config | `opc-xml-da-cli init-config` |
| Validate local config | `opc-xml-da-cli validate-config` |
| Test connectivity | `opc-xml-da-cli test-connection` |
| Get server status | `opc-xml-da-cli status` |
| Browse from root | `opc-xml-da-cli browse --depth 1` |
| Browse from item | `opc-xml-da-cli browse --item-name Plant.Area --depth 1` |
| Read one item | `opc-xml-da-cli read --item-name Plant.Area.Tag` |
| Read multiple items | `opc-xml-da-cli read --item-name Tag.A --item-name Tag.B` |
| Read items from file | `opc-xml-da-cli read --items items.txt` |
| Watch by polling | `opc-xml-da-cli watch --item-name Plant.Area.Tag --interval 1s` |
| JSON output | `opc-xml-da-cli read --item-name Plant.Area.Tag --format json` |
| JSON Lines watch | `opc-xml-da-cli watch --item-name Plant.Area.Tag --interval 1s --duration 10s --format jsonl` |

## Install

Build from source:

```bash
make test
make build
```

Binary output: `bin/opc-xml-da-cli`

## First Run

Create `config.yaml`:

```bash
opc-xml-da-cli init-config
```

Validate it locally:

```bash
opc-xml-da-cli validate-config
```

Verify the endpoint with a real SOAP request:

```bash
opc-xml-da-cli test-connection
```

Get status and read one value:

```bash
opc-xml-da-cli status
opc-xml-da-cli read --item-name Plant.Area.Tag
```

## Config and Profiles

Use a custom config file:

```bash
opc-xml-da-cli status --config site-a.yaml
```

Use a profile from config:

```bash
opc-xml-da-cli read --config config.yaml --profile site-a --item-name Plant.Area.Tag
```

Override config values with CLI flags:

```bash
opc-xml-da-cli read --endpoint http://192.168.1.50/OPC/DA --item-name Plant.Area.Tag
```

Example config:

```yaml
endpoint: http://localhost/OPC/DA
http_timeout: 30s
request_timeout: 90s
locale: en-US

default_profile: local
profiles:
  local:
    endpoint: http://localhost/OPC/DA
  site-a:
    endpoint: http://192.168.1.50/OPC/DA
```

## Core Commands

### Status and Diagnostics

```bash
opc-xml-da-cli status
opc-xml-da-cli test-connection
opc-xml-da-cli validate-config --config config.example.yaml
```

### Browse

```bash
opc-xml-da-cli browse --depth 1
opc-xml-da-cli browse --item-name Plant.Area --depth 2 --format table
opc-xml-da-cli browse --item-path /Plant/Area --format json
```

### Read

```bash
opc-xml-da-cli read --item-name Plant.Area.Tag
opc-xml-da-cli read --item-name Tag.A --item-name Tag.B --format table
opc-xml-da-cli read --items items.txt --format json
```

`items.txt` uses one item name per line. Blank lines and `#` comments are ignored.

### Watch

```bash
opc-xml-da-cli watch --item-name Plant.Area.Tag --interval 1s
opc-xml-da-cli watch --item-name Plant.Area.Tag --interval 1s --duration 30s --format jsonl
```

`watch` is polling-based. XML-DA write support is not implemented.

## Output Formats

Snapshot commands support:

- `text` (default)
- `table`
- `json`

`watch` supports:

- `text` (default)
- `jsonl`

## Troubleshooting

```bash
opc-xml-da-cli status --verbose
opc-xml-da-cli status --debug
opc-xml-da-cli status --dump-http
```

- `--verbose`: high-level connection decisions.
- `--debug`: lower-level client debug logging.
- `--dump-http`: HTTP request/response diagnostics to stderr. Authorization, cookie, and proxy authorization headers are redacted.

Common flags:

- `--config`: YAML config file, defaults to `config.yaml`.
- `--profile`: config profile name.
- `--endpoint`: OPC XML-DA endpoint URL.
- `--timeout`: end-to-end request timeout.
- `--http-timeout`: HTTP dial timeout.
