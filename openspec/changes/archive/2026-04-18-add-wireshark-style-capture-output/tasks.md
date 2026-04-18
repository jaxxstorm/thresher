## 1. Normalized Packet Row Fields

- [x] 1.1 Extend the core capture record in `internal/capture` with normalized top-level packet-row fields such as `number`, `time`, `src`, `dst`, `protocol`, `length`, `info`, `payload_length`, and optional printable payload preview
- [x] 1.2 Populate the normalized packet-row fields for inner IPv4/IPv6 traffic while preserving wrapper fields decoded from the 2-byte little-endian `path_id`, SNAT length/value byte, and DNAT length/value byte
- [x] 1.3 Populate the normalized packet-row fields for DISCO packets using the decoded DISCO metadata fields, message subtype, source port, and any endpoint information present after the wrapper payload handoff
- [x] 1.4 Add tests that verify normalized packet-row fields for SSH/TCP, DNS, and DISCO packets without regressing the nested decode

## 2. Stream Presentation And Analysis Flags

- [x] 2.1 Rename or supplement stream grouping output with `stream_id` and optional `conversation_key` while preserving stable grouping across both directions of a transport flow
- [x] 2.2 Add `stream_packet_number`, `time_since_stream_start`, and `time_since_previous_in_stream` to the analyzer-enriched packet record
- [x] 2.3 Add normalized transport direction such as `client_to_server` and `server_to_client` while keeping the Tailscale path direction from `path_id`
- [x] 2.4 Extend TCP analysis in `internal/capture/analyzer.go` to detect and emit `fast_retransmission`, `zero_window`, `keepalive`, `keepalive_ack`, and `previous_segment_not_captured` alongside the existing flags
- [x] 2.5 Add tests for per-stream numbering, relative timing, duplicate ACK, retransmission variants, zero-window or keepalive cases, and missing-segment annotations

## 3. Protocol-Specific Info Strings

- [x] 3.1 Refactor TCP info-string generation in `internal/capture` so rows read like `63242 → 22 [ACK] Seq=... Ack=... Win=... Len=...` and include analysis annotations when present
- [x] 3.2 Refactor DNS info-string generation so query and response rows read like Wireshark-style `Standard query A example.com` and `Standard query response ...`
- [x] 3.3 Refactor DISCO info-string generation so control packets surface subtype, endpoint, transaction ID, node key, and candidate endpoint information in normalized and nested locations
- [x] 3.4 Add tests covering TCP/SSH-style rows, DNS rows, and DISCO ping/pong rows

## 4. Output Modes And CLI Wiring

- [x] 4.1 Add a `--format` flag to the capture command with `jsonl`, `jsonl-compact`, and `summary` values
- [x] 4.2 Implement formatter support for full JSONL, compact JSONL, and tshark-like summary rows from one enriched packet record model
- [x] 4.3 Ensure summary output remains line-oriented and compact mode preserves nested details needed for jq and grep workflows
- [x] 4.4 Add command-level tests covering each format mode and error handling for unsupported format values

## 5. Documentation And Verification

- [x] 5.1 Add documentation and examples showing how normalized packet-row fields map to Wireshark packet-list columns and where Tailscale-specific metadata appears
- [x] 5.2 Document why DISCO packets are separate from inner TCP/DNS traffic and how summary rows relate to the detailed JSONL record
- [x] 5.3 Add or update fixtures for SSH/TCP flow, DNS query/response, DISCO ping/pong, and duplicate ACK scenarios used by output-format regression tests
- [x] 5.4 Run `go test ./...` and add a manual comparison plan for `go run . capture --format ...` against Wireshark or tshark views of the same traffic

### Manual Comparison Plan

- Run `go run . capture --format jsonl` and confirm detailed records still include nested `inner`, `analysis`, and `disco_meta` structures
- Run `go run . capture --format jsonl-compact` and confirm the output promotes `number`, `time`, `src`, `dst`, `protocol`, `length`, `info`, `stream_id`, and timing fields to the top level
- Run `go run . capture --format summary` and compare the emitted rows to tshark or Wireshark packet-list columns for the same traffic sample
- Compare TCP rows for `src`, `dst`, `protocol`, `length`, `info`, retransmission-style flags, and stream grouping fields
- Compare DNS rows for query/response summaries and transaction correlation fields
- Compare DISCO rows for control-plane subtype readability and Tailscale-specific nested metadata retention
