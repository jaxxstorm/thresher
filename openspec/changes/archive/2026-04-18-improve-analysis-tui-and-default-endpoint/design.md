## Context

`thresher analyze` already has the core mechanics for live capture analysis: it can read records from live capture or saved JSONL, batch them, send them to Aperture-compatible endpoints, discover models when available, and push updates into a Bubble Tea program. The current implementation falls short in two places that directly affect usability: the UI is a simple text renderer that does not use the terminal as an application surface, and the command still treats the endpoint as required instead of defaulting to `http://ai`.

This change is cross-cutting because it touches command configuration, session startup, Bubble Tea program options, view layout, and tests. It must improve presentation without changing the capture packet wrapper, decoded JSONL substrate, or any wire-format details described by the packet format specification.

## Goals / Non-Goals

**Goals:**
- Make `thresher analyze` feel like a full-screen interactive session rather than a scrolling text dump
- Present status, counters, active model, limits, and analysis history in a layout that remains easy to scan during long sessions
- Refresh session information dynamically as records arrive, uploads begin or finish, models load, and analysis text is appended
- Default the analyze endpoint to `http://ai` while preserving flag-over-config precedence for explicit user choices
- Keep the existing batching, model discovery, and decoded JSONL analysis flow intact while improving how the state is rendered

**Non-Goals:**
- Changing the Tailscale packet wrapper, including `path_id`, SNAT or DNAT fields, or DISCO frame parsing
- Replacing Bubble Tea with a different UI framework
- Redesigning the upstream Aperture request flow or adding direct provider authentication
- Reworking the analysis pipeline into a general-purpose agent runtime with arbitrary tools or background jobs

## Decisions

### Full-screen mode via Bubble Tea alt screen

**Decision**: Start analyze sessions in Bubble Tea alt-screen mode and treat the UI as the primary output surface for the duration of the session.

**Rationale**: The user explicitly wants a claude-code or opencode-style experience that takes over the window and continuously refreshes. Bubble Tea already supports this interaction model cleanly, and alt-screen avoids interleaving transient UI redraws with terminal scrollback.

**Alternatives considered:**
- Keep the current inline text rendering and add more lines: rejected because it still leaves state fragmented and hard to scan
- Replace Bubble Tea entirely: rejected because the existing session already uses Bubble Tea and the requirement is a layout upgrade, not a framework migration

### Split the view into fixed summary and scrollable content regions

**Decision**: Render the UI as a composed layout with a top summary area and a larger body area. The summary area will surface high-value session facts such as endpoint, model, status, packet counters, batch state, and limits. The body area will prioritize analysis history and supporting detail such as available models or recent session events.

**Rationale**: The current single text block forces the user to hunt for changing state. Separating stable session context from long-form analysis output makes the interface legible while the session is busy.

**Alternatives considered:**
- A single scrolling log only: rejected because critical state scrolls away too quickly
- Many equally weighted panels: rejected because small terminals would become cramped and noisy

### Layout must respond to terminal resize events

**Decision**: Track terminal width and height in the model, recompute panel sizes on `tea.WindowSizeMsg`, and degrade gracefully on narrow terminals by stacking sections vertically instead of forcing a broken horizontal layout.

**Rationale**: A full-screen TUI must remain usable across laptop terminals, split panes, and resized windows. Resize-aware layout is part of the interaction model, not a cosmetic enhancement.

**Alternatives considered:**
- Hard-coded widths and heights: rejected because the UI would break in split panes or smaller terminals
- Ignore resize and rely on wrapping: rejected because wrapped content alone does not preserve hierarchy

### Visual hierarchy through semantic styling, not decoration alone

**Decision**: Use Lip Gloss styling with a limited semantic palette for active, paused, warning, and success states plus bordered panels and emphasized headings. Color will reinforce state changes, but the UI will remain understandable from text labels and structure.

**Rationale**: The request calls for a more colorful and better thought-out interface. Semantic styling improves scanability without turning the TUI into ornament that only works in one theme or terminal.

**Alternatives considered:**
- Plain monochrome sections: rejected because the current interface already demonstrates that text alone is not enough for quick scanning
- Highly saturated decorative styling everywhere: rejected because too much color reduces signal and hurts readability

### Session state model will expand to explicit dashboard fields

**Decision**: Extend the analysis model with explicit fields for window size, batch progress, upload state, last event, limit state, and analysis history rendering support instead of deriving everything from a single free-form status string.

**Rationale**: A richer UI cannot reliably infer layout-level state from one string. Structured state also makes UI tests more precise and reduces regressions when new session events are added.

**Alternatives considered:**
- Continue packing most state into `status`: rejected because it keeps rendering brittle and ambiguous
- Push all formatting into the session layer: rejected because the UI should own presentation concerns while the session layer owns session events

### Default endpoint comes from built-in config fallback

**Decision**: Set `analyze.endpoint` to `http://ai` as a viper default and remove the startup error that currently requires an explicit endpoint whenever config is empty.

**Rationale**: The user asked for `http://ai` as the default endpoint. Putting that at the config-default layer preserves the precedence order of flag > config > built-in default and avoids special-casing startup logic.

**Alternatives considered:**
- Keep endpoint required and only update docs: rejected because it does not satisfy the requested behavior
- Hard-code `http://ai` only inside `runAnalyze`: rejected because it duplicates configuration behavior and makes precedence harder to reason about

## Risks / Trade-offs

- **More UI state increases implementation complexity** -> Mitigation: keep the session event model explicit and test view output through focused state-transition tests
- **Full-screen mode can interfere with test execution or non-interactive runs** -> Mitigation: isolate view logic from program startup details and keep file-backed session tests exercising the session pipeline separately
- **Aggressive use of color can reduce readability in some terminals** -> Mitigation: use semantic color sparingly and preserve textual labels for every important state
- **Small terminals may not fit all panels comfortably** -> Mitigation: define a stacked fallback layout for narrow widths and prioritize analysis output over secondary details
- **Defaulting to `http://ai` could mask misconfiguration for some users** -> Mitigation: surface the active endpoint clearly in the UI and startup status so it is obvious where requests are going

## Migration Plan

- Update analyze config defaults so `http://ai` is always available unless the user overrides it
- Refactor the analyze model to track explicit dashboard state and window dimensions
- Rebuild the Bubble Tea view into full-screen summary and body regions with resize-aware layout
- Update analyze command, UI, and session tests for the new default endpoint and richer state rendering
- Update user-facing docs and verification guidance to reflect the fullscreen interaction model
- If rollback is needed, restore the previous inline view and endpoint validation while leaving the capture-to-analysis pipeline unchanged

## Open Questions

- Should the analysis history pane auto-scroll to the newest response by default, or should it preserve manual scroll position once the user starts navigating?
- How much recent event detail belongs in the primary dashboard versus a secondary detail pane on narrow terminals?
