## MODIFIED Requirements

### Requirement: Analyze command sends capture analysis to Aperture
The system SHALL expose an `analyze` command, with `analyse` as an alias, that submits decoded capture context to an Aperture-served LLM endpoint and returns ongoing analysis about what is happening in the packet stream. The command SHALL default its endpoint base URL to `http://ai` when the user does not provide one via flags or config, and it SHALL support an explicit presentation mode selection so the analysis session can run in either `console` or `web` mode.

#### Scenario: Analyze command uses built-in default endpoint
- **WHEN** the user runs `thresher analyze` without a configured or explicit endpoint
- **THEN** the command uses `http://ai` as the Aperture endpoint base URL
- **AND** the session starts without returning an endpoint-required error

#### Scenario: Analyze command uses Aperture-compatible endpoint paths
- **WHEN** the user runs `thresher analyze --endpoint http://ai`
- **THEN** the command talks only to Aperture-compatible analysis endpoints beneath that base URL such as `/v1/messages`, `/v1/chat/completions`, or `/v1/responses`
- **AND** the command does not require direct local API-key or provider-auth configuration

#### Scenario: Analyze command defaults to console mode
- **WHEN** the user runs `thresher analyze` without an explicit mode flag
- **THEN** the command starts the analysis session in `console` mode

#### Scenario: Analyze command starts web mode explicitly
- **WHEN** the user runs `thresher analyze --mode web`
- **THEN** the command starts the same analysis workflow using the web session surface instead of the console session surface

### Requirement: Analysis sessions bound cost with batching and limits
The system SHALL batch decoded packet context before submission and SHALL expose controls that limit how much capture data is sent during an analysis session, regardless of whether the session is running in console mode or web mode.

#### Scenario: Batch size is bounded
- **WHEN** a live or file-backed analysis session accumulates decoded packet rows
- **THEN** the system groups packets into bounded batches before upload rather than sending every packet immediately

#### Scenario: Session stops when configured data limit is reached
- **WHEN** the configured packet, byte, or equivalent analysis limit is reached during a session
- **THEN** the system stops or pauses further uploads and clearly reports that the analysis limit was reached
