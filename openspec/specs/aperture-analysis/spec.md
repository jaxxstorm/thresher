# aperture-analysis Specification

## Purpose
TBD - created by syncing change add-aperture-analyze-command. Update Purpose after archive.
## Requirements
### Requirement: Analyze command sends capture analysis to Aperture
The system SHALL expose an `analyze` command, with `analyse` as an alias, that submits decoded capture context to an Aperture-served LLM endpoint and returns ongoing analysis about what is happening in the packet stream. The command SHALL default its endpoint base URL to `http://ai` when the user does not provide one via flags or config, it SHALL expose explicit `console` and `web` subcommands for presentation-specific workflows, bare `thresher analyze` SHALL invoke the `console` workflow by default, web mode SHALL support explicit local-only or tailnet-served access without changing the underlying Aperture analysis workflow, and it SHALL identify requests to Aperture with a `User-Agent` header in the form `thresher/<version>` instead of the generic Go default.

#### Scenario: Analyze command uses built-in default endpoint
- **WHEN** the user runs `thresher analyze` without a configured or explicit endpoint
- **THEN** the command uses `http://ai` as the Aperture endpoint base URL
- **AND** the session starts without returning an endpoint-required error

#### Scenario: Analyze command uses Aperture-compatible endpoint paths
- **WHEN** the user runs `thresher analyze --endpoint http://ai`
- **THEN** the command talks only to Aperture-compatible analysis endpoints beneath that base URL such as `/v1/messages`, `/v1/chat/completions`, or `/v1/responses`
- **AND** the command does not require direct local API-key or provider-auth configuration

#### Scenario: Analyze command defaults to console subcommand behavior
- **WHEN** the user runs `thresher analyze` without an explicit subcommand
- **THEN** the command starts the analysis session in the `console` workflow

#### Scenario: Analyze command starts console mode explicitly
- **WHEN** the user runs `thresher analyze console`
- **THEN** the command starts the console analysis workflow

#### Scenario: Analyze command starts local alias explicitly
- **WHEN** the user runs `thresher analyze local`
- **THEN** the command starts the same console analysis workflow as `thresher analyze console`

#### Scenario: Analyze command starts localhost web mode explicitly
- **WHEN** the user runs `thresher analyze web`
- **THEN** the command starts the same analysis workflow using the web session surface instead of the console session surface
- **AND** the web session is exposed only on localhost unless the user explicitly enables tailnet access

#### Scenario: Analyze command starts tailnet web mode through the existing Tailscale device
- **WHEN** the user runs `thresher analyze web` with the explicit tailnet web-access setting
- **THEN** the command starts the same analysis workflow using the web session surface
- **AND** the session is published through the host's existing local `tailscaled` Serve configuration instead of a separate Thresher-managed tailnet node
- **AND** the reported remote URL uses the host's existing tailnet identity rather than a new node identity

#### Scenario: Analyze command sends versioned user agent
- **WHEN** `thresher analyze` sends an HTTP request to an Aperture analysis endpoint
- **THEN** the request includes a `User-Agent` header in the form `thresher/<version>`
- **AND** the header value reflects the running CLI version rather than Go's default `go-http-client` identifier

### Requirement: Analysis sessions bound cost with batching and limits
The system SHALL batch decoded packet context before submission and SHALL expose controls that limit how much capture data is sent during an analysis session, regardless of whether the session is running in console mode or web mode.

#### Scenario: Batch size is bounded
- **WHEN** a live or file-backed analysis session accumulates decoded packet rows
- **THEN** the system groups packets into bounded batches before upload rather than sending every packet immediately

#### Scenario: Session stops when configured data limit is reached
- **WHEN** the configured packet, byte, or equivalent analysis limit is reached during a session
- **THEN** the system stops or pauses further uploads and clearly reports that the analysis limit was reached

### Requirement: User can choose a model for analysis
The system SHALL allow the user to choose which upstream model Aperture should use for analysis.

#### Scenario: Model is selected by flag
- **WHEN** the user runs `thresher analyze --model gpt-4o`
- **THEN** the request sent to Aperture includes the selected model identifier

#### Scenario: Model is selected from config default
- **WHEN** the user has configured a default analysis model and does not pass `--model`
- **THEN** the analysis session uses the configured model by default

### Requirement: Model discovery is supported when Aperture exposes it
The system SHALL query Aperture for available models when a supported discovery endpoint is available and SHALL degrade gracefully when discovery is not available.

#### Scenario: Available models can be listed
- **WHEN** Aperture exposes a supported model discovery endpoint
- **THEN** the analysis workflow can list or present available models to the user

#### Scenario: Discovery unavailable does not block analysis
- **WHEN** model discovery is not available from the configured endpoint
- **THEN** the user can still start analysis by explicitly providing a model identifier

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
