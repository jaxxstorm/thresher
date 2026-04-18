## Context

`thresher` already emits a useful Wireshark-style packet row layered on top of the nested packet decode, along with stream IDs, relative timing, and multiple output modes. The remaining gaps are now narrow but important: some analyzer-grade flags are not exposed consistently as stable top-level fields, DISCO packets still omit destination context in cases where it is derivable, synthesized-vs-captured state is only implicit in the wrapper path, and the current summary mode is close to tshark output but not yet a dedicated packet-list rendering path.

This change is intentionally a refinement rather than a redesign. The nested decode stays unchanged. The work is about making the top-level presentation, stream fields, and row output behave more like a real packet analyzer while keeping the current Tailscale-aware structure intact.

## Goals / Non-Goals

**Goals:**
- Expose DISCO destinations at the top level whenever they can be derived from decoded control-frame fields without inventing data
- Promote analyzer-grade TCP conditions into explicit, stable packet fields in addition to any existing analysis annotation list
- Add a dedicated packet-list rendering mode that is clearly distinct from the richer JSON modes and optimized for one-row-per-packet scanning
- Add a dedicated top-level field that states whether a packet was captured directly or synthesized from the wrapper `path_id`
- Promote optional relative TCP sequence and acknowledgment normalization fields to the top level for easier filtering and comparison

**Non-Goals:**
- No change to the nested `inner`, `analysis`, `disco_meta`, or wrapper decode structure
- No changes to the wire format described by the 2-byte little-endian `path_id`, address length bytes, or DISCO frame layout
- No speculative DISCO destination inference when the packet does not actually carry enough endpoint information
- No attempt to rework the entire formatter architecture or remove existing output modes

## Decisions

### Packet synthesis state: add a dedicated top-level field derived from `path_id`

**Decision**: Add a new top-level field such as `capture_origin` or `packet_origin` with explicit values like `captured` and `synthesized`, derived directly from the wrapper `path_id` values. `FromLocal` and `FromPeer` map to captured traffic; synthesized inbound/outbound rows map to synthesized traffic; DISCO remains captured unless the wrapper semantics say otherwise.

**Rationale**: Consumers should not need to parse human-readable `path` labels to answer whether a packet was observed directly or synthesized by Tailscale logic.

**Alternatives considered:**
- Keep inferring from `path` only: rejected because it is brittle and not jq-friendly
- Replace `path` with a normalized field: rejected because `path` remains useful and should coexist with the new explicit origin field

### DISCO destination derivation: derive only from concrete decoded endpoint fields

**Decision**: Populate top-level `dst` for DISCO packets only when it can be derived from concrete fields in the decoded control frame, such as `Pong` source address/port or structured endpoint/candidate endpoint payloads in future supported frame types.

**Rationale**: Analyzer-grade output should surface more information, but it should not guess. If a DISCO packet only exposes a source and transaction identity, the destination should remain empty rather than fabricated.

**Alternatives considered:**
- Guess destination from prior packets in the same conversation: rejected because DISCO correlation is weaker than transport streams and would introduce misleading output
- Leave DISCO destination empty in all cases: rejected because some frame types do carry enough endpoint detail to improve readability safely

### Analyzer flags: keep the annotation list and add stable boolean fields

**Decision**: Retain the existing annotation list for extensibility, but also add explicit boolean or normalized top-level fields for high-value TCP conditions such as `retransmission`, `fast_retransmission`, `out_of_order`, `previous_segment_not_captured`, and `zero_window`.

**Rationale**: These analyzer conditions are important enough to merit first-class fields. Boolean-style fields are easier to filter, test, and reason about than scanning an annotation array, while the annotation list can still carry future or lower-frequency values.

**Alternatives considered:**
- Annotation array only: rejected because it makes key analyzer conditions harder to consume programmatically
- Boolean fields only: rejected because the annotation list still provides flexibility for future heuristics

### Packet-list mode: add a dedicated row renderer rather than overloading summary semantics

**Decision**: Introduce a distinct `packet-list` output mode for one-row-per-packet tshark-style rendering and keep the existing `summary` mode behavior aligned or migrated deliberately rather than ambiguously overloading one label.

**Rationale**: A named packet-list mode makes the intent explicit and gives room to keep or refine summary-oriented behavior separately if needed. It also maps more clearly to the user’s request for a true packet-list output.

**Alternatives considered:**
- Reuse `summary` silently: rejected because it hides a behavior shift behind an existing format name
- Replace summary mode entirely: rejected because that is a broader breaking surface than needed for this refinement

### Relative TCP values: promote analyzer-derived values to the top level, sourced from nested truth

**Decision**: Keep relative TCP values in nested `inner.tcp`, but also promote `relative_seq` and `relative_ack` style fields to the top level when a TCP packet has them. The promoted fields must be derived from the same analyzer values already stored in the nested decode.

**Rationale**: Top-level promotion improves grep/jq usability for common transport debugging without altering the underlying nested truth.

## Risks / Trade-offs

- **Promoted top-level fields can drift from nested truth** → Mitigation: derive them in one enrichment step from the nested decode/analyzer result rather than recomputing them independently
- **Packet-list mode could duplicate summary mode semantics confusingly** → Mitigation: document the distinction clearly and use explicit format names in the CLI help and docs
- **DISCO destination derivation may be incomplete for some frame types** → Mitigation: only populate `dst` when derived from explicit decoded endpoint fields and leave unsupported frames blank
- **Analyzer-grade flags may be interpreted as exact Wireshark parity** → Mitigation: keep field names clear, document supported meanings, and avoid claiming exhaustive parity with Wireshark expert-info heuristics

## Migration Plan

- Extend the record enrichment layer with promoted analyzer flags, packet origin, and top-level relative TCP normalization fields
- Refine DISCO row population to fill `dst` when specific control-frame fields provide a safe derivation
- Add a dedicated packet-list output mode and document how it differs from JSON-oriented output
- Update tests and fixtures for DISCO destination derivation, analyzer flags, synthesized/captured origin, and packet-list rows
- Verify with `go test ./...` and manual `go run . capture --format ...` comparisons against tshark-style expectations

## Open Questions

- Should the existing `summary` format remain as-is alongside `packet-list`, or should `packet-list` become the preferred analyzer-style replacement?
- Should the explicit analyzer flags be top-level booleans, a nested `flags` object, or both?
- Which additional DISCO frame types beyond `Pong` provide enough structured endpoint data to support safe destination derivation immediately?
