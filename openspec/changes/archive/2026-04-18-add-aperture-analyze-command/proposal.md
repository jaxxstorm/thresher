## Why

`thresher` can already decode Tailscale capture traffic into useful JSONL, but there is no built-in way to hand that stream to an LLM and get ongoing analysis about what is happening during a capture. This change is needed now to turn decoded packet output into an interactive analysis workflow while keeping the integration simple by supporting only Tailscale Aperture endpoints and avoiding local credential or key management entirely.

## What Changes

- Add a new `thresher analyze` command, with `analyse` as a supported alias, that consumes capture output and sends analysis requests to an Aperture-served LLM endpoint
- Require an explicit `--endpoint` flag such as `--endpoint http://ai` and limit upstream support to Aperture-compatible `/v1/chat/completions`, `/v1/messages`, and `/v1/responses` style endpoints rather than arbitrary authenticated providers
- Add model selection, batching, and upload controls so users can choose which model to use and cap how much capture data is sent to avoid runaway cost
- Add a Bubble Tea-based interactive TUI that shows capture progress, batching/upload state, live agentic analysis output, and allows model switching within an analysis session when supported
- Add config-file support for analyze defaults such as endpoint, model, batching, and capture-size limits
- Keep the existing capture JSONL stream as the analysis substrate rather than changing the packet wrapper or nested decode format

## Non-goals

- No support for arbitrary third-party hosted LLM providers that require direct API key handling inside `thresher`
- No changes to the Tailscale packet wrapper, including the 2-byte little-endian `path_id`, SNAT/DNAT length fields, or DISCO wire structure
- No replacement of the current capture and decode pipeline; the analyze workflow builds on the existing JSONL output
- No assumption that all models expose identical upstream request shapes beyond the Aperture-compatible OpenAI/Anthropic-style endpoints it fronts

## Capabilities

### New Capabilities

- `aperture-analysis`: Analyze decoded capture streams by sending bounded batches of JSONL-derived packet context to an Aperture LLM endpoint and returning agentic explanations
- `analysis-session-ui`: Present capture ingestion, upload state, live model responses, and interactive session controls in a Bubble Tea TUI
- `analysis-config`: Configure analysis defaults such as endpoint, model, batching, and cost-limiting settings via config file and flags

### Modified Capabilities

- `localapi-capture`: extend the live-capture workflow so decoded packet output can be fed into analysis sessions rather than only printed or written as output
- `cli-root`: add analyze/analyse command discovery and configuration wiring for the new analysis workflow

## Impact

- **CLI surface**: new `analyze` / `analyse` command plus endpoint, model, batching, and cost-limiting flags
- **Networking**: new Aperture client logic for model discovery and LLM request/response handling against `/v1/messages`, `/v1/chat/completions`, and `/v1/responses`
- **UI**: Bubble Tea-based interactive interface for session progress and live analysis output
- **Config**: new persisted defaults for endpoint, model, batch sizing, and data limits
- **Testing**: new fixtures and mocked Aperture responses for batching, upload limits, model selection, and interactive session behavior
