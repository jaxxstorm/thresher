## Why

`thresher analyze` currently sends requests to the Aperture endpoint with Go's default `go-http-client` user agent, which makes it harder to identify traffic from this CLI in endpoint logs or policy layers. Updating that header now gives Aperture operators a stable, versioned client identity without changing the capture format or analysis request payloads.

## What Changes

- Update the analysis HTTP client so requests sent to Aperture include a `User-Agent` header in the form `thresher/<version>`.
- Keep the existing analyze request paths, payloads, batching, and decoded packet handling unchanged.
- Document that the user agent identifies the CLI version rather than exposing packet or capture-specific metadata.

## Non-goals

- Changing the packet wrapper format, decoded packet schema, or any wire-format behavior described by the capture spec.
- Changing Aperture request bodies, response handling, authentication, or endpoint discovery behavior.
- Adding per-request user agent customization through flags or config in this change.

## Capabilities

### New Capabilities

### Modified Capabilities
- `aperture-analysis`: analysis requests must identify themselves with a stable `thresher/<version>` user agent instead of the generic Go default header.

## Impact

- Affected code: `internal/analyze/client.go`, version plumbing if needed, and related tests under `internal/analyze/` and `cmd/`.
- External behavior: HTTP requests to Aperture gain an explicit `User-Agent` header value.
- Dependencies/systems: no new external dependencies; continues using the existing Go HTTP client stack.
