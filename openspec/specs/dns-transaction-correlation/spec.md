# dns-transaction-correlation Specification

## Purpose
TBD - created by archiving change add-stateful-capture-analysis. Update Purpose after archive.
## Requirements
### Requirement: DNS packets are correlated into transactions
The system SHALL correlate DNS requests and responses across frames and emit a stable transaction identifier for packets that belong to the same DNS exchange. DNS packets SHALL also populate normalized packet-row fields so query and response traffic is readable in Wireshark-style views.

#### Scenario: Request and response share one transaction identifier
- **WHEN** a DNS response matches a previously observed DNS request in the same conversation
- **THEN** the request and response decoded JSON objects include the same DNS transaction identifier
- **AND** both decoded packets include normalized top-level packet-row fields including `protocol` and `info`

#### Scenario: Unmatched DNS response remains visible
- **WHEN** a DNS response is observed without a matching prior request in retained analyzer state
- **THEN** the decoded JSON object still includes DNS metadata
- **AND** the decoded JSON object includes an annotation or status indicating that the transaction is incomplete or unmatched

### Requirement: DNS correlation links peer frames
The system SHALL expose frame references between correlated DNS requests and responses when both sides of the exchange have been observed.

#### Scenario: Response references request frame
- **WHEN** a DNS response is matched to an earlier request
- **THEN** the decoded JSON object for the response includes the frame number of the request packet

#### Scenario: Request is enriched after response arrives
- **WHEN** a DNS request has been retained in analyzer state and a matching response is later observed
- **THEN** the analyzer can emit correlation metadata for the response using the original request frame number and transaction identity

### Requirement: DNS summaries reflect transaction context
The system SHALL emit DNS summaries that include enough request and answer detail to identify the logical exchange without re-dissecting raw bytes.

#### Scenario: Query response summary includes answer detail
- **WHEN** a DNS response contains one or more answers
- **THEN** the decoded JSON object includes a summary string containing the response type, DNS identifier, primary question, and answer data
- **AND** the summary-mode output includes a Wireshark-like row such as `Standard query response A example.com A 1.2.3.4`

#### Scenario: Query summary includes question detail
- **WHEN** a DNS request contains one or more questions
- **THEN** the decoded JSON object includes a summary string containing the request type, DNS identifier, and primary question detail
- **AND** the summary-mode output includes a Wireshark-like row such as `Standard query A example.com`

