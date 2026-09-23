## Context

`thresher analyze` already batches decoded packet records and can send multiple LLM requests during one run as new capture windows are uploaded. Aperture can group those requests into a single session when the client provides a stable session identifier, but Thresher currently only sends request bodies plus `User-Agent`, so one analysis run is fragmented across multiple unrelated events.

The requested change is narrow in code but cross-cutting in behavior. Session identity must be created once per analysis invocation, threaded through repeated uploads, mapped into three supported endpoint shapes, and remain compatible with both console and web presentation modes. The packet wrapper itself stays unchanged: the 2-byte little-endian `path_id`, SNAT and DNAT fields, and DISCO metadata remain analysis inputs, not part of the session-tracking transport.

## Goals / Non-Goals

**Goals:**
- Generate one fresh logical session identifier for each `thresher analyze` invocation.
- Reuse that identifier across every analysis upload produced by the same run, even when batching, pausing, or model switching leads to multiple requests.
- Encode the identifier in Aperture-compatible request locations for `/v1/messages`, `/v1/responses`, and `/v1/chat/completions`.
- Keep packet-derived prompt fields such as `path_id`, `snat`, `dnat`, and `payload_preview` unchanged.
- Make the active session identifier observable enough to verify locally and correlate with Aperture.

**Non-Goals:**
- Changing model discovery or treating `/v1/models` calls as part of a tracked analysis conversation.
- Persisting session identifiers across separate CLI invocations or adding resume semantics.
- Modifying packet capture decoding, the wrapper byte layout, or DISCO parsing.

## Decisions

### Generate one canonical session ID at session construction time

Generate a canonical identifier in `NewSession` when one `thresher analyze` run starts. The canonical form is `session_<uuid>`, which is stable for the life of that analyze invocation and replaced on the next invocation.

This keeps session identity aligned with the actual unit the user cares about: one console or web analysis run. It also prevents request builders from minting ad hoc IDs per batch.

Alternatives considered:
- Generate a new ID per batch upload: rejected because Aperture would still see fragmented conversations.
- Reuse a persisted ID from config: rejected because separate runs should not collapse into one logical session.

### Keep one logical session ID but serialize it per endpoint style

Use one logical session identifier and map it into the request shape Aperture expects for each style:
- `/v1/messages`: include `metadata.user_id = session_<uuid>` in the JSON body.
- `/v1/responses`: include `Session_id: session_<uuid>` as an HTTP header.
- `/v1/chat/completions`: include `Session_id: <fingerprint>` where `<fingerprint>` is a deterministic 16-character lowercase hex digest derived from the canonical session ID.

This preserves a single analysis session concept while still fitting the different request styles Thresher already supports. The chat-completions serialization uses a shorter fingerprint because that shape is commonly grouped by compact conversation fingerprints rather than a full UUID-like identifier.

Alternatives considered:
- Use the same full `session_<uuid>` header value for every endpoint: rejected because the chat-completions path benefits from a compact fingerprint shape.
- Rely on Aperture to infer chat-completions sessions from request or response content: rejected because the user asked for Thresher to send a session-tracking identifier explicitly.

### Apply session tracking only to analysis uploads

Stamp session-tracking fields only on the requests that carry analysis prompts and responses. Leave `/v1/models` discovery untracked, because it is setup traffic rather than part of the ongoing conversation represented in Aperture Logs.

Alternatives considered:
- Add the same session identifier to model discovery requests: rejected because it would mix discovery with the actual analysis conversation and make grouped session costs noisier.

### Expose the active session identifier in session state

Add the generated session identifier to the in-memory analysis session state so console or web mode, logs, and tests can surface it for correlation. This keeps verification local and avoids forcing operators to infer which grouped Aperture session belongs to a given CLI run.

Alternatives considered:
- Keep the identifier internal to the HTTP client: rejected because it would be harder to test and harder for users to correlate local and remote session views.

## Risks / Trade-offs

- [Aperture expects a different field for chat-completions session grouping] -> Keep the chat-completions serialization behind one helper so the field choice can be adjusted without changing session lifecycle semantics.
- [Session ID exposure in the UI adds clutter] -> Surface it as secondary session metadata rather than a primary workflow control.
- [Future analysis request types bypass the shared builder] -> Centralize all request stamping in `internal/analyze/client.go` so new request shapes inherit the same session-tracking logic.

## Migration Plan

This is an additive request-format change with no persisted state migration. Implementation should:
1. Generate and store the canonical session identifier when the analysis session is created.
2. Thread that identifier into the analyze client and serialize it per endpoint style.
3. Add targeted tests for repeated uploads, style-specific request encoding, and optional session-state exposure.
4. Update analyze documentation and manual verification steps so one local run can be matched to one grouped Aperture session.

Rollback is straightforward: remove the session-tracking serialization and state field while leaving batching, prompts, and packet decoding untouched.

## Open Questions

- None at this time.
