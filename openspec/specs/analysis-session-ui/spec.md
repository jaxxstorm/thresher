# analysis-session-ui Specification

## Purpose
TBD - created by syncing change add-aperture-analyze-command. Update Purpose after archive.
## Requirements
### Requirement: Analysis sessions provide a live interactive UI
The system SHALL present a Bubble Tea-based interactive UI for analysis sessions that shows capture progress, batching or upload state, selected model, and live LLM analysis output.

#### Scenario: Session shows live analysis updates
- **WHEN** the analysis session is running and Aperture responses are received
- **THEN** the UI updates incrementally with new analysis output rather than waiting for the entire session to finish

#### Scenario: Session shows batch and upload progress
- **WHEN** packets are being collected and uploaded for analysis
- **THEN** the UI shows enough state to understand how many packets or batches have been processed and whether a request is in flight

### Requirement: Session supports interactive model changes when possible
The system SHALL allow the active model to be changed during a session when the endpoint and session state allow it.

#### Scenario: User changes model during session
- **WHEN** the user selects a different available model during an interactive session
- **THEN** subsequent analysis requests use the new model
- **AND** the session UI records or displays that the active model changed

### Requirement: Session remains usable when analysis is paused or limited
The UI SHALL communicate when uploads are paused, rate-limited, capped by configured limits, or waiting on endpoint responses.

#### Scenario: Data limit reached is visible
- **WHEN** the configured upload limit is reached during a session
- **THEN** the UI clearly indicates that further analysis has been paused or stopped because the limit was hit
