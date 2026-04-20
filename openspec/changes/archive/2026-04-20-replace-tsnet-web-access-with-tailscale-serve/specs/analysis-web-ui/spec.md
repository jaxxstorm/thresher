## MODIFIED Requirements

### Requirement: Analysis sessions can be viewed in a localhost web UI
The system SHALL support a web analysis mode that starts an HTTP server for `thresher analyze` and exposes the active session through a browser-accessible web UI. The web server SHALL support a default localhost-only form and an explicit tailnet-served form for remote access, and the browser workflow SHALL be started through the explicit `thresher analyze web` subcommand rather than a `--mode` flag.

#### Scenario: Web mode starts a localhost server by default
- **WHEN** the user runs `thresher analyze web` without enabling tailnet access
- **THEN** the command starts an HTTP server bound only to a localhost address
- **AND** the command reports the local URL that can be opened in a browser

#### Scenario: Web mode starts a tailnet server explicitly through Serve
- **WHEN** the user runs `thresher analyze web` with the explicit tailnet web-access setting
- **THEN** the command starts the web UI on a localhost listener and publishes it remotely through the host's existing Tailscale Serve configuration
- **AND** the command reports a tailnet URL that resolves on the host's existing tailnet identity
- **AND** the remote URL points at one dedicated Serve-backed web path for the analysis session

#### Scenario: Local web mode remains non-remote by default
- **WHEN** the web analysis server starts without the tailnet web-access setting
- **THEN** it listens only on localhost interfaces
- **AND** it does not expose non-local interfaces in the default configuration

## ADDED Requirements

### Requirement: Web UI routes work behind a Serve-backed path prefix
The system SHALL allow the analyze web UI to run behind a dedicated path prefix when tailnet access is published through Tailscale Serve.

#### Scenario: Remote page uses prefixed routes
- **WHEN** a user opens the Serve-published analysis web UI from another tailnet device
- **THEN** the page, snapshot feed, live event stream, and control actions resolve beneath the same dedicated path prefix
- **AND** the browser does not rely on root-relative URLs that would escape that published path
