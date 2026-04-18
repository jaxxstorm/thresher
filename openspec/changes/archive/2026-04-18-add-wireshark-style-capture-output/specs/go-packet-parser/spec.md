## MODIFIED Requirements

### Requirement: Parser decodes DISCO metadata and frame types
For packets with `path_id == 254`, the system SHALL parse DISCO metadata fields and identify the DISCO frame type, version, and any frame-specific fields present in the payload. The decoded packet SHALL also populate normalized top-level packet-row fields so DISCO traffic is readable in the same way as inner IP traffic while preserving Tailscale-specific structure.

#### Scenario: Ping frame decoded with packet-row fields
- **WHEN** the DISCO frame type is `1`
- **THEN** the decoded JSON object includes a `disco_meta.frame.type` value of `Ping` plus the frame version, transaction ID, and node key
- **AND** the decoded JSON object includes normalized top-level fields for `src`, `dst` when available, `protocol`, `length`, and `info`
- **AND** the normalized `protocol` identifies Tailscale control traffic such as `TSMP/DISCO`

#### Scenario: Pong frame decoded with packet-row fields
- **WHEN** the DISCO frame type is `2`
- **THEN** the decoded JSON object includes a `disco_meta.frame.type` value of `Pong` plus the frame version, transaction ID, pong source address, and pong source port
- **AND** the decoded JSON object includes normalized top-level fields for `src`, `dst` when available, `protocol`, `length`, and `info`

### Requirement: Parser decodes inner network metadata for non-DISCO packets
For packets where `path_id != 254`, the system SHALL decode the inner IPv4 or IPv6 packet and include transport protocol and port metadata when present. The decoded JSON object SHALL also include richer protocol detail, raw packet bytes, packet summaries, stable stream identity fields, normalized top-level packet-row columns, and payload usability fields required for downstream stateful analysis and Wireshark-style presentation.

#### Scenario: TCP packet decoded with detailed metadata
- **WHEN** a non-DISCO payload contains an IPv4 or IPv6 packet with TCP
- **THEN** the decoded JSON object includes `inner.ip_version`, `inner.protocol`, `inner.src_ip`, `inner.dst_ip`, `inner.src_port`, and `inner.dst_port`
- **AND** the decoded JSON object includes normalized top-level packet columns such as `number`, `time`, `src`, `dst`, `protocol`, `length`, and `info`
- **AND** the decoded JSON object includes `frame_length`, `raw_hex`, `summary`, and a stable `stream_id`
- **AND** the decoded JSON object includes structured TCP metadata such as absolute and relative sequence and acknowledgment values, flags, checksums, header sizing, payload hex, transport-level payload length, and optional safe ASCII preview data when printable

#### Scenario: UDP packet decoded with detailed metadata
- **WHEN** a non-DISCO payload contains an IPv4 or IPv6 packet with UDP
- **THEN** the decoded JSON object includes `inner.ip_version`, `inner.protocol`, `inner.src_ip`, `inner.dst_ip`, `inner.src_port`, and `inner.dst_port`
- **AND** the decoded JSON object includes normalized top-level packet columns such as `number`, `time`, `src`, `dst`, `protocol`, `length`, and `info`
- **AND** the decoded JSON object includes `frame_length`, `raw_hex`, `summary`, and a stable `stream_id`
- **AND** the decoded JSON object includes structured UDP and IP metadata such as checksums, header detail, payload hex, transport-level payload length, and optional safe ASCII preview data when printable

#### Scenario: ICMP packet decoded with detailed metadata
- **WHEN** a non-DISCO payload contains an IPv4 or IPv6 packet with ICMP
- **THEN** the decoded JSON object includes `inner.ip_version`, `inner.protocol`, `inner.src_ip`, and `inner.dst_ip`
- **AND** the decoded JSON object includes normalized top-level packet columns such as `number`, `time`, `src`, `dst`, `protocol`, `length`, and `info`
- **AND** the decoded JSON object includes ICMP type/code metadata, checksums, payload hex, and a stable `stream_id` when a stream can be assigned

### Requirement: Parser handles malformed packet data safely
The parser SHALL fail safely when wrapper lengths, DISCO metadata lengths, or inner packet bytes are malformed or truncated, and SHALL preserve frame-level context in any structured decode failure emitted to the output stream.

#### Scenario: Truncated wrapper rejected with frame context
- **WHEN** packet data ends before the declared SNAT, DNAT, or payload offsets are available
- **THEN** decoding returns a contextual error instead of panicking
- **AND** any structured error record retains the frame number, timestamp, available frame-level metadata already decoded, and normalized packet-row fields that could be safely derived

#### Scenario: Unsupported inner payload reported with raw bytes
- **WHEN** a non-DISCO payload cannot be decoded as a supported inner IP packet
- **THEN** decoding returns a contextual error or structured decode failure without panicking
- **AND** the output retains the original frame bytes or payload bytes needed for later offline analysis
