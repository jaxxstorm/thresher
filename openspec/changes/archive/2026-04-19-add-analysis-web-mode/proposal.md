## Why

`thresher analyze` currently requires an interactive terminal session to get the live analysis experience, which makes it hard to inspect the session from a browser or share a richer UI surface beyond the local console. Adding a localhost web mode now lets analysis sessions expose the same live state through a browser while keeping the existing console workflow available behind an explicit mode selection.

## What Changes

- Add a web presentation mode for `thresher analyze` that starts a localhost-only HTTP server and renders live analysis session state in a browser-friendly UI.
- Add an explicit mode flag on `thresher analyze` so users can choose between the existing console experience and the new web experience.
- Keep analysis batching, model selection, limits, and packet decoding behavior consistent across console and web modes.
- Clarify that this change does not alter the packet capture wire format or decoded packet schema; it only changes how analysis session state is presented locally.

## Non-goals

- Exposing the analysis web UI on non-localhost interfaces.
- Adding authentication, multi-user access, or remote sharing.
- Changing the underlying capture wrapper format, decode model, or Aperture request contract.

## Capabilities

### New Capabilities
- `analysis-web-ui`: Serve a localhost-only web application for live analysis sessions, including current status, uploaded analysis output, and session counters.

### Modified Capabilities
- `aperture-analysis`: `analyze` must support explicit presentation mode selection and start the correct local session surface for the chosen mode.
- `analysis-session-ui`: the existing interactive analysis session requirements must be scoped to console mode so the Bubble Tea UI remains available without being the only live session interface.

## Impact

- Affected code: `cmd/analyze.go`, `internal/analyze/session.go`, and the existing console UI code in `internal/analyze/`.
- New code: localhost HTTP serving, browser-oriented session rendering, and session-to-UI state publication for web mode.
- User-facing API: new `analyze` mode flag and localhost web startup behavior.
- Dependencies/systems: Go standard library HTTP serving and whichever local browser-opening or static asset approach fits the existing CLI design.
