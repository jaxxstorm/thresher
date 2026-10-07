# Capture Output Guide

`thresher capture` now exposes packet records with both detailed nested decode and Wireshark-style packet-row fields.

## Packet Row Columns

These top-level fields correspond most closely to the packet list you would see in Wireshark or tshark:

- `number`: packet number in the capture
- `time`: RFC3339 timestamp for the packet
- `src`: normalized packet source
- `dst`: normalized packet destination
- `protocol`: normalized protocol label such as `TCP`, `UDP`, `DNS`, or `TSMP/DISCO`
- `length`: full wrapped packet length in bytes
- `info`: Wireshark-style packet summary

Additional scan-friendly fields:

- `packet_origin`: whether the packet was `captured` directly or `synthesized`
- `stream_id`: stable conversation identifier
- `conversation_key`: readable grouping key for the stream
- `stream_packet_number`: packet number within the stream direction
- `time_since_stream_start`: seconds since the first packet in the stream
- `time_since_previous_in_stream`: seconds since the previous packet in the same stream direction
- `transport_direction`: `client_to_server` or `server_to_client`
- `relative_seq`: promoted TCP relative sequence value when available
- `relative_ack`: promoted TCP relative acknowledgment value when available
- `payload_length`: decoded inner payload length when available
- `payload_preview`: safe ASCII preview when the payload is printable

Analyzer-grade packet flags are exposed both in `analysis.annotations` and in stable explicit fields when available:

- `analysis.retransmission`
- `analysis.fast_retransmission`
- `analysis.out_of_order`
- `analysis.previous_segment_not_captured`
- `analysis.zero_window`

## Tailscale-Specific Fields

These fields remain separate from the Wireshark-style columns because they describe the wrapper around the inner packet rather than the inner packet itself:

- `path`
- `path_id`
- `snat`
- `dnat`
- `disco`

For DISCO traffic, `protocol` is normalized to `TSMP/DISCO` and the full control-plane details remain under `disco_meta`.
When a DISCO frame exposes a derivable endpoint such as a `Pong` source, `dst` is populated from that endpoint.

## Why DISCO Is Separate

DISCO packets are not inner TCP, UDP, or DNS traffic. They are Tailscale control traffic carried in the debug capture wrapper. That is why:

- `protocol` shows a control-plane label such as `TSMP/DISCO`
- `info` shows control-plane context such as `Ping from 192.168.1.32:41641`
- detailed subtype fields stay under `disco_meta`

## Output Modes

### Detailed JSONL

```bash
go run . capture --format jsonl
```

Use this when you want the full nested decode plus the normalized row fields.

### Compact JSONL

```bash
go run . capture --format jsonl-compact
```

Use this when you want the normalized row columns promoted to the top while still retaining nested `inner`, `analysis`, and `disco_meta` data for jq or grep.

### Summary Rows

```bash
go run . capture --format summary
```

Use this when you want tshark-like one-line rows for quick human scanning.

### Packet List Rows

```bash
go run . capture --format packet-list
```

Use this when you want a dedicated packet-list-style rendering with one tab-separated row per packet, similar to a stripped-down tshark packet list.

## Aggregate JSON Summary

```bash
go run . convert capture.pcap --mode summary -o summary.json
go run . convert capture.pcap --mode summary -o -
go run . convert capture.pcap --mode detailed -o detailed.json
```

Unlike `capture --format summary`, which prints one human-readable row per
packet, `convert --mode summary` writes one aggregate JSON document for manual
LLM upload. It runs offline and uploads nothing. Detailed mode remains the
default. Both modes support classic Tailscale USER0 PCAPs, including gzip input,
not general Ethernet PCAPs or PCAPNG. These documents are not accepted by
`analyze --input`, which expects JSONL. Summary mode rejects `--include-raw=true`.

### Document Fields

| Field | Meaning |
| --- | --- |
| `schema_version`, `mode` | `1` and `"summary"`; use mode to distinguish this schema from detailed export |
| `capture` | Packet/captured-byte counts, start/end times, duration, protocol/path/origin counts, DISCO and decode-error counts |
| `endpoints` | Reusable IP entries with IDs such as `e1`; only retained conversations add endpoints |
| `conversations` | Canonical endpoints A/B with ports, protocol/IP version, stream ID, frame/time bounds, directional counts, paths/origins, and SYN/FIN/RST packet counts |
| `dns` | Query/response packet counts, analyzer match observations, response codes, latency statistics, and query/response examples |
| `findings` | Heuristic categories, packet counts, and selected evidence with frame/time references |
| `limits` | Fixed retention limits applied to the document |
| `omissions` | Detail excluded by limits and records without conversation identity |
| `notes` | Selection policy, interpretation caveats, and privacy warnings |

There is no full `packets` array. Raw hex, payload previews, unrestricted packet
summary text, checksums, TCP options, and DNS authority/additional sections are
excluded. Selected DNS questions and answers, TCP sequence/ack/window/flags, and
decode-error messages can remain. Empty collections are arrays; an empty capture
has zero totals and no start/end timestamps.

### Counts and Timing

- Capture-wide counters cover all records, including malformed packets and conversations whose details were not retained.
- `captured_bytes` sums wrapped frame lengths, including Tailscale wrapper bytes and duplicate capture observations. It is not unique application traffic volume.
- Conversations group canonical endpoint tuples, including IP version and protocol. Reuse of a tuple is merged; counts do not represent distinct connection lifetimes.
- `a_to_b` and `b_to_a` refer to canonical endpoint ordering, not inferred client/server roles. SYN/FIN/RST counters are observed packet flags, not proof of established or closed sessions.
- Capture start/end use minimum/maximum timestamps. Relative `time_ms`, `start_ms`, and `end_ms` are milliseconds from capture start with sub-millisecond precision; selection follows frame order even if timestamps regress.
- Protocol counters use the inner transport category; DNS observations have their own counters. Unknown or unsupported categories use finite unknown/other buckets.
- DNS counts describe packets, not unique transactions. Matched response examples reference the analyzer's query frame; repeated responses can reference the same query. The summary does not infer unanswered queries or timeouts.
- DNS latency aggregates use only nonnegative matched-response observations. Negative latency observations are counted separately; no latency minimum/maximum/mean is emitted when there are no valid observations.
- TCP findings are existing analyzer heuristics, not proof of packet loss or root cause. Incomplete capture history can affect them.

### Limits and Omissions

| Retained detail | Fixed limit |
| --- | ---: |
| First distinct conversations | 100 |
| Endpoint IP entries | 200 |
| First examples per finding category | 3 |
| First DNS query examples | 20 |
| First DNS response examples | 20 |
| Questions and answers per DNS example | 4 each |
| Input-derived text field length | 256 Unicode code points |

Conversation selection is first-seen, not highest-volume. Retained conversations
continue accumulating counts after the cap is reached. Findings and DNS evidence
are collected independently, so later anomalies can remain visible even when
their conversation detail is omitted. Output ordering and IDs are deterministic.

`omissions.conversation_detail_packets` counts packets excluded by conversation
capacity, not excluded distinct conversations. `packets_without_conversation_identity`
accounts separately for records without usable identity, such as DISCO or malformed
records. Other counters report omitted examples by finding category and DNS side,
questions/answers beyond retained-example limits, and truncated text fields.
List/text omission counts apply to retained examples; entirely omitted examples
are counted separately. Global packet, byte, DNS, and finding counts are unaffected.

The new accumulator retains bounded detail, but the existing analyzer's DNS
correlation state and transient decoded packets are outside that bound. Summary
mode does not guarantee bounded whole-process memory.

### Size and Safety

The regression fixture in `internal/capture/summary_export_test.go` contains 1,000
routine packets in one conversation. Detailed JSON without raw bytes measured
1,172,503 bytes; summary JSON measured 2,708 bytes, a 99.77% serialized-byte reduction.
Both report 1,000 packets and 49,000 captured bytes. The test requires summary
size to remain at most 20% of detailed size. Savings vary by capture, and byte
size is a proxy, not an exact model-token count or price estimate. Small captures
can have more summary overhead than packet detail.

IP addresses, DNS names/answers, and other metadata remain sensitive. Review the
file before sharing it; summary mode is lossy compression, not anonymization.
Retain the original PCAP and use detailed export to investigate omitted frames.

Summary output is encoded only after a successful scan. File publication remains
atomic: read errors or cancellation preserve an existing destination and clean up
temporary output. A stdout write failure can leave partial JSON. Input/output
aliases are rejected to protect the original capture.

## Comparing To Wireshark

When comparing output from `thresher` to Wireshark or tshark:

- compare `number`, `time`, `src`, `dst`, `protocol`, `length`, and `info` to the packet list columns
- compare `packet_origin` to whether traffic was captured directly versus synthesized from the wrapper path
- use `stream_id` and `stream_packet_number` like Wireshark stream-following context
- use `analysis.annotations` plus the explicit analyzer boolean fields for machine-readable equivalents of expert-info style TCP flags
- use `path`, `path_id`, `snat`, `dnat`, and `disco_meta` for Tailscale-specific metadata that Wireshark packet rows typically do not surface directly
