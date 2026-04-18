## 1. Packet Row Refinements

- [x] 1.1 Extend the top-level capture record in `internal/capture` with a dedicated captured-vs-synthesized origin field derived from the 2-byte little-endian `path_id`
- [x] 1.2 Refine DISCO row population so `dst` is filled when endpoint information can be derived from decoded control-frame fields such as `Pong` source or supported candidate endpoint structures
- [x] 1.3 Promote relative TCP normalization values to top-level packet fields while leaving nested `inner.tcp` values unchanged
- [x] 1.4 Add tests covering explicit packet origin, promoted relative TCP fields, and DISCO destination derivation without regressing the nested decode

## 2. Analyzer Flags

- [x] 2.1 Add explicit stable analyzer-grade fields for `retransmission`, `fast_retransmission`, `out_of_order`, `previous_segment_not_captured`, and `zero_window` sourced from existing analyzer logic in `internal/capture/analyzer.go`
- [x] 2.2 Ensure the explicit analyzer fields stay consistent with the existing annotation list rather than replacing it
- [x] 2.3 Add tests covering each explicit analyzer field and its relationship to the existing annotations on representative TCP sequences

## 3. Packet-List Output Mode

- [x] 3.1 Add a dedicated packet-list-oriented format mode in the formatter layer that renders one tshark-style row per packet from the shared enriched record model
- [x] 3.2 Wire the new packet-list mode through the CLI format parsing and help text without disturbing the existing JSON-oriented modes
- [x] 3.3 Add tests covering packet-list rendering for TCP, DNS, and DISCO packets plus unsupported format handling

## 4. Verification And Docs

- [x] 4.1 Update capture-output documentation to explain the new packet origin field, explicit analyzer flags, and the packet-list mode
- [x] 4.2 Add or update fixtures for DISCO packets with derivable destinations and TCP sequences that trigger the explicit analyzer-grade flags
- [x] 4.3 Run `go test ./...` and add a manual verification note for comparing packet-list mode against tshark-style packet rows

### Manual Verification Note

- Run `go run . capture --format packet-list` and confirm each packet renders as a single tab-separated row suitable for tshark-style scanning
- Compare `src`, `dst`, `protocol`, `length`, and `info` columns to tshark output for the same traffic sample
- Verify `packet_origin` in JSON modes lines up with the wrapper `path_id` semantics for captured versus synthesized packets
- Verify DISCO `Pong` frames populate a top-level `dst` from the decoded pong source endpoint when present
- Verify explicit analyzer fields such as `analysis.retransmission`, `analysis.fast_retransmission`, `analysis.out_of_order`, `analysis.previous_segment_not_captured`, and `analysis.zero_window` align with the annotation list on representative TCP sequences
