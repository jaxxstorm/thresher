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

## Comparing To Wireshark

When comparing output from `thresher` to Wireshark or tshark:

- compare `number`, `time`, `src`, `dst`, `protocol`, `length`, and `info` to the packet list columns
- compare `packet_origin` to whether traffic was captured directly versus synthesized from the wrapper path
- use `stream_id` and `stream_packet_number` like Wireshark stream-following context
- use `analysis.annotations` plus the explicit analyzer boolean fields for machine-readable equivalents of expert-info style TCP flags
- use `path`, `path_id`, `snat`, `dnat`, and `disco_meta` for Tailscale-specific metadata that Wireshark packet rows typically do not surface directly
