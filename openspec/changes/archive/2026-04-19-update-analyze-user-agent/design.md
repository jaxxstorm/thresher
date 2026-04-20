## Context

`thresher analyze` already centralizes Aperture HTTP requests in `internal/analyze/client.go`, while version reporting for the CLI is handled elsewhere through the existing version command plumbing. Today the analyze client relies on Go's default HTTP behavior, so Aperture sees `go-http-client` rather than a thresher-specific identity. The requested change is narrowly scoped: keep the packet format, request payloads, and endpoint behavior unchanged while making outbound analysis requests identify the CLI and its version.

## Goals / Non-Goals

**Goals:**
- Send `User-Agent: thresher/<version>` on analyze HTTP requests to Aperture.
- Reuse the existing CLI version source rather than inventing a second version string.
- Keep request paths, payload bodies, batching, and decode behavior unchanged.
- Add tests that verify the header is present on both analysis and model-discovery requests.

**Non-Goals:**
- Adding user-configurable user-agent overrides.
- Changing non-analyze HTTP clients in unrelated parts of the CLI.
- Modifying packet wrapper parsing, decoded record fields, or Aperture request bodies.

## Decisions

### Thread the CLI version into the analyze HTTP client explicitly
The analyze client should accept or derive the running thresher version once and use it to set `User-Agent` on each request. This keeps the header logic local to the HTTP client instead of scattering ad hoc header mutation around command code.

Alternative considered: hard-code a static header such as `thresher/dev`. This was rejected because the request should identify the running CLI version, not just the binary name.

### Apply the header uniformly across analyze and model-discovery requests
Both `/v1/models` discovery requests and analysis submission requests should use the same user agent, because both are part of the analyze workflow and hit the same endpoint family.

Alternative considered: set the header only on analysis submission requests. This was rejected because it would leave the endpoint with inconsistent client identity across the same session.

### Preserve request payload and transport behavior
The change should update only request headers. Endpoint selection, request bodies, timeouts, and batching logic remain unchanged so there is no packet-format or prompt-construction impact.

Alternative considered: use a custom `http.RoundTripper` just to stamp headers. This was rejected because the client currently builds requests directly, and setting the header when constructing the request is simpler and easier to test.

## Risks / Trade-offs

- [Version plumbing is not available where the analyze client is constructed] -> Thread the resolved version through config or constructor parameters once at command startup.
- [Tests become brittle around development-version formatting] -> Assert the expected `thresher/` prefix plus the resolved version string used in the test setup.
- [Future analyze requests bypass the shared client helper] -> Keep header setting inside the analyze client's request-construction path so new request types inherit it automatically.

## Migration Plan

No data migration is required. The change is a request-header update only. Rollback is straightforward: remove the explicit `User-Agent` assignment and the client returns to the Go default behavior.

## Open Questions

- Should development builds use the exact existing version string as reported by the CLI, even if it is something like `dev`, or should the analyze client normalize it further?
