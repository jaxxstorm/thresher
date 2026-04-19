# analysis-session-ui Specification

## Purpose
TBD - created by syncing change add-aperture-analyze-command. Update Purpose after archive.
## Requirements
### Requirement: Analysis sessions provide a live interactive UI
The system SHALL present a Bubble Tea-based full-window interactive UI for analysis sessions that run in `console` mode, use the available terminal area, refresh incrementally as session state changes, and separate high-value session status from long-form analysis output.

#### Scenario: Session uses full-window layout
- **WHEN** the user starts `thresher analyze` in `console` mode from an interactive terminal
- **THEN** the analysis session opens as a full-screen terminal UI rather than a small inline text block
- **AND** the visible layout includes dedicated regions for current session status and live analysis output

#### Scenario: Session shows live analysis updates
- **WHEN** the analysis session is running in `console` mode and Aperture responses are received
- **THEN** the UI updates incrementally with new analysis output rather than waiting for the entire session to finish
- **AND** previously rendered session status remains visible while new analysis text is appended

#### Scenario: Session shows batch and upload progress
- **WHEN** packets are being collected and uploaded for analysis during a `console` mode session
- **THEN** the UI shows enough state to understand how many packets or batches have been processed and whether a request is in flight

#### Scenario: Session adapts to terminal resize
- **WHEN** the terminal window is resized during a `console` mode analysis session
- **THEN** the UI recomputes its layout to continue using the available window space
- **AND** analysis output and critical session status remain readable after the resize

### Requirement: Session supports interactive model changes when possible
The system SHALL allow the active model to be changed during a `console` mode session when the endpoint and session state allow it, and SHALL present available model information in a way that is easy to find inside the session UI.

#### Scenario: User changes model during session
- **WHEN** the user selects a different available model during an interactive `console` mode session
- **THEN** subsequent analysis requests use the new model
- **AND** the session UI records or displays that the active model changed

#### Scenario: Available models are easy to inspect
- **WHEN** Aperture exposes available models for the session endpoint during a `console` mode session
- **THEN** the full-screen UI presents the model list or current selection in a dedicated, easy-to-scan section
- **AND** the active model is visually distinguished from inactive choices

### Requirement: Session remains usable when analysis is paused or limited
The console UI SHALL communicate when uploads are paused, rate-limited, capped by configured limits, waiting on endpoint responses, or otherwise constrained, and SHALL use visual hierarchy that makes the current session state easy to interpret quickly.

#### Scenario: Data limit reached is visible
- **WHEN** the configured upload limit is reached during a `console` mode session
- **THEN** the UI clearly indicates that further analysis has been paused or stopped because the limit was hit

#### Scenario: Pause and wait states are visible
- **WHEN** the session is paused or waiting on an endpoint response during a `console` mode session
- **THEN** the current state remains visible in the full-screen UI without requiring the user to infer it from scrolling output alone
- **AND** the session continues to show the most relevant counters and recent analysis context
