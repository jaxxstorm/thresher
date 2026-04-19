## Why

The current `thresher analyze` session works, but the UI is a sparse text dump that does not use the terminal well and makes live analysis harder to scan during long-running sessions. This change is needed now to turn analysis into a full-window, continuously refreshing experience with a sensible default endpoint so users can start quickly and understand session state at a glance.

## What Changes

- Replace the current minimal analyze view with a full-screen Bubble Tea interface that uses the full terminal window, refreshes live state continuously, and presents analysis output, counters, status, and model information in clearly separated panes
- Improve visual hierarchy with intentional color, borders, and status treatments so users can distinguish active uploads, limits, model state, and new analysis output without reading a wall of text
- Add richer session summaries in the analyze UI, including easier-to-scan packet, batch, and session limit information while preserving incremental analysis updates
- Change the analyze command defaults so the endpoint falls back to `http://ai` when neither flags nor config provide an endpoint
- Update command help, docs, and tests to reflect the new default endpoint and the revised full-screen interaction model
- Keep the existing capture decode pipeline and packet format unchanged; this change only affects the analysis UX and default configuration behavior

## Non-goals

- No changes to the Tailscale packet wrapper, including the 2-byte little-endian `path_id`, SNAT or DNAT length fields, or DISCO frame structure
- No change to the upstream Aperture-compatible endpoint family beyond defaulting the base URL to `http://ai`
- No introduction of direct provider credentials, local key management, or arbitrary non-Aperture analysis backends
- No replacement of the existing batching and upload-limit controls with a fundamentally different analysis pipeline

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `analysis-session-ui`: require a full-window, dynamically refreshing analyze UI with stronger visual hierarchy and clearer presentation of session state and analysis output
- `analysis-config`: require the analyze workflow to default the endpoint to `http://ai` when neither config nor command-line flags provide one
- `aperture-analysis`: remove the requirement for an explicit endpoint and instead require analysis sessions to use the built-in default base URL unless overridden

## Impact

- **UI**: `internal/analyze/ui.go` and session state handling need a more structured full-screen layout and richer rendering model
- **CLI and config**: `cmd/analyze.go`, config defaults, and related tests or docs need to treat `http://ai` as the built-in default endpoint
- **Documentation**: analyze command guidance and verification steps need to describe the new fullscreen UX and default endpoint behavior
- **Testing**: existing analyze command and session tests need updates for the changed endpoint requirement and the richer UI behavior
