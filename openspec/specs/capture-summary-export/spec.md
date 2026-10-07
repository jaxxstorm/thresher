# capture-summary-export Specification

## Purpose
Define offline, bounded, deterministic summary exports of Tailscale USER0 captures, including aggregation, omission semantics, and conversion failure safety.

## Requirements

### Requirement: Opt-in offline summary export
The convert command SHALL accept `--mode detailed|summary`, default to detailed, and preserve existing detailed output and output-path behavior. Summary mode SHALL operate offline on supported Tailscale USER0 PCAPs. Unknown modes and summary combined with `--include-raw=true` MUST fail before output creation or modification. Live capture formats SHALL remain unchanged.

#### Scenario: Default compatibility
- **WHEN** the same capture is converted without a mode and with `--mode detailed`
- **THEN** both outputs preserve the existing detailed schema and raw-data flag behavior

#### Scenario: Summary file or stdout
- **WHEN** a valid capture is converted with `--mode summary`
- **THEN** the command writes one aggregate JSON document to the default sibling `.json` file, the specified `-o` path, or stdout for `-o -`, without contacting a network service

#### Scenario: Invalid options
- **WHEN** an unknown mode or summary with raw inclusion is requested
- **THEN** the command returns an actionable error and leaves any existing output untouched

### Requirement: Versioned capture-wide observations
Summary JSON SHALL contain `schema_version: 1`, `mode: "summary"`, capture totals, endpoints, conversations, DNS observations, findings, applied limits, omissions, and explanatory notes. It SHALL omit the full packet array. Every record, including decode errors, SHALL contribute once to packet count and captured bytes, defined as the sum of frame lengths including wrapper bytes. Totals SHALL include timestamp range, duration, protocol/path/origin counts, DISCO count, and decode-error count. Category counters SHALL use finite known categories with other/unknown buckets.

#### Scenario: Mixed capture
- **WHEN** a capture contains TCP, UDP, DISCO, and malformed packets
- **THEN** totals account for all records, malformed packets contribute to decode-error totals, and no malformed record is silently discarded

#### Scenario: Empty capture
- **WHEN** a valid capture has no packets
- **THEN** output is valid JSON with zero packet/byte/duration totals, empty arrays, and absent start/end timestamps

#### Scenario: Nonmonotonic timestamps
- **WHEN** timestamps regress in capture order
- **THEN** capture start/end use minimum/maximum timestamps, relative times use milliseconds from capture start, and evidence selection still follows frame order

### Requirement: Canonical conversation aggregation
The summary SHALL retain the first 100 distinct canonical conversation keys in frame order and intern only their endpoint IPs, with at most 200 endpoint entries. It SHALL aggregate by full conversation key rather than only a hash and retain stream IDs for reference. Each retained conversation SHALL include canonical A/B endpoints and ports, protocol/IP version, frame/time bounds, directional packet/captured-byte counts, path/origin counts, and observed TCP SYN/FIN/RST packet counts. It MUST NOT equate conversation identity with a connection lifetime or canonical direction with actual client/server roles.

#### Scenario: Bidirectional traffic and tuple reuse
- **WHEN** packets share a canonical endpoint tuple across both directions and later reuse that tuple
- **THEN** one conversation summary accumulates those observations with separate A-to-B and B-to-A counts and without claiming multiple established sessions

#### Scenario: Conversation capacity
- **WHEN** a 101st distinct conversation appears and packets subsequently return to a retained conversation
- **THEN** the new conversation is not retained, its packets contribute to global totals and omitted-conversation-detail packet counts, and the retained conversation continues accumulating its packets

#### Scenario: No conversation identity
- **WHEN** DISCO or malformed records have no usable conversation key
- **THEN** they remain in global totals and are counted separately as packets without conversation identity rather than fabricated conversations

### Requirement: DNS observation summaries
The summary SHALL count query, response, matched-response, and unmatched-response packets and response codes across the full capture. It SHALL provide count/min/max/mean latency for nonnegative matched-response latency observations and separately count negative latency observations. It SHALL retain the first 20 query examples and first 20 response examples, each with at most 4 questions and 4 answers, frame/time references, and available matched-request frame and latency. It MUST NOT infer unanswered queries, timeouts, or unique completed transactions from existing analyzer statuses.

#### Scenario: Matched and repeated responses
- **WHEN** the analyzer reports two matched responses referencing the same query frame
- **THEN** two matched response packets are counted, examples reference the supplied query frame, and the summary does not claim two distinct completed transactions

#### Scenario: Unmatched response and invalid timing
- **WHEN** a response is unmatched or a matched response has negative latency
- **THEN** unmatched responses remain visible without a timeout claim and negative latency is excluded from nonnegative latency aggregates and counted separately

#### Scenario: DNS evidence cap
- **WHEN** query or response examples exceed 20 or a retained example exceeds 4 questions or answers
- **THEN** the first allowed examples/items are retained and remaining example/item counts are disclosed without reducing capture-wide DNS packet totals

### Requirement: Bounded diagnostic evidence
The summary SHALL count existing analyzer finding categories across all records and retain the first 3 examples per category, independently of conversation retention. It SHALL use a fixed vocabulary for existing TCP annotations, partial history, decode errors, and other analysis. A category represented by both a flag and annotation on one packet SHALL count once. Examples SHALL retain frame/time references, optional stream identity, selected TCP flags/sequence/ack/window values, or a bounded decode-error message. Findings MUST be described as heuristic observations, not proof of root cause or packet loss.

#### Scenario: Later anomaly outside conversation capacity
- **WHEN** routine traffic fills conversation retention and a later non-retained conversation contains a retransmission observation
- **THEN** the finding count and available evidence slot still capture that observation

#### Scenario: Repeated finding
- **WHEN** five packets each contain both a retransmission boolean and equivalent annotation
- **THEN** the category count is five, its first three examples are retained, and two omitted examples are reported

### Requirement: Explicit compression and omission semantics
Summary output SHALL exclude raw bytes, payload previews, per-packet dumps, arbitrary packet summary text, checksums, TCP option dumps, and DNS authority/additional records. Input-derived text fields SHALL be limited to 256 Unicode code points without invalid UTF-8. The document SHALL report applied limits and exact omission counts for conversation-detail packets, finding and DNS examples, DNS list items within retained examples, and truncated text fields. It MUST NOT report an exact number of excluded distinct conversations without tracking those identities. Metadata privacy and lossy selection SHALL be documented.

#### Scenario: Oversized text
- **WHEN** a retained DNS or error field exceeds the text limit
- **THEN** output retains a valid UTF-8 prefix within the limit and increments its text-truncation count

#### Scenario: Raw data and sensitive metadata
- **WHEN** summary mode processes payload-bearing packets and named DNS answers
- **THEN** raw payload content and previews are absent, decoded names can remain, and documentation warns that summary mode is not anonymization

### Requirement: Deterministic and measurably smaller output
For identical input and options, the exporter SHALL produce byte-identical output, using stable identifiers, ordering, and no wall-clock generation values. Its new accumulator SHALL retain only capped detail and fixed-category counters, without retaining all packet records or all conversation identities. The implementation SHALL include a reproducible 1,000-packet routine single-conversation fixture whose summary serialized size is at most 20 percent of detailed export size. Documentation SHALL distinguish this byte-size proxy from exact model tokens and SHALL state that existing analyzer memory is outside the accumulator bound.

#### Scenario: Repeat conversion
- **WHEN** the same fixture is converted twice in summary mode
- **THEN** output bytes match exactly

#### Scenario: Repetitive capture reduction
- **WHEN** the documented 1,000-packet fixture is exported in detailed and summary modes
- **THEN** summary bytes are at most one fifth of detailed bytes without changing global packet or captured-byte totals

### Requirement: Preserve conversion failure safety
Summary mode SHALL preserve same-file protection, supported-link-type validation, error propagation, and atomic file publication. Packet decode errors SHALL be retained as observations, while invalid containers, read errors, cancellation, and write failures SHALL fail conversion. A summary SHALL be finalized only after successful input scanning. File failures MUST preserve existing destination content and clean up temporary files; stdout write failures MAY leave partial JSON and SHALL be documented.

#### Scenario: Read failure or cancellation
- **WHEN** input is truncated or conversion is canceled before successful finalization
- **THEN** conversion returns an error and an existing destination file is unchanged with no leaked temporary output

#### Scenario: Unsupported or aliased input
- **WHEN** input uses an unsupported link type or output identifies the input file directly or through an alias
- **THEN** conversion fails without modifying the input

#### Scenario: Output write failure
- **WHEN** the output writer fails during encoding
- **THEN** conversion returns the write error rather than reporting success
