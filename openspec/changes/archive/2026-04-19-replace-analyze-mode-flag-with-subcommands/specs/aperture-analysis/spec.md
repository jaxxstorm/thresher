## MODIFIED Requirements

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

#### Scenario: Analyze command starts tailnet web mode explicitly
- **WHEN** the user runs `thresher analyze web` with the explicit tailnet web-access setting
- **THEN** the command starts the same analysis workflow using the web session surface
- **AND** the session is served through the tailnet-access runtime instead of a localhost-only listener

#### Scenario: Analyze command sends versioned user agent
- **WHEN** `thresher analyze` sends an HTTP request to an Aperture analysis endpoint
- **THEN** the request includes a `User-Agent` header in the form `thresher/<version>`
- **AND** the header value reflects the running CLI version rather than Go's default `go-http-client` identifier
