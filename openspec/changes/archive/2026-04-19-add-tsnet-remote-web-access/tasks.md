## 1. Command and Config Wiring

- [x] 1.1 Update `cmd/analyze.go` to add an explicit web-access control for `--mode web`, default it to local-only behavior, and thread the selected access mode into `analyze.Config` without changing the existing `http://ai` endpoint or console-mode defaults
- [x] 1.2 Extend `cmd/analyze_test.go` to verify `thresher analyze --mode web` stays localhost-only by default and switches into the tailnet-serving path only when the explicit web-access setting is provided

## 2. Remote Web Runtime

- [x] 2.1 Refactor `internal/analyze/web.go` so the existing page, snapshot, events, and control handlers can run behind either the current localhost listener or a `tsnet.Server` listener without duplicating the presenter logic
- [x] 2.2 Add focused tests under `internal/analyze` to verify the shared web presenter still exposes live session state, pause, and model controls while consuming the same decoded JSONL-driven analysis flow built from fields such as `path_id`, `inner.src_ip`, and `disco_meta`

## 3. Tailnet Authorization

- [x] 3.1 Add a tsnet-backed remote serving path under `internal/analyze` that starts the analyze web UI on the tailnet, resolves the advertised remote URL, and shuts down cleanly with the session lifecycle
- [x] 3.2 Add authorization middleware and supporting types so remote requests are allowed only when the peer has `lbrlabs.com/cap/thresher`, while keeping the remote page, snapshot, event, and control routes under one shared permission surface
- [x] 3.3 Add or update tests under `internal/analyze` to cover allowed and denied remote requests, single-surface route handling, and clean shutdown on quit or session completion

## 4. Documentation and Verification

- [x] 4.1 Update `docs/analyze-command.md` and related help text to document local versus tailnet web access, the `lbrlabs.com/cap/thresher` requirement, and the remote-view workflow for running capture near the target host
- [x] 4.2 Run `go test ./cmd ./internal/analyze` and perform a manual smoke test with `go run . analyze --mode web ...` in both local and tailnet forms to confirm remote access works without changing the decoded packet substrate, including wrapper-derived fields such as `path_id`, `inner`, and `disco_meta`
