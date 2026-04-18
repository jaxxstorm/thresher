## Why

The recent capture-output work made `thresher` much easier to scan, but a few packet-analyzer-grade behaviors are still missing or inconsistent. This change is needed now to close the last usability gaps so the output behaves more like a real packet analyzer while preserving the nested Tailscale-aware decode as the authoritative structure.

## What Changes

- Populate `dst` for DISCO packets whenever the destination can be inferred from decoded control-frame fields such as pong source or candidate endpoint information
- Make key TCP analyzer conditions explicit and stable as machine-readable flags, including `retransmission`, `fast_retransmission`, `out_of_order`, `previous_segment_not_captured`, and `zero_window`
- Add a dedicated compact packet-list presentation mode that renders one row per packet in a tshark-style format for quick scanning
- Preserve whether each packet was captured directly or synthesized from the wrapper path in a dedicated top-level field rather than forcing consumers to infer it from `path`
- Promote optional per-stream relative TCP sequence and acknowledgment normalization fields to the top level while keeping nested transport metadata unchanged
- Keep the nested decode contract intact and additive rather than flattening or replacing it

## Non-goals

- No removal or restructuring of the existing nested `inner`, `analysis`, or `disco_meta` decode trees
- No changes to the Tailscale debug capture wrapper, including the 2-byte little-endian `path_id`, SNAT length/value byte, DNAT length/value byte, or DISCO wire framing
- No attempt to match every Wireshark expert-info heuristic or UI behavior
- No decryption or semantic interpretation of encrypted application payloads

## Capabilities

### New Capabilities

<!-- None -->

### Modified Capabilities

- `go-packet-parser`: refine normalized packet-row fields for DISCO destinations, synthesized-vs-captured metadata, and top-level relative TCP normalization fields
- `stateful-flow-analysis`: make key TCP analyzer flags explicit and stable for analyzer-grade packet inspection
- `capture-output-modes`: add a dedicated packet-list-oriented mode and clarify row-based presentation expectations for analyzer-style output

## Impact

- **Capture record contract**: packet records gain additional top-level analyzer-oriented fields while leaving the nested decode unchanged
- **Analyzer logic**: TCP heuristic output and DISCO endpoint derivation expand in `internal/capture`
- **Formatting layer**: output-mode rendering will need a clearer packet-list presentation path for tshark-style scanning
- **Tests and docs**: regression coverage will need to assert DISCO destination derivation, explicit analyzer flags, synthesized/captured metadata, and packet-list rendering behavior
