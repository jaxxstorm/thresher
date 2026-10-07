## Context

`convert` currently streams enriched records into a single JSON document via `ExportJSON` and `StreamRecords`. Raw bytes are optional, but endpoints, headers, and routine packet metadata still repeat. File publication already uses a temporary file and rename. The decoder accepts classic Tailscale USER0 PCAPs and the analyzer provides canonical conversation identity, TCP observations, and response-side DNS matching.

Conversation IDs do not distinguish tuple reuse. Transport direction labels derive from first observation, not proven client/server roles. DNS queries are not retroactively updated, responses can repeatedly match one query, and analyzer DNS state is currently unbounded. These constraints govern summary claims.

## Goals / Non-Goals

**Goals:**
- Reduce repeated information without an LLM or new dependency.
- Retain capture-wide counts and bounded diagnostic evidence with deterministic omission accounting.
- Preserve existing detailed export behavior and safe file publication.

**Non-Goals:**
- Exact token counting/budgets, automatic uploads, compact packet mode, reassembly, new capture formats, or analyzer repairs.
- Guaranteed whole-process memory bounds or inferred DNS failures, client/server roles, and root causes.

## Decisions

### 1. Opt-in mode with a separate projection

Add `--mode detailed|summary`, defaulting to `detailed`. Reject unknown modes and `summary` combined with `--include-raw=true` before opening output. Route summary through a new exporter consuming `StreamRecords`; reuse the command's existing publication path. Do not change the existing `ExportJSON` schema or live `capture --format summary` behavior.

Alternative: replace the default export or reuse live summary rows. Rejected because a lossy aggregate document is a different contract and existing packet consumers must remain unaffected.

### 2. Summary envelope and observation semantics

Use compact JSON with `schema_version: 1`, `mode: "summary"`, `capture`, `endpoints`, `conversations`, `dns`, `findings`, `limits`, `omissions`, and `notes`. The mode discriminates this schema from detailed export. Do not include a full `packets` array. Empty lists encode as arrays rather than null.

`capture` includes packet count, captured bytes (sum of `Record.FrameLength`, including wrapper bytes, not unique application bytes), minimum/maximum timestamps, duration, decode-error count, DISCO count, and protocol/path/origin counters. Unknown values use a fixed unknown bucket. Protocol categories are TCP, UDP, ICMPv4, ICMPv6, DISCO, other, and unknown; DNS has separate counters. Normalize other category maps to finite known buckets plus other/unknown rather than arbitrary strings. Every decoded or error record contributes exactly once to packet and byte totals. Empty captures omit timestamps and have zero duration.

Retain the first 100 distinct canonical conversation keys in capture order. Intern IP addresses from retained conversations into `e1`, `e2`, etc. on first retained use; keep ports on conversation endpoints. Use full canonical keys internally to avoid hash-collision aggregation; preserve analyzer stream IDs as references. Conversation output includes endpoints A/B, protocol/IP version, first/last frame, timestamp range relative to capture start, A-to-B/B-to-A packet and captured-byte counts, path/origin counters, and TCP SYN/FIN/RST packet counts. Count each flag once per packet, without claiming completed handshakes or sessions. Direction is canonical, never inferred client/server. Summaries merge tuple reuse and remain stable if analyzer state is evicted. DISCO and records without a usable conversation remain represented in capture totals, not fabricated conversations.

Compute relative milliseconds from the minimum capture timestamp at finalization, keeping absolute time once in capture metadata. Preserve sub-millisecond precision. Scan in frame order even when timestamps regress.

Alternative: exact top conversations by traffic volume. Rejected for initial scope because exact ranking requires retaining every conversation. First-seen selection has simple bounded state and explicit coverage limitations.

### 3. DNS statistics without invented transactions

Capture-wide DNS counters count observed query packets, response packets, matched response packets, unmatched response packets, and response codes. A finite response-code map includes an other bucket. Latency aggregates count/min/max/mean use only matched responses with nonnegative reported latency; count negative latency observations separately. Do not derive unanswered requests or transaction-success rates from query status or transaction IDs.

Retain the first 20 query examples and first 20 response examples, independently. Each contains frame, relative time, up to 4 questions (name/type/class), up to 4 answers (name/type/data), response code when applicable, and matched request frame/latency when supplied. This is evidence, not a comprehensive domain inventory. Preserve the analyzer's matching observations without introducing a second correlator.

Alternative: aggregate every unique domain or track pending requests. Rejected because it adds unbounded cardinality or a new correlation policy and expands scope beyond projection.

### 4. Bounded structured evidence and omission accounting

Use a fixed finding vocabulary covering the analyzer's existing annotations (retransmission, fast retransmission, out-of-order, missing preceding segment, zero window, duplicate ACK, ACK of unseen data, keepalive and keepalive ACK), partial history, decode errors, plus an other-analysis bucket. Deduplicate a category within each packet when both annotation and boolean represent it. Keep category counts across the entire capture and first 3 examples per category, so routine traffic cannot consume anomaly evidence slots.

Examples contain frame number, relative time, optional stream ID, and selected TCP sequence/ack/window/flags or a bounded error message. They do not depend on retained conversations and do not introduce unbounded endpoint entries. No payload previews, raw hex, checksums, option dumps, arbitrary Info/Summary text, or DNS authority/additional sections are emitted.

Fixed limits are recorded in the document: 100 conversations, at most 200 endpoint IPs, 3 examples per finding category, 20 examples per DNS side, 4 questions and 4 answers per DNS example, and 256 Unicode code points per input-derived text field. Truncate on rune boundaries without appending unbounded content. Protocol/path/origin/finding keys come from finite vocabularies, not packet-controlled strings. Limits are constants, not CLI flags in this iteration.

`omissions` includes packets excluded from conversation detail because its cap was reached (not an untracked distinct-conversation count), packets without conversation identity, examples omitted by each finding category and DNS side, DNS list items omitted, and text fields truncated. List/text omission counts refer to retained examples; omitted examples are counted separately. Retained conversation counters continue to update after limits are reached. No record is excluded from global counts. Emit fixed notes explaining selection policy, heuristic findings, duplicate capture observations, and sensitive metadata.

Alternative: a global first-N packet sample. Rejected because it can hide later anomalies. Gzip or terse keys reduce storage or readability without addressing repeated information as effectively.

### 5. Finalize after successful scanning

Accumulate only bounded summary detail and counters, then encode after EOF and context checks. Propagate read, decode-container, cancellation, and write failures; individual packet decode errors remain observations. File exports still publish only on success. Stdout cannot be rolled back if encoding fails. No timestamps derived from wall clock or nondeterministic map iteration enter output; arrays use first-seen order, finding categories use fixed order, and maps use deterministic JSON encoding.

The existing analyzer remains in use. The bounded-accumulator guarantee deliberately excludes its DNS state and the transient decoder record. This limitation must be documented rather than silently claiming bounded process memory.

## Risks / Trade-offs

- Lossy output hides details -> report limits/omissions and direct users to detailed export or original PCAP for follow-up.
- First-seen conversations exclude later traffic -> capture-wide findings and totals still cover all packets, even omitted conversations.
- Existing heuristics and capture duplication can mislead -> label findings as observations, bytes as captured bytes, and conversations as endpoint groups rather than sessions.
- DNS correlation has imperfect matching and unbounded state -> do not change its semantics or promise whole-process memory bounds; track hardening separately.
- Metadata remains sensitive -> retain explicit privacy documentation; this is not anonymization.
- Serialized bytes are not model tokens -> use a size-reduction regression test as a proxy, not an exact price or token guarantee.

## Migration Plan

Ship as an additive flag with unchanged defaults and no data migration. Existing detailed output is the rollback/fallback path. Document that the aggregate JSON is for manual external use, not `analyze --input`. Validate the new exporter and existing suite before release.

## Open Questions

None blocking implementation. Exact token budgets, configurable caps, analyzer DNS retention hardening, and richer domain aggregation are possible follow-ups, not requirements of this change.
