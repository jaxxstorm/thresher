## Why

The current JSONL output decodes packets independently, which drops the cross-packet context that makes Wireshark useful for debugging Tailnet failures. This change is needed now because the project goal has shifted from basic packet export to producing capture artifacts rich enough for humans and LLMs to diagnose Tailscale behavior without opening Wireshark.

## What Changes

- Add stateful TCP flow analysis so packet records can be grouped into conversations and annotated with relative sequence and acknowledgment numbers
- Add packet-level analysis annotations for retransmissions, out-of-order delivery, missing predecessor data, and ACK relationships derived from prior frames
- Add DNS transaction correlation so requests and responses can be linked across frames and summarized as a single logical exchange
- Expand the JSON output contract for decoded packets to include stable flow identifiers, raw packet bytes, protocol-specific detail, and analysis summaries that preserve more of the signal present in packet-capture tooling
- Keep the Tailscale wrapper and DISCO wire format unchanged while extending the decoder pipeline to maintain per-flow state during capture processing

## Non-goals

- No changes to the Tailscale debug capture wrapper wire format or DISCO frame layout
- No full TCP stream reassembly into application payloads beyond metadata and packet-level annotations
- No attempt to reproduce all Wireshark dissectors or every expert-info heuristic for unsupported protocols
- No external Wireshark, tshark, or Lua dependency for deriving analysis results

## Capabilities

### New Capabilities

- `stateful-flow-analysis`: Track packet relationships across frames, assign stable flow identifiers, and emit conversation-level annotations for transport protocols
- `dns-transaction-correlation`: Correlate DNS requests and responses across frames and include transaction-aware summaries in packet output

### Modified Capabilities

- `go-packet-parser`: Expand decoded packet output from stateless per-packet metadata to richer structured protocol detail, raw bytes, and stateful analysis fields

## Impact

- **Parser code**: `internal/capture` will grow flow tracking, packet annotation, and transaction-correlation logic on top of the current decoder
- **JSON contract**: packet records will include additional required fields for flow identity, relative TCP analysis, packet annotations, and DNS correlation metadata
- **Testing**: fixture coverage will need multi-packet sequences that exercise retransmission, out-of-order, missing-segment, and DNS request/response scenarios
- **Runtime**: live capture processing will maintain bounded in-memory state across frames instead of treating each packet as isolated
