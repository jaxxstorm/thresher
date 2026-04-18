## 1. Analyzer Structure

- [x] 1.1 Add a stateful analyzer stage in `internal/capture` that enriches decoded packet records after wrapper parsing and inner protocol decoding
- [x] 1.2 Define bounded analyzer state for active conversations and recent history keyed from decoded protocol, IP family, addresses, and transport ports
- [x] 1.3 Add unit tests for analyzer lifecycle and state eviction across long-running frame sequences

## 2. Flow Identity And TCP Analysis

- [x] 2.1 Extend packet record structs with stable `flow_id`, per-direction flow metadata, and analysis annotation fields
- [x] 2.2 Implement canonical bidirectional flow keys for TCP and UDP packets using decoded `inner.src_ip`, `inner.dst_ip`, `inner.src_port`, and `inner.dst_port`
- [x] 2.3 Add tests that verify opposite-direction packets share one `flow_id` and unrelated conversations do not
- [x] 2.4 Implement relative TCP sequence and acknowledgment calculations using the first observed sequence number per flow direction
- [x] 2.5 Add tests covering absolute and relative TCP values for multi-packet conversations
- [x] 2.6 Implement TCP packet annotations for retransmission, out-of-order delivery, unseen predecessor data, and duplicate ACK behavior
- [x] 2.7 Add fixture-driven tests for retransmission, out-of-order, missing-segment, and duplicate-ACK annotation scenarios

## 3. Detailed Packet Output

- [x] 3.1 Extend non-DISCO JSON output with frame-level and inner-payload raw hex fields while preserving existing wrapper fields including `path_id`, SNAT length/value, and DNAT length/value semantics
- [x] 3.2 Expand TCP, UDP, IP, and ICMP metadata fields so packet records retain checksums, header sizing, payload bytes, and packet summaries needed for downstream debugging
- [x] 3.3 Add tests that verify the expanded JSON contract for TCP, UDP, ICMP, and structured decode-failure records

## 4. DNS Transaction Correlation

- [x] 4.1 Implement DNS transaction keys from canonical flow identity, DNS ID, opcode, and question tuples
- [x] 4.2 Enrich DNS packet records with transaction identifiers, peer frame references, and transaction-aware summaries for matched and unmatched exchanges
- [x] 4.3 Add multi-frame tests that cover matched DNS request/response pairs, unmatched responses, and repeated DNS IDs on different flows

## 5. Pipeline Integration And Verification

- [x] 5.1 Integrate the analyzer stage into capture streaming so JSONL emission includes stateful annotations for live LocalAPI captures
- [x] 5.2 Ensure per-packet errors preserve frame number, timestamp, and any already-decoded frame metadata instead of dropping context
- [x] 5.3 Run `go test ./...` and add a targeted manual smoke test plan for `go run . capture` that validates flow grouping and DNS correlation in live output

### Manual Smoke Test Plan

- Start a live capture with `go run . capture -o /tmp/thresher-capture.jsonl`
- Generate bidirectional Tailnet traffic plus at least one DNS query/response during the capture window
- Stop the capture with `Ctrl-C` and inspect `/tmp/thresher-capture.jsonl`
- Verify related TCP frames share the same `flow_id` and include relative TCP fields plus any analysis annotations
- Verify DNS request/response packets share `inner.dns.transaction_id` and that matched responses include `inner.dns.peer_frame_number`
