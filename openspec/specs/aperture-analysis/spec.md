# aperture-analysis Specification

## Purpose
TBD - created by syncing change add-aperture-analyze-command. Update Purpose after archive.
## Requirements
### Requirement: Analyze command sends capture analysis to Aperture
The system SHALL expose an `analyze` command, with `analyse` as an alias, that submits decoded capture context to an Aperture-served LLM endpoint and returns ongoing analysis about what is happening in the packet stream. The command SHALL default its endpoint base URL to `http://ai` when the user does not provide one via flags or config, it SHALL support an explicit presentation mode selection so the analysis session can run in either `console` or `web` mode, web mode SHALL support explicit local-only or tailnet-served access without changing the underlying Aperture analysis workflow, and it SHALL identify requests to Aperture with a `User-Agent` header in the form `thresher/<version>` instead of the generic Go default.

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

#### Scenario: Analyze command starts localhost web mode explicitly
- **WHEN** the user runs `thresher analyze --mode web`
- **THEN** the command starts the same analysis workflow using the web session surface instead of the console session surface
- **AND** the web session is exposed only on localhost unless the user explicitly enables tailnet access

#### Scenario: Analyze command starts tailnet web mode explicitly
- **WHEN** the user runs `thresher analyze --mode web` with the explicit tailnet web-access setting
- **THEN** the command starts the same analysis workflow using the web session surface
- **AND** the session is served through the tailnet-access runtime instead of a localhost-only listener

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
