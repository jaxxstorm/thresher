## Context

`thresher` now decodes the Tailscale wrapper and several inner protocols, but it still treats each frame as an isolated event. That is enough for basic inspection, but not for debugging transport failures: useful signals such as relative TCP sequence numbers, missing predecessor data, retransmissions, and request/response correlation only emerge when packets are interpreted as part of a flow over time.

The new requirement is to preserve enough of the diagnostic value of packet-capture tooling in native JSONL so a consumer can reason about failures without external Wireshark state. The Tailscale wrapper and DISCO formats remain fixed; the change is entirely in how decoded inner traffic is modeled and how cross-frame state is maintained while a capture stream is processed.

## Goals / Non-Goals

**Goals:**
- Assign stable flow identifiers to packets so humans and LLMs can group related frames without re-deriving 5-tuples
- Track TCP conversations across frames and emit relative sequence/acknowledgment numbers alongside absolute values
- Annotate TCP packets with packet-level analysis such as retransmission, out-of-order delivery, unseen-segment dependencies, and ACK relationships based on observed history
- Correlate DNS requests and responses into a shared transaction identity and add transaction-aware summaries to packet records
- Preserve enough raw packet material in JSON output to allow later offline reasoning when structured decoding is insufficient
- Keep the analysis implementation reusable for both live capture and future file-based ingestion paths

**Non-Goals:**
- Full TCP stream reassembly into application sessions or byte streams
- Reproducing every Wireshark expert-info rule or protocol dissector
- Changing the Tailscale debug capture wrapper, DISCO frame layout, or LocalAPI transport
- Introducing external analyzers, Lua, tshark, or Wireshark as runtime dependencies

## Decisions

### Flow analysis: add a stateful analyzer stage after stateless packet decode

**Decision**: Keep wrapper parsing and layer decoding as the first stage, then pass decoded packet records through a second analyzer stage that can enrich them with flow IDs, relative values, and annotations before JSON encoding.

**Rationale**: The current decoder already converts bytes into structured packet records. Stateful analysis is easier to reason about, test, and bound if it consumes those records rather than interleaving transport heuristics with byte parsing. This separation also keeps future `read` support aligned with live capture.

**Alternatives considered:**
- Fold flow tracking directly into byte parsing logic: rejected because it mixes per-packet decode with long-lived state and makes testing multi-packet heuristics harder
- Post-process emitted JSON externally: rejected because the goal is for `thresher` itself to produce a debugging-grade artifact

### Flow identity: use canonical conversation keys plus emitted stable flow IDs

**Decision**: Identify conversations internally with canonical keys derived from protocol, IP family, source/destination addresses, and source/destination ports. Emit a stable `flow_id` field plus direction-specific fields so each packet can be grouped to a conversation while still retaining the original packet direction.

**Rationale**: Canonical keys let both halves of a TCP or UDP exchange map to one conversation. An emitted `flow_id` removes the need for downstream tools or LLMs to reconstruct that grouping.

**Alternatives considered:**
- Directional 5-tuples only: rejected because request and response packets would appear as separate flows
- Hash raw packet bytes: rejected because grouping should remain stable across packets in the same conversation, not per frame

### TCP analysis: emit both absolute and relative transport values

**Decision**: Preserve absolute TCP header values, but also compute relative sequence and acknowledgment numbers per direction using the first observed sequence number in each side of a conversation as the base. Track the next expected sequence number and previously acknowledged ranges so packet records can include annotations such as retransmission, out-of-order, unseen-segment dependency, duplicate ACK, and ACKing previously unseen data.

**Rationale**: Relative values are far easier to inspect and match the mental model exposed by packet-capture tools. Keeping the absolute numbers avoids losing fidelity for consumers that need the raw wire values.

**Alternatives considered:**
- Absolute values only: rejected because they are harder to interpret and do not close the usability gap with Wireshark
- Relative values only: rejected because they discard wire-level fidelity and make reconciliation harder

### DNS correlation: maintain transaction state keyed by flow plus DNS ID

**Decision**: Track DNS exchanges with a transaction key composed of the canonical flow, DNS ID, opcode, and question tuple(s). Emit a `transaction_id` on both request and response packets, attach the peer frame number when known, and expose request/response timing when both sides have been observed.

**Rationale**: DNS IDs alone are not sufficient in busy captures, while full packet matching is brittle. Flow-aware transaction keys keep the correlation deterministic enough for debugging while remaining cheap to compute.

**Alternatives considered:**
- DNS ID alone: rejected because IDs can repeat across conversations
- Full payload fingerprinting: rejected because it is more fragile and provides little benefit over tuple-based correlation

### Output model: enrich records with structured analysis plus raw hex fallbacks

**Decision**: Extend packet records with additional structured fields for flow metadata, analysis annotations, relative values, and correlation references, while also retaining frame-level and payload-level hex fields.

**Rationale**: Structured fields make the common debugging cases queryable and LLM-friendly. Raw hex preserves lossless evidence when the structured model does not yet cover a protocol nuance.

**Alternatives considered:**
- Raw bytes only: rejected because consumers would need to re-dissect packets to answer common questions
- Summary strings only: rejected because they are readable but underspecified for machine reasoning

### State management: keep bounded in-memory caches with explicit eviction

**Decision**: Maintain bounded analyzer state for active TCP flows, UDP/DNS transactions, and recently completed conversations. Use frame-count or time-based eviction so long-running live captures do not grow memory unbounded.

**Rationale**: Stateful analysis is required, but live capture can run indefinitely. Explicit eviction preserves runtime safety while still capturing the recent history needed for packet annotations.

**Alternatives considered:**
- Unbounded state: rejected because a long-running capture would eventually exhaust memory
- Aggressive one-packet lookback only: rejected because many useful annotations depend on longer conversation history

## Risks / Trade-offs

- **Heuristics will not exactly match Wireshark in every edge case** → Mitigation: document which annotations are supported, keep raw and relative values side by side, and add fixture tests for representative failure patterns
- **Stateful analysis increases memory and implementation complexity** → Mitigation: isolate analyzer state behind a small package boundary and add explicit eviction rules with tests
- **Packet loss in the source capture can make analysis ambiguous** → Mitigation: expose uncertainty as annotations such as unseen-segment or incomplete-transaction instead of silently inferring certainty
- **Relative numbering depends on the first observed packet, not necessarily the connection start** → Mitigation: include both absolute and relative values and mark flows with partial-history analysis when bases are inferred mid-stream
- **DNS correlation can mis-associate repeated IDs under unusual traffic patterns** → Mitigation: scope transactions by canonical flow and question tuple, not DNS ID alone

## Migration Plan

- Extend the decoder output structs with new analysis and correlation fields in a backward-compatible JSON expansion
- Introduce a stateful analyzer layer into the capture pipeline before JSON encoding
- Add fixture-driven tests for TCP conversations, retransmission/out-of-order scenarios, unseen-segment annotations, and DNS request/response correlation
- Verify with `go test ./...` and targeted `go run . capture` smoke tests against a running `tailscaled`
- If rollback is needed, remove the analyzer stage and fall back to the stateless decoder while leaving the wrapper parsing unchanged

## Open Questions

- Should flow IDs be deterministic hashes of canonical flow keys, or simpler monotonic conversation IDs scoped to a single capture run?
- What eviction thresholds are reasonable for long-running captures without weakening retransmission and correlation analysis too aggressively?
- Should annotations be emitted as enumerated machine-readable codes, free-text summaries, or both?
