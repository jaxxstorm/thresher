## Why

`thresher analyze` can issue many Aperture requests while one console or web session is running, but those uploads currently look like unrelated events instead of one conversation. Adding a stable per-session identifier now lets Aperture group cost, logs, and traces around a whole Thresher analysis run without changing packet decoding or prompt content.

## What Changes

- Generate one logical session identifier for each `thresher analyze` invocation and reuse it across all Aperture analysis requests emitted by that session.
- Map the session identifier into endpoint-style-specific request fields for `/v1/messages`, `/v1/responses`, and `/v1/chat/completions` so Aperture can group repeated uploads from the same analysis run.
- Keep existing batching, prompt construction, model selection, and web or console presentation behavior unchanged, including packet-derived fields such as `path_id`, `snat`, `dnat`, and DISCO metadata.
- Document the session-tracking behavior so operators can correlate one Thresher session with one grouped conversation in Aperture.

## Non-goals

- Changing the capture wrapper, including the 2-byte little-endian `path_id`, SNAT or DNAT length fields, or DISCO wire format.
- Changing model discovery, model switching, or batch sizing semantics beyond attaching a stable session identifier to outbound analysis requests.
- Supporting cross-process session resume; each new `thresher analyze` invocation gets a fresh identifier.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `aperture-analysis`: analysis sessions must carry a stable session-tracking identifier across repeated Aperture requests, using request fields that match the selected endpoint style.

## Impact

- Affected code: `internal/analyze/session.go`, `internal/analyze/client.go`, presenter or state plumbing used to expose the active session, and related tests.
- APIs: outbound Aperture analysis requests gain session-tracking headers or body metadata; no packet wire-format changes are introduced.
- Systems: Aperture Logs can attribute multiple batched requests to one Thresher analysis session.
