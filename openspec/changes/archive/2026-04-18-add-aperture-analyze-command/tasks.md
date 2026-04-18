## 1. CLI And Config Surface

- [x] 1.1 Add `analyze` with `analyse` as an alias under `cmd/`, wiring it into the CLI root without regressing existing subcommands
- [x] 1.2 Add analyze flags for `--endpoint`, `--model`, batching controls, and upload or cost limiting values using kebab-case names
- [x] 1.3 Extend viper-backed config loading so analysis defaults such as endpoint, model, batch size, and data caps can be set in config and overridden by flags
- [x] 1.4 Add command-level tests for analyze/analyse registration, required endpoint handling, and flag-overrides-config behavior

## 2. Aperture Client

- [x] 2.1 Implement an internal Aperture client package that targets only Aperture-compatible paths beneath the configured base endpoint such as `/v1/messages`, `/v1/chat/completions`, and `/v1/responses`
- [x] 2.2 Add request-shape handling for the supported endpoint styles while exposing one unified API to the analysis workflow
- [x] 2.3 Add model selection and best-effort model discovery support when Aperture exposes a compatible model-list endpoint
- [x] 2.4 Add tests using mocked Aperture HTTP handlers for request formatting, model selection, discovery success, and discovery fallback behavior

## 3. Capture-To-Analysis Pipeline

- [x] 3.1 Add an analysis pipeline that consumes decoded JSONL-style packet records from live capture or saved input without changing the existing packet wrapper decode
- [x] 3.2 Implement bounded rolling batches over decoded packet rows with configurable limits for packets, bytes, and total session upload volume
- [x] 3.3 Add prompt-building logic that summarizes packet windows for the LLM while preserving enough flow and analyzer context for useful explanations
- [x] 3.4 Add tests for batch sizing, upload caps, session stop conditions, and prompt construction from decoded packet records

## 4. Interactive Session UI

- [x] 4.1 Add a Bubble Tea-based TUI for analysis sessions that shows capture progress, batch or upload state, active model, and live analysis output
- [x] 4.2 Add interactive session controls for pausing analysis, showing session limits or counters, and switching models when supported by the configured endpoint
- [x] 4.3 Add UI-focused tests for session state transitions, model-switch events, and limit-reached messaging

## 5. Integration And Verification

- [x] 5.1 Integrate the analyze workflow with the existing local capture stream so live decoded packets can feed the analysis pipeline directly
- [x] 5.2 Document how Aperture endpoints are configured, how model selection works, and how batching or cost-limiting flags protect against runaway upload volume
- [x] 5.3 Add manual verification steps for `go run . analyze --endpoint http://ai ...` covering live updates, model selection, and bounded upload behavior
- [x] 5.4 Run `go test ./...` and verify the analyze command does not require local key or auth handling beyond the configured Aperture endpoint

### Manual Verification Steps

- Run `go run . analyze --endpoint http://ai --model gpt-4o` and confirm the command starts without any local key or auth prompt
- Verify the TUI shows packet count, status updates, available models when `/v1/models` is exposed, and incremental analysis output
- Use `m` in the TUI to switch models when multiple models are available and confirm the active model display updates
- Use `p` to toggle paused/resumed state in the session UI
- Run with bounded flags such as `--batch-packets 5 --session-packets 10` and verify the session reports when upload limits are reached
- Run `go run . analyze --endpoint http://ai --model gpt-4o --input capture.jsonl` and confirm saved JSONL input is analyzed without requiring live capture
