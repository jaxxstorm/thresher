## MODIFIED Requirements

### Requirement: Packets are grouped into stable flows
The system SHALL assign a stable stream identifier to decoded non-DISCO packets so related frames can be grouped into one conversation across both traffic directions. The decoded packet SHALL also expose readable conversation fields that make stream grouping easy in human-readable and JSON-oriented workflows.

#### Scenario: Bidirectional TCP traffic shares one stream identifier
- **WHEN** two TCP packets belong to the same conversation but travel in opposite directions
- **THEN** both decoded JSON objects include the same `stream_id`
- **AND** each decoded JSON object retains its observed source and destination addresses and ports
- **AND** each decoded JSON object can expose a stable `conversation_key` or equivalent readable grouping field

#### Scenario: Unrelated conversations are separated
- **WHEN** packets belong to different transport conversations
- **THEN** the decoded JSON objects use different `stream_id` values

### Requirement: TCP analysis emits relative transport values
The system SHALL preserve absolute TCP sequence and acknowledgment numbers and SHALL also emit relative values derived from the first observed packet in each direction of a stream. The decoded packet SHALL include per-stream packet numbering and relative timing that support Wireshark-style stream inspection.

#### Scenario: Relative TCP sequence values are emitted
- **WHEN** a TCP stream contains multiple packets in the same direction
- **THEN** each decoded JSON object includes the absolute TCP sequence number
- **AND** each decoded JSON object includes a relative sequence number for that stream direction
- **AND** each decoded JSON object includes `stream_packet_number`, `time_since_stream_start`, and `time_since_previous_in_stream`

#### Scenario: Relative TCP acknowledgment values are emitted
- **WHEN** a TCP packet acknowledges data from the peer direction
- **THEN** the decoded JSON object includes the absolute acknowledgment number
- **AND** the decoded JSON object includes a relative acknowledgment number derived from the peer direction base sequence

### Requirement: TCP analysis annotates packet relationships
The system SHALL analyze TCP packets against prior packets in the same stream and emit machine-readable annotations when a packet appears to retransmit data, arrive out of order, depend on unseen predecessor data, acknowledge previously observed data patterns, advertise zero window, or represent keepalive-style traffic.

#### Scenario: Retransmission-style events are annotated
- **WHEN** a TCP packet repeats previously observed sequence space for the same flow direction or matches fast-retransmission heuristics
- **THEN** the decoded JSON object includes an analysis annotation identifying retransmission behavior such as `retransmission` or `fast_retransmission`

#### Scenario: Out-of-order or missing-segment events are annotated
- **WHEN** a TCP packet arrives with sequence space beyond the next expected sequence and prior data for the gap has not been observed
- **THEN** the decoded JSON object includes an analysis annotation identifying out-of-order delivery or missing predecessor data such as `out_of_order` or `previous_segment_not_captured`

#### Scenario: ACK and keepalive events are annotated
- **WHEN** a TCP packet repeats an acknowledgment without advancing the peer acknowledgment state, advertises zero window, or matches keepalive-style framing
- **THEN** the decoded JSON object includes analysis annotations such as `duplicate_ack`, `zero_window`, `keepalive`, or `keepalive_ack`

### Requirement: Stateful analysis uses bounded memory
The system SHALL retain only bounded state for active and recently completed conversations so long-running captures do not grow memory without limit.

#### Scenario: Old stream state is evicted
- **WHEN** a stream has aged out of the configured analyzer history window
- **THEN** the analyzer evicts its state before unbounded growth occurs
- **AND** newly observed packets for that conversation start a fresh analysis history
