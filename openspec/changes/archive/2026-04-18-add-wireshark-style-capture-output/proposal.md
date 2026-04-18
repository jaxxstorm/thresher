## Why

The current JSONL output contains rich protocol detail, but it is still harder to scan than a Wireshark or tshark packet list and does not present the most useful packet columns consistently at the top level. This change is needed now because the tool has enough decode depth to support serious debugging, but still needs a more readable, grep-friendly packet view that preserves Tailscale-specific context instead of forcing users to mentally reconstruct it from nested fields.

## What Changes

- Add normalized top-level packet columns such as number, time, src, dst, protocol, length, and info while preserving the existing nested decode
- Add stream-oriented fields including a stable stream identifier, per-stream packet numbering, relative timing, and normalized transport direction
- Improve protocol-specific info strings so TCP, DNS, and DISCO packets read more like Wireshark or tshark packet rows
- Expand analysis flags to cover additional TCP conditions such as duplicate ACK, retransmission variants, zero-window, keepalive, and missing predecessor data
- Add payload usability fields such as top-level payload length and safe ASCII previews for printable payloads
- Add output modes for full JSONL, compact JSONL, and tshark-like summary rows
- Add documentation and examples showing how the output maps to Wireshark-style packet columns and where Tailscale-specific metadata appears

## Non-goals

- No removal of the detailed nested decode already emitted by the parser and analyzer
- No decryption of encrypted payloads such as SSH application data
- No change to the Tailscale debug capture wrapper, DISCO wire format, or LocalAPI transport
- No attempt to replicate all of Wireshark’s dissectors or UI behaviors

## Capabilities

### New Capabilities

- `capture-output-modes`: Provide summary-oriented output formats alongside the existing detailed JSONL output

### Modified Capabilities

- `go-packet-parser`: Normalize top-level packet columns, payload usability fields, and clearer separation of outer capture metadata, Tailscale metadata, control traffic, and inner decoded traffic
- `stateful-flow-analysis`: Extend stream identifiers, per-stream numbering, relative timing, and Wireshark-like TCP analysis flags
- `dns-transaction-correlation`: Improve DNS info strings and transaction-aware packet presentation in Wireshark-style output
- `localapi-capture`: Expose the new `--format` modes and document the relationship between summary rows and detailed JSONL output

## Impact

- **Output contract**: packet records will gain additional top-level columns and output-format-specific behavior while keeping nested structures intact
- **CLI**: `thresher capture` and related readers will need a `--format` flag with multiple output modes
- **Analyzer and formatter code**: stream timing, stream packet numbering, info-string generation, and compact/summary rendering will expand in `internal/capture` and command output paths
- **Docs and tests**: examples, fixtures, and regression coverage will need to cover SSH/TCP, DNS, DISCO, and duplicate-ACK style packet views
