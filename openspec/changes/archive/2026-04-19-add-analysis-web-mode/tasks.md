## 1. Analyze mode selection

- [x] 1.1 Add `--mode` flag parsing and configuration defaults in `cmd/analyze.go`, with `console` as the default and validation for `console` and `web` values.
- [x] 1.2 Add command tests in `cmd/analyze_test.go` covering default `console` mode, explicit `--mode web`, and invalid mode rejection.

## 2. Shared session state

- [x] 2.1 Refactor `internal/analyze/session.go` so session progress, counters, active model, and analysis text are emitted through a presentation-agnostic state/update interface instead of being coupled directly to Bubble Tea messages.
- [x] 2.2 Add tests in `internal/analyze/` verifying the shared session state preserves existing analysis behavior and does not change packet-derived fields such as `path_id`, `snat`, `dnat`, or payload-driven prompts built from decoded records.

## 3. Console mode integration

- [x] 3.1 Adapt the existing Bubble Tea console UI in `internal/analyze/ui.go` to consume the shared session state while preserving full-screen layout, live updates, pause handling, and model selection in `console` mode.
- [x] 3.2 Update console UI tests in `internal/analyze/ui_test.go` to cover the refactored state flow and confirm resize, scrolling, and limit states still render correctly.

## 4. Localhost web mode

- [x] 4.1 Implement a localhost-only web presenter in `internal/analyze/` that starts an HTTP server bound to `127.0.0.1`, serves the session page, and exposes a current snapshot plus live update feed for analysis state.
- [x] 4.2 Add tests for the web presenter covering localhost binding, snapshot rendering, live update delivery, and session shutdown cleanup.

## 5. Analyze command wiring

- [x] 5.1 Wire `cmd/analyze.go` to construct the correct presenter for `console` or `web` mode and report the resolved localhost URL when web mode starts.
- [x] 5.2 Add integration-style command tests covering file-backed or mocked analysis sessions in web mode and confirming batching and limits still apply without changing decode handling for wrapper-derived fields like `path_id` and payload content.

## 6. Verification

- [x] 6.1 Run targeted Go tests for `cmd` and `internal/analyze` covering console and web modes.
- [x] 6.2 Run a manual smoke test with `go run . analyze --mode console` and `go run . analyze --mode web` to verify the console session still works and the browser UI is reachable on localhost.
