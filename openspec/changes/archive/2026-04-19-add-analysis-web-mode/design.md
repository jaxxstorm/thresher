## Context

`thresher analyze` currently couples session execution to a Bubble Tea console UI. The command already owns the analysis session lifecycle, packet batching, model discovery, and request flow, while `internal/analyze/ui.go` renders state changes for interactive terminals. Adding a browser experience crosses command wiring, session state publication, and presentation concerns, so the change needs a small architectural split between session state production and mode-specific rendering.

The change must keep packet decoding and Aperture interaction unchanged. It also needs to preserve the existing console workflow while adding a localhost-only web surface that can reflect the same session progress and analysis output.

## Goals / Non-Goals

**Goals:**
- Add an explicit `--mode` flag on `thresher analyze` with `console` and `web` values.
- Keep console mode aligned with the existing Bubble Tea full-screen experience.
- Add a localhost-only HTTP server for web mode that renders live session status, counters, active model, and streamed analysis output.
- Reuse one analysis session engine so batching, limits, model switching, and request handling behave the same in both modes.

**Non-Goals:**
- Remote access, authentication, TLS termination, or multi-user sessions.
- A generic public API for third-party consumers.
- Changes to the packet wrapper format, decoded capture schema, or Aperture request/response semantics.

## Decisions

### Split session execution from presentation updates
The session engine should emit structured state updates through a presentation-agnostic interface rather than sending Bubble Tea messages directly from session logic. This keeps `Session` responsible for packet ingestion, batching, upload control, and model changes while allowing both console and web presenters to observe the same state transitions.

Alternative considered: keep Bubble Tea as the source of truth and adapt the web layer around it. This was rejected because the current UI model is terminal-specific and mixes rendering concerns with state transitions that the web server would also need.

### Keep `console` as the default analyze mode
`thresher analyze` should default `--mode` to `console` so existing users retain the current behavior without changing scripts or habits. Web mode becomes opt-in through `--mode web`.

Alternative considered: auto-detect browser-capable environments or switch defaults in non-interactive contexts. This was rejected because the user request called for explicit console or web mode selection and predictable startup behavior.

### Serve the web UI from a localhost-only in-process HTTP server
Web mode should start an HTTP server bound to `127.0.0.1` on a configurable or ephemeral local port and print the final URL to stderr/stdout so the user can open it locally. The server can serve a minimal HTML/JS UI plus a live session feed endpoint from the same process.

Alternative considered: generate static files only, or require an external web framework. This was rejected because the session is live and local, and the standard library server is enough for a single-user localhost experience.

### Publish live session state through snapshots plus incremental events
The web UI should be able to fetch the current session snapshot and subscribe to incremental updates for analysis text, status changes, counters, and model changes. A snapshot plus server-sent events style feed is a minimal fit for a local browser client and avoids adding bidirectional protocol complexity unless the implementation needs browser-to-server actions such as model switching or pause/resume commands.

Alternative considered: polling only. This was rejected because live analysis updates are a core part of the current session experience and polling adds lag and unnecessary state duplication.

### Keep interactive controls available only where they are supportable
Console mode already supports pause/resume and model selection. Web mode should expose the same controls only if the session presenter interface can safely route those commands back into the session engine. If that command path complicates the first iteration, the web UI should still render accurate read-only session state and active model information, with follow-up work adding control actions through the same shared session controller.

Alternative considered: require full parity immediately. This was rejected because read-only visibility plus live analysis already solves the primary browser-view use case, while control parity can be implemented on top of the same state/control abstraction.

## Risks / Trade-offs

- [Two presentation layers can diverge in displayed state] -> Use one shared session snapshot model and event stream for both presenters where practical, with tests around emitted state transitions.
- [Web mode lifecycle may leave a listener running after the session ends] -> Tie HTTP server shutdown to the session context and stop accepting connections when analysis completes or is canceled.
- [Live browser updates introduce concurrency bugs] -> Centralize state mutation inside the session/controller layer and expose read-only snapshots to presenters.
- [Mode-specific branching in `cmd/analyze.go` can become hard to extend] -> Keep mode selection limited to constructing the correct presenter/runtime while leaving the analysis engine unchanged.
- [Choosing SSE may make interactive browser commands asymmetric] -> Use HTTP endpoints for browser actions if needed; keep the update channel one-way for simplicity.

## Migration Plan

No data migration is required. Implementation should add the new mode flag and web presenter behind the existing `analyze` command. Rollback is straightforward: remove or disable web mode while leaving console mode untouched. Because the change does not alter packet formats or stored state, there is no compatibility migration for existing capture workflows.

## Open Questions

- Should web mode use a fixed default port or choose an ephemeral available port and print the resolved URL?
- Should the command attempt to open the browser automatically, or only print the localhost URL?
- Does the first version need browser-driven actions such as pause/resume and model switching, or is live read-only visibility sufficient for initial implementation?
