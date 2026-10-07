## 1. Summary Model and Aggregation

- [x] 1.1 In `internal/capture`, define the versioned summary projection, finite category vocabularies, fixed retention limits, omission counters, and capture-wide aggregation. Reuse decoded `path_id` (wrapper bytes 0-1, little-endian), packet origin, frame length, and DISCO classification without modifying wrapper parsing.
- [x] 1.2 Add colocated tests for empty/mixed captures, malformed records, captured-byte totals including wrapper bytes, known/unknown `path_id` values, protocol/origin buckets, and nonmonotonic timestamps.
- [x] 1.3 In `internal/capture`, implement first-100 canonical conversation retention, endpoint IP interning, bidirectional counts, relative timing, SYN/FIN/RST observations, and overflow accounting. Keep IP version and transport protocol in identity and avoid client/server inference.
- [x] 1.4 Add conversation tests for IPv4/IPv6, reversed direction, tuple reuse, analyzer eviction, full-key identity, endpoint caps, the 101st conversation, return traffic to retained conversations, and records without identity.

## 2. DNS and Finding Evidence

- [x] 2.1 In `internal/capture`, add capture-wide DNS packet/response-code/latency statistics and separately capped query/response examples using existing `status`, `peer_frame_number`, questions, and answers; do not add a second transaction correlator.
- [x] 2.2 Add DNS tests for matched/repeated/unmatched responses, missing or negative latency, 20-example limits, 4-item question/answer limits, and omission accounting without timeout or unique-transaction claims.
- [x] 2.3 In `internal/capture`, implement fixed-category finding counts, per-packet category deduplication, first-three evidence retention, UTF-8-safe text limits, and omission metadata. Exclude raw fields and unrestricted packet text while preserving frame references and selected TCP evidence.
- [x] 2.4 Add evidence tests for flag/annotation duplication, anomalies after conversation capacity, decode errors, category overflow, retained-text truncation, and absence of raw hex, previews, checksums, options, and DNS authority/additional content.

## 3. Export and CLI Integration

- [x] 3.1 In `internal/capture`, add the summary exporter around `StreamRecords`, finalize relative timestamps and deterministic ordering only after successful scanning, and propagate context/read/write errors without changing `ExportJSON`.
- [x] 3.2 Add exporter tests for stable repeated output, empty arrays, valid JSON, read/write/cancellation failures, and capped accumulator collections. Add a reproducible routine 1,000-packet fixture asserting summary size is at most 20 percent of detailed size with matching global packet/byte counts.
- [x] 3.3 In `cmd/convert.go`, add `--mode detailed|summary`, validate incompatible raw inclusion before output handling, and route to the correct exporter while reusing same-file protection and atomic publication.
- [x] 3.4 In `cmd/convert_test.go`, test default/explicit detailed compatibility, summary default/custom/stdout destinations, invalid modes/raw combinations, unsupported PCAP and PCAPNG, aliased input/output, truncated input, cancellation, preservation of existing output, and temporary-file cleanup.

## 4. Documentation and Verification

- [x] 4.1 Update `README.md` with summary-mode examples, retained information and fixed caps, privacy warnings, and detailed-export fallback; keep manual upload distinct from `analyze --input`.
- [x] 4.2 Update `docs/capture-output.md` with the aggregate schema and count/direction/time semantics, heuristic caveats, omission accounting, byte-size-versus-token distinction, and the existing analyzer memory limitation; distinguish aggregate conversion from live per-packet summary rows.
- [x] 4.3 Run `go test -race ./...`, `go run . convert --help`, `git diff --check`, and `openspec validate add-convert-summary-mode --strict`; verify documentation examples match the CLI and record the fixture's measured size reduction without claiming exact token savings.
