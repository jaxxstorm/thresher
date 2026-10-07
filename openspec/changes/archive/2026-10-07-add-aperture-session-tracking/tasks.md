## 1. Session Identity Plumbing

- [x] 1.1 Add session-identity helpers in `internal/analyze/session.go` or a focused helper file so each `thresher analyze` invocation creates one canonical `session_<uuid>` value without changing packet-derived prompt fields such as `path_id`, `snat`, `dnat`, `payload_preview`, or DISCO metadata.
- [x] 1.2 Add unit tests for the session-identity helper and session lifecycle to verify one run reuses its identifier across multiple uploads while a later run gets a fresh identifier.

## 2. Endpoint-Specific Request Mapping

- [x] 2.1 Update `internal/analyze/client.go` so `/v1/messages` writes `metadata.user_id`, `/v1/responses` sends `Session_id: session_<uuid>`, and `/v1/chat/completions` sends the deterministic 16-character fingerprint header, without changing existing prompt fields derived from `path_id`, `snat`, `dnat`, or `payload_preview`.
- [x] 2.2 Extend `internal/analyze/client_test.go` with request-shape tests for all three endpoint styles, asserting both the session-tracking fields and the unchanged packet-derived request content.

## 3. Session Visibility And Documentation

- [x] 3.1 Update `internal/analyze/state.go` and the relevant presenter surfaces so the active analysis session identifier can be inspected for local correlation without altering wrapper semantics such as the 2-byte little-endian `path_id` field or SNAT and DNAT handling.
- [x] 3.2 Update `docs/analyze-command.md` to explain how one `thresher analyze` invocation maps to one grouped Aperture session and how the per-session identifier behaves across multiple uploads.

## 4. Verification

- [x] 4.1 Run targeted tests for `./internal/analyze` and `./cmd` to confirm batching, pausing, and model switching do not rotate the active session identifier.
- [x] 4.2 Perform a manual smoke test against an Aperture-compatible endpoint and confirm one local analyze run appears as one grouped session in Aperture while packet-level inputs such as `path_id`, `snat`, `dnat`, and DISCO details remain unchanged.

Verification: `go test ./internal/analyze ./cmd -count=1`, `go test -race ./internal/analyze ./cmd -count=1`, and `go test ./... -count=1` passed. A manual reader-session smoke test against `http://ai/v1/messages` using `claude-haiku-4-5` successfully uploaded two synthetic packet batches with a 64-token response limit. Canonical ID: `session_364e8b78-c659-44d3-ae96-e2d4da3bbcdb`; chat fingerprint: `390b16229db76081`. The temporary live test was removed so automated tests remain self-contained.

Grouping confirmed through `http://ai` on 2026-09-23: filtering `/api/load-metrics` and `/api/sessions` to 10:35-10:38 UTC, model `claude-haiku-4-5`, and user agent `thresher/dev` returned two successful requests in one stored session, `ancc_78ef4d544e3c6538`, with `request_count: 2`. `/api/sessions/ancc_78ef4d544e3c6538/captures?page=0&limit=10` independently returned exactly two captures: `CfLDukFou2jMMqtjm4JX7` and `CfLDurjF3wWUNwev7d9Jh`. Both captured bodies retained the canonical `metadata.user_id`, `path_id=7`, `snat=192.0.2.10`, `dnat=192.0.2.20`, and synthetic DISCO info and payload. Aperture's stored key differs from the submitted identity; its derivation is not established by this smoke test. Live verification covered messages; all endpoint mappings and identity lifecycle behavior are covered by automated tests.
