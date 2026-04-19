## 1. Endpoint Default And Analyze Startup

- [x] 1.1 Update [cmd/analyze.go](/Users/lbriggs/src/github/jaxxstorm/thresher/cmd/analyze.go) so `analyze.endpoint` defaults to `http://ai` through viper config precedence instead of returning an endpoint-required error when flags and config are empty
- [x] 1.2 Extend [cmd/analyze_test.go](/Users/lbriggs/src/github/jaxxstorm/thresher/cmd/analyze_test.go) to verify the built-in `http://ai` default is used, config still overrides it, and `--endpoint` still wins over config

## 2. Session State For Full-Screen Rendering

- [x] 2.1 Refactor [internal/analyze/session.go](/Users/lbriggs/src/github/jaxxstorm/thresher/internal/analyze/session.go) and [internal/analyze/ui.go](/Users/lbriggs/src/github/jaxxstorm/thresher/internal/analyze/ui.go) so the UI model tracks explicit dashboard state for packet totals, batch progress, upload state, limit state, and recent analysis history instead of relying on one free-form status string
- [x] 2.2 Add or update tests under [internal/analyze](/Users/lbriggs/src/github/jaxxstorm/thresher/internal/analyze) to cover state transitions for record ingestion, upload start or finish, model discovery, pause state, and limit-reached messaging while preserving the existing decoded JSONL-driven session flow built from fields such as `path_id`, `inner.src_ip`, and `disco_meta`

## 3. Full-Window TUI Layout And Interaction

- [x] 3.1 Rebuild [internal/analyze/ui.go](/Users/lbriggs/src/github/jaxxstorm/thresher/internal/analyze/ui.go) into a full-screen Bubble Tea layout that uses the available terminal window, separates summary panels from analysis output, and presents model or status information with clear visual hierarchy
- [x] 3.2 Add resize-aware behavior in [internal/analyze/ui.go](/Users/lbriggs/src/github/jaxxstorm/thresher/internal/analyze/ui.go) so `tea.WindowSizeMsg` recomputes the layout for wide and narrow terminals without losing visibility of critical session counters or recent analysis text
- [x] 3.3 Add UI-focused tests in [internal/analyze](/Users/lbriggs/src/github/jaxxstorm/thresher/internal/analyze) that verify the full-window view renders the active endpoint, model, packet counters, upload state, and analysis history in a stable layout across representative window sizes

## 4. Program Startup And Session Wiring

- [x] 4.1 Update [internal/analyze/session.go](/Users/lbriggs/src/github/jaxxstorm/thresher/internal/analyze/session.go) to start Bubble Tea in the correct full-screen mode and keep incremental updates flowing from live capture or saved JSONL batches into the richer UI
- [x] 4.2 Add or update session tests in [internal/analyze/client_test.go](/Users/lbriggs/src/github/jaxxstorm/thresher/internal/analyze/client_test.go) and related files to verify batching, analysis delivery, and limit handling still work while consuming the same decoded packet records derived from the wrapper fields such as the 2-byte little-endian `path_id` and nested `inner` or `disco_meta` payloads

## 5. Documentation And Verification

- [x] 5.1 Update [docs/analyze-command.md](/Users/lbriggs/src/github/jaxxstorm/thresher/docs/analyze-command.md) and related help text to document the default `http://ai` endpoint, the full-screen analyze experience, and the revised expectations for session controls and layout
- [x] 5.2 Run `go test ./...` and perform a manual smoke test with `go run . analyze --model <model>` to verify the fullscreen UI starts cleanly, uses `http://ai` by default, refreshes dynamically, and does not alter the decoded packet substrate or packet-wrapper semantics
