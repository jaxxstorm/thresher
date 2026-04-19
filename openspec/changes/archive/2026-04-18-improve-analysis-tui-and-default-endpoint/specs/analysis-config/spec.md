## MODIFIED Requirements

### Requirement: Analysis defaults can be configured
The system SHALL support config-backed defaults for analysis settings such as endpoint, model, batching, and upload or cost limiting values. When neither config nor flags provide an endpoint, the system SHALL default the analyze endpoint to `http://ai`.

#### Scenario: Endpoint comes from config
- **WHEN** the user has configured an Aperture analysis endpoint in config and does not pass `--endpoint`
- **THEN** the analysis command uses the configured endpoint value

#### Scenario: Built-in endpoint default is used
- **WHEN** the user runs `thresher analyze` without `--endpoint` and without configuring `analyze.endpoint`
- **THEN** the analysis command uses `http://ai` as the endpoint base URL

#### Scenario: Flags override config
- **WHEN** a setting such as `--model` or `--endpoint` is provided on the command line
- **THEN** the command-line value overrides any config-file default for that setting

### Requirement: Analysis settings can be constrained by config
The system SHALL allow config defaults to set protective limits for analysis sessions.

#### Scenario: Config defines upload caps
- **WHEN** the user has configured default packet, byte, or equivalent upload limits
- **THEN** those limits apply to analysis sessions unless overridden by flags
