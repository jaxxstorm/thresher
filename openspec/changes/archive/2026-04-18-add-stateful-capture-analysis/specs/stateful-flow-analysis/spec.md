## ADDED Requirements

### Requirement: Packets are grouped into stable flows
The system SHALL assign a stable flow identifier to decoded non-DISCO packets so related frames can be grouped into one conversation across both traffic directions.

#### Scenario: Bidirectional TCP traffic shares one flow identifier
- **WHEN** two TCP packets belong to the same conversation but travel in opposite directions
- **THEN** both decoded JSON objects include the same `flow_id`
- **AND** each decoded JSON object retains its observed source and destination addresses and ports

#### Scenario: Unrelated conversations are separated
- **WHEN** packets belong to different transport conversations
- **THEN** the decoded JSON objects use different `flow_id` values

### Requirement: TCP analysis emits relative transport values
The system SHALL preserve absolute TCP sequence and acknowledgment numbers and SHALL also emit relative values derived from the first observed packet in each direction of a flow.

#### Scenario: Relative TCP sequence values are emitted
- **WHEN** a TCP flow contains multiple packets in the same direction
- **THEN** each decoded JSON object includes the absolute TCP sequence number
- **AND** each decoded JSON object includes a relative sequence number for that flow direction

#### Scenario: Relative TCP acknowledgment values are emitted
- **WHEN** a TCP packet acknowledges data from the peer direction
- **THEN** the decoded JSON object includes the absolute acknowledgment number
- **AND** the decoded JSON object includes a relative acknowledgment number derived from the peer direction base sequence

### Requirement: TCP analysis annotates packet relationships
The system SHALL analyze TCP packets against prior packets in the same flow and emit machine-readable annotations when a packet appears to retransmit data, arrive out of order, depend on unseen predecessor data, or acknowledge previously observed data patterns.

#### Scenario: Retransmission is annotated
- **WHEN** a TCP packet repeats previously observed sequence space for the same flow direction
- **THEN** the decoded JSON object includes an analysis annotation identifying retransmission behavior

#### Scenario: Out-of-order packet is annotated
- **WHEN** a TCP packet arrives with sequence space beyond the next expected sequence and prior data for the gap has not been observed
- **THEN** the decoded JSON object includes an analysis annotation identifying out-of-order delivery or missing predecessor data

#### Scenario: Duplicate acknowledgment is annotated
- **WHEN** a TCP packet repeats an acknowledgment without advancing the peer acknowledgment state
- **THEN** the decoded JSON object includes an analysis annotation identifying duplicate ACK behavior

### Requirement: Stateful analysis uses bounded memory
The system SHALL retain only bounded state for active and recently completed conversations so long-running captures do not grow memory without limit.

#### Scenario: Old flow state is evicted
- **WHEN** a flow has aged out of the configured analyzer history window
- **THEN** the analyzer evicts its state before unbounded growth occurs
- **AND** newly observed packets for that conversation start a fresh analysis history
