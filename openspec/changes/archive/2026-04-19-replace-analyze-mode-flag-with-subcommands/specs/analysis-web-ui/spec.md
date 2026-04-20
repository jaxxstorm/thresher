## MODIFIED Requirements

### Requirement: Analysis sessions can be viewed in a localhost web UI
The system SHALL support a web analysis mode that starts an HTTP server for `thresher analyze` and exposes the active session through a browser-accessible web UI. The web server SHALL support a default localhost-only form and an explicit tailnet-served form for remote access, and the browser workflow SHALL be started through the explicit `thresher analyze web` subcommand rather than a `--mode` flag.

#### Scenario: Web mode starts a localhost server by default
- **WHEN** the user runs `thresher analyze web` without enabling tailnet access
- **THEN** the command starts an HTTP server bound only to a localhost address
- **AND** the command reports the local URL that can be opened in a browser

#### Scenario: Web mode starts a tailnet server explicitly
- **WHEN** the user runs `thresher analyze web` with the explicit tailnet web-access setting
- **THEN** the command starts the web UI through the remote serving runtime instead of a localhost-only listener
- **AND** the command reports a tailnet URL that can be opened from another device on the same tailnet

#### Scenario: Local web mode remains non-remote by default
- **WHEN** the web analysis server starts without the tailnet web-access setting
- **THEN** it listens only on localhost interfaces
- **AND** it does not expose non-local interfaces in the default configuration
