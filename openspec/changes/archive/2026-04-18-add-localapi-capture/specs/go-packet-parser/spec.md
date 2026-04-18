## ADDED Requirements

### Requirement: Parser decodes Tailscale wrapper fields
The system SHALL decode the custom Tailscale debug capture wrapper from packet bytes, including the 2-byte little-endian `path_id`, SNAT address length and value, and DNAT address length and value, before decoding the payload.

#### Scenario: Non-DISCO packet wrapper decoded
- **WHEN** a packet begins with a valid wrapper where `path_id` is not `254`
- **THEN** the decoded JSON object includes `path`, `path_id`, `snat`, `dnat`, and `disco: false`

#### Scenario: DISCO packet wrapper decoded
- **WHEN** a packet begins with a valid wrapper where `path_id` is `254`
- **THEN** the decoded JSON object includes `path`, `path_id`, `snat`, `dnat`, and `disco: true`

### Requirement: Parser decodes DISCO metadata and frame types
For packets with `path_id == 254`, the system SHALL parse DISCO metadata fields and identify the DISCO frame type, version, and any frame-specific fields present in the payload.

#### Scenario: Ping frame decoded
- **WHEN** the DISCO frame type is `1`
- **THEN** the decoded JSON object includes a `disco_meta.frame.type` value of `Ping` plus the frame version, transaction ID, and node key

#### Scenario: Pong frame decoded
- **WHEN** the DISCO frame type is `2`
- **THEN** the decoded JSON object includes a `disco_meta.frame.type` value of `Pong` plus the frame version, transaction ID, pong source address, and pong source port

### Requirement: Parser decodes inner network metadata for non-DISCO packets
For packets where `path_id != 254`, the system SHALL decode the inner IPv4 or IPv6 packet and include transport protocol and port metadata when present.

#### Scenario: TCP packet decoded
- **WHEN** a non-DISCO payload contains an IPv4 or IPv6 packet with TCP
- **THEN** the decoded JSON object includes `inner.ip_version`, `inner.protocol`, `inner.src_ip`, `inner.dst_ip`, `inner.src_port`, and `inner.dst_port`

#### Scenario: UDP packet decoded
- **WHEN** a non-DISCO payload contains an IPv4 or IPv6 packet with UDP
- **THEN** the decoded JSON object includes `inner.ip_version`, `inner.protocol`, `inner.src_ip`, `inner.dst_ip`, `inner.src_port`, and `inner.dst_port`

### Requirement: Parser handles malformed packet data safely
The parser SHALL fail safely when wrapper lengths, DISCO metadata lengths, or inner packet bytes are malformed or truncated.

#### Scenario: Truncated wrapper rejected
- **WHEN** packet data ends before the declared SNAT, DNAT, or payload offsets are available
- **THEN** decoding returns a contextual error instead of panicking

#### Scenario: Unsupported inner payload reported
- **WHEN** a non-DISCO payload cannot be decoded as a supported inner IP packet
- **THEN** decoding returns a contextual error or structured decode failure without panicking
