## Context

`thresher` already decodes Tailscale wrapper metadata, DISCO control traffic, inner IP/TCP/UDP/DNS payloads, and some stream-aware analysis state. The main remaining gap is presentation: the current JSONL is rich but not shaped like the packet-list view users expect from Wireshark or tshark, so common questions such as "who is talking to whom", "what stream is this packet in", and "why is this TCP packet suspicious" still require reading nested structures rather than scanning the top-level record.

This change is intentionally additive. The nested decode remains the source of truth, while a new normalized packet-row layer makes the output easier to scan, grep, and compare to Wireshark. The same decoded packet needs to support three distinct consumption modes: full JSONL, compact JSONL, and a human-readable summary row.

## Goals / Non-Goals

**Goals:**
- Add Wireshark-like top-level packet columns such as number, time, src, dst, protocol, length, and info to every record when the data is available
- Preserve explicit separation between outer capture metadata, Tailscale path metadata, DISCO/control metadata, and inner decoded traffic
- Extend stream modeling with a stable stream identifier, per-stream packet numbering, per-stream relative timing, and normalized transport direction
- Improve TCP, DNS, and DISCO info strings so summary output resembles tshark/Wireshark packet rows while remaining structured in JSON modes
- Add output modes for detailed JSONL, compact JSONL, and one-line summary rows without duplicating decode logic
- Add payload usability fields such as top-level payload length and safe printable previews while avoiding misleading treatment of encrypted payloads

**Non-Goals:**
- Removing or flattening the detailed nested decode that already exists
- Decrypting or heuristically interpreting encrypted application payloads such as SSH
- Reproducing the full Wireshark UI or every protocol dissector/expert-info rule
- Changing the Tailscale wrapper, DISCO wire format, or LocalAPI transport semantics

## Decisions

### Output model: add a normalized packet-row layer on top of the existing record

**Decision**: Extend the existing `Record` structure with a packet-row section of normalized top-level fields rather than replacing the current nested layout. The top-level record will carry the common Wireshark-style columns (`number`, `time`, `src`, `dst`, `protocol`, `length`, `info`) plus stream-oriented fields, while nested `inner`, `disco_meta`, and path metadata remain intact.

**Rationale**: The current nested decode is already useful for exact protocol detail and Tailscale-specific structure. Replacing it would lose fidelity and break jq/grep workflows that depend on explicit structure. A normalized row layer makes the common case readable without sacrificing the detailed model.

**Alternatives considered:**
- Replace the existing JSON schema with a flat packet-row schema: rejected because it would make Tailscale-specific and protocol-specific detail harder to consume safely
- Keep only nested fields and improve documentation: rejected because users still need a scan-friendly first view in the actual output

### Formatting architecture: derive all output modes from one enriched record

**Decision**: Keep one enrichment pipeline that produces a single fully populated record, then render it through separate formatters for `jsonl`, `jsonl-compact`, and `summary`.

**Rationale**: Output modes differ mostly in presentation, not in decode logic. A shared enriched record keeps behavior consistent and avoids subtle drift where one mode gains fields or heuristics that another mode does not.

**Alternatives considered:**
- Separate decode-and-render code paths for each format: rejected because they would duplicate logic and diverge quickly
- Compact mode as a post-processing jq example only: rejected because the CLI needs a supported native output mode

### Stream identity: expose both stable IDs and readable grouping keys

**Decision**: Continue using a stable stream identifier for grouped transport flows, rename the presentation to `stream_id`, and optionally expose a human-readable `conversation_key` for debugging and downstream grouping. Add `stream_packet_number`, `time_since_stream_start`, and `time_since_previous_in_stream` at the top level.

**Rationale**: `flow_id` is implementation-oriented, while `stream_id` better matches Wireshark terminology. Per-stream numbering and relative timing make it possible to follow a conversation at a glance without reconstructing it externally.

**Alternatives considered:**
- Keep only `flow_id`: rejected because it is less intuitive for users expecting Wireshark-style conversations
- Use a monotonic packet number per stream without a stable shared ID: rejected because grouping would remain awkward in JSON tooling

### Direction modeling: keep Tailscale path direction and add transport-relative direction

**Decision**: Preserve Tailscale path direction exactly as decoded from the wrapper (`FromPeer`, synthesized directions, `Disco frame`), and add a separate stream-relative direction such as `client_to_server` and `server_to_client` based on the first observed packet in the stream.

**Rationale**: Tailscale wrapper direction and transport conversation direction answer different questions. Conflating them would make it harder to debug whether a packet is local, synthesized, peer-originated, or simply the reverse direction of a TCP exchange.

### Info-string generation: protocol-aware summaries with machine-readable flags

**Decision**: Generate protocol-specific `info` strings from structured fields rather than storing presentation strings as the only truth. TCP info should include ports, flags, seq/ack, window, length, and detected annotations. DNS info should resemble standard query/response phrasing. DISCO info should include message subtype and peer endpoint context.

**Rationale**: Users want Wireshark-like rows, but downstream tools still need structured flags for filtering and testing. Deriving `info` from structured fields keeps both aligned.

**Alternatives considered:**
- Free-text `info` only: rejected because it is hard to validate and harder for jq/LLMs to reason about safely
- Structured flags only, no readable info string: rejected because human scanability is a primary goal of this change

### Payload exposure: add top-level usability fields without pretending encrypted payloads are decodable

**Decision**: Promote payload length and optional safe ASCII previews to top-level fields while retaining nested payload hex under the specific protocol structures. Only emit previews for printable/safe bytes and never imply semantic meaning for encrypted payloads.

**Rationale**: A short preview helps quickly identify DNS names, HTTP fragments, and plain-text control data, but encrypted traffic like SSH must remain framed rather than misrepresented.

### CLI surface: add `--format` to capture and reader-style entry points

**Decision**: Add a `--format` flag with `jsonl`, `jsonl-compact`, and `summary` values. `jsonl` remains the default detailed mode. `jsonl-compact` emits the normalized columns plus nested sections. `summary` emits tshark-like one-line rows.

**Rationale**: The default must stay lossless and automation-friendly, while summary-oriented consumption should be one flag away for manual debugging.

## Risks / Trade-offs

- **Top-level fields could drift from nested truth** → Mitigation: derive normalized fields from decoded/nested structures in one place and add tests that assert both stay aligned
- **Adding multiple output modes increases surface area** → Mitigation: use one enriched record model and narrow formatter responsibilities to rendering only
- **Wireshark-style analysis names may imply parity with Wireshark heuristics** → Mitigation: keep explicit machine-readable flags, document supported meanings, and avoid claiming exhaustive parity
- **Readable summary rows may hide Tailscale-specific context** → Mitigation: preserve path metadata and DISCO structure in JSON modes and document how summary rows map back to full records
- **ASCII previews may expose noisy or misleading bytes** → Mitigation: gate previews behind safe-printable checks and keep hex as the authoritative payload view

## Migration Plan

- Extend the enriched packet model with normalized top-level packet-row fields and stream presentation fields
- Add formatter implementations for detailed JSONL, compact JSONL, and summary row output
- Update CLI flags and docs to expose the new formats and explain how they map to Wireshark-like packet columns
- Add fixtures and regression tests for SSH/TCP, DNS, DISCO, and duplicate-ACK style traffic in each supported output mode
- Verify with `go test ./...` and targeted manual comparisons between `thresher` output and Wireshark/tshark views of the same captures

## Open Questions

- Should `summary` output include fixed-width columns for terminal readability, or stay simple whitespace-separated rows for easier piping?
- Should `jsonl-compact` omit some bulky nested fields such as full raw hex by default, or remain structurally complete with only normalized columns promoted?
- Is `TSMP/DISCO` the right normalized protocol label for control traffic, or should DISCO stay distinct from broader Tailscale control-plane labeling?
