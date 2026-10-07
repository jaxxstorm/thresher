## ADDED Requirements

### Requirement: Analysis sessions send a stable Aperture session identifier
The system SHALL assign one logical session identifier to each `thresher analyze` invocation and reuse it for every analysis request emitted by that invocation, regardless of whether the session is presented in console mode or web mode. The identifier MUST be fresh for each new invocation and MUST remain stable while batching, pausing, resuming, or model switching causes multiple uploads during the same analysis session.

#### Scenario: One analyze invocation reuses the same logical session identifier
- **WHEN** a single `thresher analyze` invocation sends two or more analysis uploads to Aperture
- **THEN** each upload carries the same session-tracking identifier for that invocation
- **AND** a later `thresher analyze` invocation uses a different session-tracking identifier

#### Scenario: Messages endpoint uses metadata user identity
- **WHEN** the analyze client sends an analysis request to `/v1/messages`
- **THEN** the JSON request body includes `metadata.user_id`
- **AND** the value matches `session_<uuid>` for the active analysis session

#### Scenario: Responses endpoint uses Session_id header
- **WHEN** the analyze client sends an analysis request to `/v1/responses`
- **THEN** the HTTP request includes a `Session_id` header
- **AND** the header value matches `session_<uuid>` for the active analysis session

#### Scenario: Chat completions endpoint uses a deterministic session fingerprint
- **WHEN** the analyze client sends an analysis request to `/v1/chat/completions`
- **THEN** the HTTP request includes a `Session_id` header
- **AND** the header value is the same 16-character lowercase hexadecimal fingerprint for every upload in that analysis session

#### Scenario: Model switches do not rotate the active session identifier
- **WHEN** the user changes the active model during one running analysis session and a later upload is sent
- **THEN** the later upload uses the same session-tracking identifier as earlier uploads from that session
