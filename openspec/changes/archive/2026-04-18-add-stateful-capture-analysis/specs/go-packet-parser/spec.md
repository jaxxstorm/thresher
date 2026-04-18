## MODIFIED Requirements

### Requirement: Parser decodes inner network metadata for non-DISCO packets
For packets where `path_id != 254`, the system SHALL decode the inner IPv4 or IPv6 packet and include transport protocol and port metadata when present. The decoded JSON object SHALL also include richer protocol detail, raw packet bytes, packet summaries, and stable flow identity fields required for downstream stateful analysis.

#### Scenario: TCP packet decoded with detailed metadata
- **WHEN** a non-DISCO payload contains an IPv4 or IPv6 packet with TCP
- **THEN** the decoded JSON object includes `inner.ip_version`, `inner.protocol`, `inner.src_ip`, `inner.dst_ip`, `inner.src_port`, and `inner.dst_port`
- **AND** the decoded JSON object includes `frame_length`, `raw_hex`, `summary`, and a stable `flow_id`
- **AND** the decoded JSON object includes structured TCP metadata such as absolute sequence and acknowledgment values, flags, checksums, header sizing, payload hex, and transport-level payload length

#### Scenario: UDP packet decoded with detailed metadata
- **WHEN** a non-DISCO payload contains an IPv4 or IPv6 packet with UDP
- **THEN** the decoded JSON object includes `inner.ip_version`, `inner.protocol`, `inner.src_ip`, `inner.dst_ip`, `inner.src_port`, and `inner.dst_port`
- **AND** the decoded JSON object includes `frame_length`, `raw_hex`, `summary`, and a stable `flow_id`
- **AND** the decoded JSON object includes structured UDP and IP metadata such as checksums, header detail, payload hex, and transport-level payload length

#### Scenario: ICMP packet decoded with detailed metadata
- **WHEN** a non-DISCO payload contains an IPv4 or IPv6 packet with ICMP
- **THEN** the decoded JSON object includes `inner.ip_version`, `inner.protocol`, `inner.src_ip`, and `inner.dst_ip`
- **AND** the decoded JSON object includes ICMP type/code metadata, checksums, payload hex, and a stable `flow_id`

### Requirement: Parser handles malformed packet data safely
The parser SHALL fail safely when wrapper lengths, DISCO metadata lengths, or inner packet bytes are malformed or truncated, and SHALL preserve frame-level context in any structured decode failure emitted to the output stream.

#### Scenario: Truncated wrapper rejected with frame context
- **WHEN** packet data ends before the declared SNAT, DNAT, or payload offsets are available
- **THEN** decoding returns a contextual error instead of panicking
- **AND** any structured error record retains the frame number, timestamp, and available frame-level metadata already decoded

#### Scenario: Unsupported inner payload reported with raw bytes
- **WHEN** a non-DISCO payload cannot be decoded as a supported inner IP packet
- **THEN** decoding returns a contextual error or structured decode failure without panicking
- **AND** the output retains the original frame bytes or payload bytes needed for later offline analysis
