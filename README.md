# thresher

`thresher` is a CLI for decoding Tailscale debug capture traffic.

It can:

- stream and decode live packet capture from a local `tailscaled`
- print packet output as JSONL, compact JSONL, summary rows, or packet-list rows
- convert saved Tailscale debug PCAPs into JSON for manual LLM upload
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

Convert a saved Tailscale debug capture into a JSON file for uploading to an LLM:

```bash
thresher convert capture.pcap                  # writes capture.json
thresher convert capture.pcap -o analysis.json
thresher convert capture.pcap -o -             # writes JSON to stdout
thresher convert capture.pcap --include-raw    # also includes raw hex and payload previews
thresher convert capture.pcap --summary -o summary.json # smaller aggregate export
```

Conversion runs entirely offline and does not upload anything. It supports classic
PCAP files from Tailscale debug capture (USER0 link type), not general Ethernet
captures or PCAPNG. The output is one JSON document with `schema_version`, a
`packets` array, and `packet_count`. It retains decoded protocol details, stream
timing, analysis annotations, and packet decode errors, while omitting raw hex
and payload previews by default to reduce size. Addresses, DNS names, and other
sensitive metadata remain: review the file before sharing it with an LLM.

File output is replaced only after successful conversion; stdout may contain
partial JSON if conversion fails. This JSON document is for external use, not
`analyze --input`, which expects JSONL. Large captures may exceed an LLM's upload
or context limits.

For cheaper LLM context, use the global `--summary` flag. It replaces per-packet records with
capture-wide totals, directional conversation counts, DNS observations, and
selected TCP findings or decode errors. It retains the first 100 conversations,
3 examples per finding category, and 20 query and 20 response examples. Global
counts still cover the entire capture; the output reports limits and omissions.
This is lossy compression, not anonymization: IP addresses and DNS names remain.

Detailed export remains the default and is the fallback for
investigating specific frames in the original PCAP. Summary mode rejects
`--include-raw=true`. Neither export is input for `analyze --input`.
See the [capture output guide](docs/capture-output.md#aggregate-json-summary) for
schema details, interpretation caveats, and a reproducible size comparison.

The same flag works with live capture and all analysis workflows:

```bash
thresher capture --summary -o summary.json # writes one aggregate when stopped with Ctrl-C
thresher analyze --summary --model gpt-4o
thresher analyze console --summary --model gpt-4o
thresher analyze web --summary --model gpt-4o
```

Analysis sends one aggregate JSON summary per batch to the LLM; the console and
web views still show individual packets. Existing packet and byte limits remain
based on the original records, not the summary size. `analyze --input` still takes
packet JSONL, not an exported summary. `capture --summary` overrides the selected
`--format`; `capture --format summary` without the global flag still prints
per-packet text rows. The older `convert --mode summary` spelling remains supported.

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
