# thresher

`thresher` is a CLI for decoding Tailscale debug capture traffic.

It can:

- stream and decode live packet capture from a local `tailscaled`
- print packet output as JSONL, compact JSONL, summary rows, or packet-list rows
- analyze live or saved packet streams with an Aperture-served LLM

## Installation

Homebrew:

```bash
brew install jaxxstorm/tap/thresher
```

GitHub Releases:

- Download the archive for your platform from `https://github.com/jaxxstorm/thresher/releases`
- Extract it and place `thresher` somewhere on your `PATH`

## Requirements

- a local `tailscaled` for live capture
- Go 1.26+
- an Aperture endpoint for `analyze`

## Usage

Show help:

```bash
thresher --help
```

Capture live traffic:

```bash
thresher capture
```

Write capture output to a file:

```bash
thresher capture -o capture.jsonl
```

Choose a capture format:

```bash
thresher capture --format jsonl
thresher capture --format jsonl-compact
thresher capture --format summary
thresher capture --format packet-list
```

Analyze live traffic with an Aperture endpoint:

```bash
thresher analyze --endpoint http://ai --model gpt-4o
```

Analyze a saved capture:

```bash
thresher analyze --endpoint http://ai --model gpt-4o --input capture.jsonl
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
