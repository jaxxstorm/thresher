# thresher

`thresher` is a CLI for decoding Tailscale debug capture traffic.

It can:

- stream and decode live packet capture from a local `tailscaled`
- print packet output as JSONL, compact JSONL, summary rows, or packet-list rows
- analyze live or saved packet streams with an Aperture-served LLM

## Requirements

- a local `tailscaled` for live capture
- Go 1.26+
- an Aperture endpoint for `analyze`

## Usage

Show help:

```bash
go run . --help
```

Capture live traffic:

```bash
go run . capture
```

Write capture output to a file:

```bash
go run . capture -o capture.jsonl
```

Choose a capture format:

```bash
go run . capture --format jsonl
go run . capture --format jsonl-compact
go run . capture --format summary
go run . capture --format packet-list
```

Analyze live traffic with an Aperture endpoint:

```bash
go run . analyze --endpoint http://ai --model gpt-4o
```

Analyze a saved capture:

```bash
go run . analyze --endpoint http://ai --model gpt-4o --input capture.jsonl
```

## Config File

`thresher` reads a YAML config file named `thresher.yaml`.

It looks in:

- the current directory
- your home directory
- `~/.config/thresher`

Example `thresher.yaml`:

```yaml
analyze:
  endpoint: http://ai
  model: gpt-4o
  endpoint_style: auto
  batch_packets: 20
  batch_bytes: 65536
  session_packets: 500
  session_bytes: 2097152
  max_tokens: 300
```

CLI flags override config values.

## Notes

- `analyze` also has the alias `analyse`
- `analyze` only talks to Aperture-compatible endpoints such as `/v1/messages`, `/v1/chat/completions`, and `/v1/responses`
- `thresher` does not manage provider API keys directly for analysis
