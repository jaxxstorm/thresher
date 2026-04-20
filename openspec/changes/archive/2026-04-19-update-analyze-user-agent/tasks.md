## 1. Versioned User-Agent Plumbing

- [x] 1.1 Update `cmd/analyze.go` and `internal/analyze/client.go` so the analyze client receives the resolved CLI version and sends `User-Agent: thresher/<version>` on Aperture requests without changing request paths, payload bodies, or decoded-record prompt content such as `path_id`, `snat`, `dnat`, and `payload_preview`.
- [x] 1.2 Add or update tests in `internal/analyze/client_test.go` to verify both `/v1/models` and analysis requests include the versioned `User-Agent` header while preserving the existing request body fields derived from decoded packets, including `path_id`, `snat`, `dnat`, and payload text.

## 2. Integration And Verification

- [x] 2.1 Wire any required version fallback through the analyze command/session construction path so development builds still emit a stable `thresher/<version>` header value without affecting console or web mode selection.
- [x] 2.2 Run targeted tests for `cmd` and `internal/analyze`, plus a manual smoke test against an Aperture-compatible endpoint, to confirm the request header changes from `go-http-client` to `thresher/<version>` and that analyze behavior remains unchanged for prompts built from decoded packet fields such as `path_id` and `payload_preview`.
