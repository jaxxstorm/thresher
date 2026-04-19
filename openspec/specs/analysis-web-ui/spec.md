# analysis-web-ui Specification

## Purpose
TBD - created by syncing change add-analysis-web-mode. Update Purpose after archive.

## Requirements
### Requirement: Analysis sessions can be viewed in a localhost web UI
The system SHALL support a web analysis mode that starts an HTTP server for `thresher analyze` and exposes the active session through a browser-accessible web UI. The web server SHALL support a default localhost-only form and an explicit tailnet-served form for remote access.

#### Scenario: Web mode starts a localhost server by default
- **WHEN** the user runs `thresher analyze --mode web` without enabling tailnet access
- **THEN** the command starts an HTTP server bound only to a localhost address
- **AND** the command reports the local URL that can be opened in a browser

#### Scenario: Web mode starts a tailnet server explicitly
- **WHEN** the user runs `thresher analyze --mode web` with the explicit tailnet web-access setting
- **THEN** the command starts the web UI through the remote serving runtime instead of a localhost-only listener
- **AND** the command reports a tailnet URL that can be opened from another device on the same tailnet

#### Scenario: Local web mode remains non-remote by default
- **WHEN** the web analysis server starts without the tailnet web-access setting
- **THEN** it listens only on localhost interfaces
- **AND** it does not expose non-local interfaces in the default configuration

### Requirement: Web UI shows live session state and analysis output
The system SHALL render the current analysis session state in the web UI, including session status, packet and batch counters, active model information, and analysis text as new responses are received.

#### Scenario: Browser shows current session snapshot
- **WHEN** a user opens the localhost web UI during an active analysis session
- **THEN** the page shows the current status, counters, and any analysis text already received

#### Scenario: Browser receives live analysis updates
- **WHEN** new analysis output arrives after the page has loaded
- **THEN** the web UI updates without requiring a full page refresh
- **AND** previously displayed analysis output remains visible alongside the new content

### Requirement: Web UI reflects session completion and failure states
The system SHALL show when the analysis session completes, is canceled, reaches a configured limit, or fails to contact the analysis endpoint.

#### Scenario: Limit reached is visible in the browser
- **WHEN** the configured analysis session limit is reached while web mode is active
- **THEN** the web UI shows that uploads have stopped because the limit was hit

#### Scenario: Request failure is visible in the browser
- **WHEN** an analysis request fails during a web session
- **THEN** the web UI shows the failure state and the most recent error context
