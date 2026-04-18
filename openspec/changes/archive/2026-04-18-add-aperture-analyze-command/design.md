## Context

`thresher` already has a strong packet-decoding pipeline that can stream live Tailscale debug capture output as structured JSONL. The missing piece is an analysis workflow that can continuously explain what the decoded traffic means. The user requirement is explicit: use only Tailscale Aperture as the upstream LLM transport, do not handle credentials or keys locally, and make the analysis loop feel interactive and agent-like rather than a one-shot batch upload.

This change spans multiple concerns at once: a new CLI command, upstream HTTP client behavior for Aperture-compatible endpoints, bounded batching and upload controls to avoid surprise cost, a Bubble Tea session UI for real-time analysis, and config-file support for endpoint/model defaults. That makes it a cross-cutting feature requiring clear design decisions before implementation.

## Goals / Non-Goals

**Goals:**
- Add `thresher analyze` with `analyse` as an alias for LLM-backed live or file-based packet analysis
- Require an explicit Aperture base endpoint via `--endpoint` or config and avoid any direct API-key or provider-auth handling in `thresher`
- Support user-selectable models and discover available models from Aperture when possible
- Batch decoded JSONL intelligently with configurable size limits and cost-control flags so uploads remain bounded
- Provide real-time, agentic analysis updates through a Bubble Tea TUI that shows capture progress, upload/batch state, model, and live analysis messages
- Persist sensible analysis defaults such as endpoint, model, and batch limits via config, with flags overriding config values

**Non-Goals:**
- Supporting arbitrary LLM vendors directly from `thresher`
- Changing the Tailscale packet wrapper, DISCO format, or the underlying capture JSONL schema as part of this change
- Building a fully general coding-agent environment with tool invocation; the analysis loop is conversational/explanatory, not an unrestricted tool-using agent runtime
- Guaranteeing all Aperture upstreams expose identical request or model-list endpoints; the implementation only needs to support the Aperture-compatible shapes the user described and degrade clearly when one is unavailable

## Decisions

### Endpoint policy: Aperture-only with explicit base URL

**Decision**: The analysis command accepts a required Aperture base URL via `--endpoint` or config and only speaks to Aperture-compatible paths beneath that base, such as `/v1/messages`, `/v1/chat/completions`, `/v1/responses`, and a model-list endpoint when available.

**Rationale**: This preserves the user’s requirement that `thresher` not manage credentials or direct vendor integrations. The CLI can assume that authentication, routing, and key management have already been solved by Aperture.

**Alternatives considered:**
- Native OpenAI/Anthropic integrations: rejected because they require local auth/key handling
- Implicit localhost default endpoint: rejected because the user asked for an explicit endpoint flag such as `--endpoint http://ai`

### Transport abstraction: one analysis client with endpoint-shape adapters

**Decision**: Implement a small internal Aperture client abstraction that can render requests into the supported upstream-compatible shapes (`messages`, `chat completions`, `responses`) based on user selection or model/provider compatibility, while presenting one unified API to the rest of the app.

**Rationale**: The TUI and batching logic should not care whether the selected model is reached through a Claude-style `messages` endpoint or an OpenAI-style `chat completions`/`responses` endpoint.

**Alternatives considered:**
- Hardcode only one endpoint shape: rejected because the user explicitly showed multiple Aperture-compatible request shapes and wants model choice flexibility
- Expose raw JSON body construction to users: rejected because it is not user-friendly

### Batching and upload controls: default to bounded rolling windows plus hard caps

**Decision**: Analyze capture data in bounded rolling batches derived from decoded JSONL records rather than streaming every packet immediately. Expose flags and config for limits such as maximum packets per batch, maximum bytes uploaded per batch, maximum total packets or bytes per session, and optional session-duration ceilings.

**Rationale**: Raw live capture can grow quickly and produce uncontrolled model usage. A rolling-window approach with hard caps gives the LLM enough context for meaningful analysis while keeping spend predictable.

**Alternatives considered:**
- Upload the entire JSONL stream continuously with no limits: rejected because it risks runaway costs
- Force users to pre-save a file and analyze afterward: rejected because the request explicitly wants real-time updates

### Analysis flow: summarize packet windows, not raw binary blobs

**Decision**: Build prompts from the decoded packet JSONL stream, using normalized packet rows and selected nested details as the substrate. The prompt builder should summarize windows of packets, include notable analyzer flags and flow context, and optionally include raw JSONL excerpts when needed, rather than uploading arbitrary binary capture data.

**Rationale**: The existing decode pipeline already turns packet bytes into structured, LLM-friendly records. Reusing that prevents duplicate parsing and keeps the analysis workflow grounded in the same packet semantics users see locally.

### UI: Bubble Tea session view with incremental analysis panes

**Decision**: Use Bubble Tea for an interactive analysis session UI with panes or sections for capture status, batch/upload progress, selected model, recent packet/batch summaries, and live LLM analysis output. Add interactive commands or keybindings for model switching, pausing analysis, and viewing session stats when the endpoint supports it.

**Rationale**: The user explicitly asked for a coding-agent-like feel with real-time updates. Bubble Tea fits the existing CLI ecosystem well and makes it easier to present long-running interactive state than plain line-oriented logs.

**Alternatives considered:**
- Plain stdout streaming only: rejected because it will become noisy and hard to control in a long-running session
- Full-screen bespoke UI framework: rejected because Bubble Tea is sufficient and aligns with the request

### Config model: analysis defaults in viper-backed config with flag precedence

**Decision**: Add config support for endpoint, preferred model, endpoint shape preference, batch size, total upload limits, and UI-related defaults. Command-line flags override config file values, and environment variables continue to flow through viper.

**Rationale**: The user expects model and endpoint defaults to be saved once rather than repeated for every session.

### Model discovery: query Aperture model listing when available and degrade gracefully

**Decision**: Investigate and support a model-list request path if Aperture exposes one. If model discovery is unavailable for a given endpoint, allow explicit model configuration without blocking the command.

**Rationale**: Model switching is far more usable if the UI can show concrete available models, but the command must still work with an explicitly provided model even if discovery is not supported by the endpoint.

## Risks / Trade-offs

- **Aperture endpoint capabilities may vary by deployment** → Mitigation: design explicit fallback paths for model discovery and endpoint-shape selection, and surface clear UI errors instead of assuming uniform behavior
- **Real-time LLM analysis can still become expensive even with batching** → Mitigation: enforce hard default session limits and expose obvious UI/session counters for packets, bytes, and batches sent
- **Long-running TUI sessions can drift from the underlying capture timeline** → Mitigation: keep capture ingestion, batch scheduling, and response rendering as separate state machines with visible timestamps and counters
- **Prompting the LLM with too much raw JSON can overwhelm context windows** → Mitigation: summarize packet windows first, attach only focused excerpts, and cap batch payload size strictly
- **Model switching mid-session may produce inconsistent analysis style** → Mitigation: clearly show the active model in the UI and treat model switches as session events recorded in the analysis history

## Migration Plan

- Add the new analyze/analyse CLI command and config wiring without changing existing capture behavior
- Implement the Aperture client and model discovery layer behind an internal package boundary
- Add the bounded batcher that consumes the existing decoded JSONL stream
- Build the Bubble Tea TUI around capture/batch/analyze session state
- Add integration-style tests with mocked Aperture endpoints and manual verification using `go run . analyze --endpoint http://ai ...`
- If rollback is needed, remove the analyze command and leave the existing capture/read pipeline untouched

## Open Questions

- Which Aperture model-list endpoint is actually available and stable enough to rely on for discovery?
- Should `analyze` default to reading from live capture, an input file, or require the user to choose explicitly when both are possible?
- What should the default hard limits be for packets, bytes, or elapsed analysis time to best balance usefulness against cost?
