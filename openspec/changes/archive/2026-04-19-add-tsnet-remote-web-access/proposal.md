## Why

`thresher analyze --mode web` currently only serves a localhost browser view, which means the tool has to run on the same machine where the user is watching the session. That blocks the remote-debugging case where capture and analysis need to run near the target host while the user watches the web UI over the tailnet.

## What Changes

- Add a tsnet-backed remote web serving mode for `thresher analyze --mode web` so the active analysis session can be viewed over the Tailscale network instead of only through a localhost listener.
- Require remote access to be gated by a single Tailscale capability, `lbrlabs.com/cap/thresher`, because the web surface is a single-page session UI rather than a multi-endpoint application.
- Preserve the existing local capture and Aperture analysis flow while reusing the same session state, pause controls, model controls, and streamed analysis output in the remote view.
- Keep localhost-only web mode available as the default behavior so remote serving remains explicit and does not widen exposure unexpectedly.

## Non-goals

- Replacing the existing console mode or changing the packet decode format, wrapper semantics, or Aperture request payloads.
- Introducing multiple remote pages, per-feature capability names, or a general-purpose public API.
- Requiring direct internet exposure, standalone TLS termination, or a separate reverse proxy outside the Tailscale network.

## Capabilities

### New Capabilities
- `tsnet-remote-web-access`: Serve the analyze web session on the tailnet through tsnet with a single `lbrlabs.com/cap/thresher` capability gate.

### Modified Capabilities
- `analysis-web-ui`: Expand web mode from localhost-only viewing to support an explicit tailnet-served session surface while preserving live state updates and browser controls.
- `aperture-analysis`: Extend the analyze command contract so web mode can be started in a local-only or tailnet-served form without changing the underlying Aperture analysis workflow.

## Impact

- Affected code will include `cmd/analyze.go`, the analyze web presenter/runtime under `internal/analyze`, and any new package needed to host the web UI over tsnet.
- Adds a tsnet-backed listener path and Tailscale capability configuration using `lbrlabs.com/cap/thresher`.
- Changes command and web-mode documentation to explain when the browser UI is localhost-only versus tailnet-accessible.
