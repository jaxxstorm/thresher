## ADDED Requirements

### Requirement: Capture command is registered on the CLI root
The root command SHALL register a `capture` subcommand so that it appears in the CLI help output and can be invoked as `thresher capture`.

#### Scenario: Capture appears in help output
- **WHEN** the user runs `thresher --help`
- **THEN** `capture` appears in the list of available commands
