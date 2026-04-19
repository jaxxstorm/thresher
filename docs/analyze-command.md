# Analyze Command Guide

`thresher analyze` sends decoded packet capture context to an Aperture-served LLM endpoint and renders ongoing analysis in either a full-screen console session or a web session that can stay local to the host or be exposed on the tailnet.

## Endpoint Model

`thresher` does not manage API keys or talk to model vendors directly for analysis.
If you do not configure or pass an endpoint explicitly, analysis defaults to `http://ai`.

Example:

```bash
go run . analyze --model gpt-4o
```

Supported Aperture-compatible request shapes:

- `/v1/chat/completions`
- `/v1/messages`
- `/v1/responses`

Use `--endpoint-style` if you need to force a specific shape.

## Config Defaults

Analysis defaults can be configured in `thresher.yaml`:

```yaml
analyze:
  endpoint: http://ai
  model: gpt-4o
  web_access: local
  endpoint_style: auto
  batch_packets: 20
  batch_bytes: 65536
  session_packets: 500
  session_bytes: 2097152
  max_tokens: 300
```

Flags override config values. The effective endpoint precedence is:

1. `--endpoint`
2. `analyze.endpoint` from config
3. built-in default `http://ai`

## Cost Controls

Use these flags to avoid runaway usage:

- `--batch-packets`
- `--batch-bytes`
- `--session-packets`
- `--session-bytes`
- `--max-tokens`

The analysis session will stop or pause uploads when configured limits are reached.

## Live And File-Based Analysis

Analyze live capture:

```bash
thresher analyze --model gpt-4o
```

Analyze a saved JSONL packet stream:

```bash
thresher analyze --model gpt-4o --input capture.jsonl
```

Start the localhost web session:

```bash
thresher. analyze --model gpt-4o --mode web
```

Start the tailnet-served web session:

```bash
thresher analyze --model gpt-4o --mode web --web-access tailnet
```

## Session UI

Console mode takes over the terminal window and keeps a live dashboard visible while analysis is running.

The full-screen UI shows:

- current endpoint, active model, and session state
- packet, byte, and batch counters
- live status for buffering, uploads, pauses, and limit states
- live analysis output from the model in a dedicated pane
- available models when Aperture exposes `/v1/models`
- recent session events and keybindings in a sidebar

Console controls:

- `m`: cycle models when available
- `p`: pause/resume analysis state in the UI
- `q`: quit the session

## Web Mode

`--mode web` starts the browser UI and prints the resolved URL.

Use `--web-access` to control how that UI is exposed:

- `local` (default): bind only to localhost
- `tailnet`: serve the same UI over tsnet so another tailnet device can open it

Remote web access requires the connecting peer to have the Tailscale capability `lbrlabs.com/cap/thresher`. The entire web UI, including the page, snapshot feed, live events, and control actions, is treated as one permission surface under that capability.

The web UI shows:

- current endpoint, active model, and session phase
- packet, byte, batch, and limit counters
- live analysis updates as new responses arrive
- recent session events
- browser controls for model selection, pause or resume, and quit

## Remote Capture Workflow

The intended remote workflow is:

1. Run `thresher analyze --mode web --web-access tailnet` on the machine closest to the target capture source.
2. Let that host perform live capture, batching, pause or resume, model changes, and Aperture requests locally.
3. Open the printed tailnet URL from another device on the same tailnet to watch the session and use the browser controls remotely.

This does not change the decoded packet substrate. Wrapper-derived fields such as `path_id`, nested `inner` traffic, and `disco_meta` still come from the same local capture and analysis flow.

## Manual Verification

1. Run `thresher analyze --model gpt-4o`
2. Confirm the session starts without requiring local keys or auth
3. Verify the full-screen UI opens and keeps session status visible while packets and analysis update
4. Verify packet batches are uploaded and analysis output appears incrementally in the analysis pane
5. Verify configured session limits stop uploads before excessive volume is sent and surface a clear limit state
6. If model discovery is available, verify model switching updates the active model in the UI
7. Run `thresher analyze --model gpt-4o --mode web` and confirm the printed URL is localhost-only by default
8. Run `thresher analyze --model gpt-4o --mode web --web-access tailnet` and confirm the printed URL is tailnet-reachable and only usable by peers with `lbrlabs.com/cap/thresher`
