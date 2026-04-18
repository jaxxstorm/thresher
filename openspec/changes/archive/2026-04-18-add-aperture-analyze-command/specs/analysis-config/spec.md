## ADDED Requirements

### Requirement: Analysis defaults can be configured
The system SHALL support config-backed defaults for analysis settings such as endpoint, model, batching, and upload or cost limiting values.

#### Scenario: Endpoint comes from config
- **WHEN** the user has configured an Aperture analysis endpoint in config and does not pass `--endpoint`
- **THEN** the analysis command uses the configured endpoint value

#### Scenario: Flags override config
- **WHEN** a setting such as `--model` or `--endpoint` is provided on the command line
- **THEN** the command-line value overrides any config-file default for that setting

### Requirement: Analysis settings can be constrained by config
The system SHALL allow config defaults to set protective limits for analysis sessions.

#### Scenario: Config defines upload caps
- **WHEN** the user has configured default packet, byte, or equivalent upload limits
- **THEN** those limits apply to analysis sessions unless overridden by flags
