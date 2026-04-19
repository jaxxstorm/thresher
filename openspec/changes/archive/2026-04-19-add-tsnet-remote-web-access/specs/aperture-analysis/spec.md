## MODIFIED Requirements

### Requirement: Analyze command sends capture analysis to Aperture
The system SHALL expose an `analyze` command, with `analyse` as an alias, that submits decoded capture context to an Aperture-served LLM endpoint and returns ongoing analysis about what is happening in the packet stream. The command SHALL default its endpoint base URL to `http://ai` when the user does not provide one via flags or config, it SHALL support an explicit presentation mode selection so the analysis session can run in either `console` or `web` mode, and web mode SHALL support explicit local-only or tailnet-served access without changing the underlying Aperture analysis workflow.

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
