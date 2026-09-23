# Analyze Command Guide

`thresher analyze` sends decoded packet capture context to an Aperture-served LLM endpoint and renders ongoing analysis in either a full-screen console session or a web session that can stay local to the host or be exposed on the tailnet through the host's existing Tailscale device.

Bare `thresher analyze` runs the console workflow by default. The explicit entrypoints are:

- `thresher analyze console`
- `thresher analyze local` as an alias of `console`
- `thresher analyze web`

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

The default `auto` style selects the active model's API from `supported_endpoints` returned by `/v1/models`. For example, `thresher analyze --model claude-haiku-4-5` automatically uses `/v1/messages` when Aperture advertises it. Selection is repeated from the discovered metadata after model switches, without another discovery request. When a model supports multiple APIs, Thresher prefers chat completions, then responses, then messages. If discovery fails or no recognized endpoint is advertised for the selected model, it falls back to chat completions. An explicit `--endpoint-style` always overrides discovery.

## Aperture Session Tracking

One `thresher analyze` invocation represents one grouped Aperture analysis session, in either console or web mode. Thresher creates a canonical `session_<uuid>` identifier once at startup and reuses it for every analysis upload. Batching, pausing and resuming, and switching the active model do not rotate the identifier.

The console header and web Session panel show both the canonical **Session ID** and its **Chat fingerprint** as secondary metadata for correlation with Aperture Logs. The fingerprint is the first 8 bytes of the SHA-256 digest of the canonical ID, encoded as 16 lowercase hexadecimal characters. Web snapshots and live events expose these as `session_id` and `session_fingerprint`.

The request mapping depends on the endpoint style:

| Endpoint style | Analysis request | Tracking field | Value |
| --- | --- | --- | --- |
| `messages` | `/v1/messages` | JSON `metadata.user_id` | Canonical `session_<uuid>` ID |
| `responses` | `/v1/responses` | HTTP `Session_id` header | Canonical `session_<uuid>` ID |
| `chat-completions` | `/v1/chat/completions` | HTTP `Session_id` header | Chat fingerprint |

With `auto`, tracking uses the mapping for the selected API above. The canonical identity stays stable even if a model switch selects a different API; Aperture's grouping across API styles is server-dependent.

Model discovery through `/v1/models` is untracked setup traffic, not part of the grouped analysis conversation. Tracking only adds request metadata; it does not change packet-derived prompt fields such as `path_id`, `snat`, `dnat`, `payload_preview`, or DISCO metadata. The capture wrapper, including its 2-byte little-endian `path_id` and SNAT/DNAT handling, is unchanged.

Session identity is not persisted. Quitting and starting another invocation creates a fresh ID and fingerprint, even with the same input, endpoint, and model. Pause/resume applies only within the running process; there is no cross-process session resume.

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

Analyze live capture with the default console workflow:

```bash
thresher analyze --model gpt-4o
```

Run the explicit console subcommand:

```bash
thresher analyze console --model gpt-4o
```

Run the console alias:

```bash
thresher analyze local --model gpt-4o
```

Analyze a saved JSONL packet stream:

```bash
thresher analyze --model gpt-4o --input capture.jsonl
```

Start the localhost web session:

```bash
thresher analyze web --model gpt-4o
```

Start the tailnet-served web session:

```bash
thresher analyze web --model gpt-4o --web-access tailnet
```

## Session UI

Console mode takes over the terminal window and keeps a live dashboard visible while analysis is running.

The full-screen UI shows:

- current endpoint, active model, and session state
- canonical Session ID and Chat fingerprint for Aperture correlation
- packet, byte, and batch counters
- live status for buffering, uploads, pauses, and limit states
- live analysis output from the model in a dedicated pane
- available models when Aperture exposes `/v1/models`
- recent session events and keybindings in a sidebar

Console controls:

- `tab`: switch focus between panes
- `↑/↓`, `pgup/pgdown`, `home/end`: scroll the active pane
- `↑/k`, `↓/j`, `enter`: select and apply a model when the sidebar is focused
- `p`: pause/resume analysis state in the UI
- `q` or `esc`: quit the session

## Web Mode

`thresher analyze web` starts the browser UI and prints the resolved URL.

Use `--web-access` to control how that UI is exposed:

- `local` (default): bind only to localhost
- `tailnet`: keep Thresher bound to localhost and publish the UI through the host's existing `tailscaled` Serve configuration at `/thresher/`

Remote web access requires the connecting peer to have the Tailscale capability `lbrlabs.com/cap/thresher`. The entire web UI, including the page, snapshot feed, live events, and control actions, is treated as one permission surface under that capability.

Example tailnet policy grant:

```json
"grants": [
  {
    "src": ["group:engineering"],
    "dst": ["tag:thresher-host"],
    "ip": ["tcp:443"],
    "app": {
      "lbrlabs.com/cap/thresher": [
        {}
      ]
    }
  }
]
```

Adjust the selectors for your tailnet:

- `src`: the users, groups, or devices that should be allowed to open the remote analysis UI
- `dst`: the device or tag on the machine running `thresher analyze web --web-access tailnet`
- `ip`: the network-layer access needed for the Serve endpoint, typically `tcp:443`

The app capability is only forwarded when the requesting peer matches both the network-layer grant and the app grant. If the browser reaches the Thresher page but the app returns `missing required capability`, the usual cause is that the tailnet policy allows HTTPS access but does not grant `lbrlabs.com/cap/thresher` to that source/destination pair.

If `/thresher/` is already claimed by another Serve handler on the host, `thresher analyze web --web-access tailnet` fails fast instead of overwriting that route.

The web UI shows:

- current endpoint, active model, and session phase
- canonical Session ID and Chat fingerprint for Aperture correlation
- packet, byte, batch, and limit counters
- live analysis updates as new responses arrive
- recent session events
- browser controls for model selection, pause or resume, and quit

## Remote Capture Workflow

The intended remote workflow is:

1. Run `thresher analyze web --web-access tailnet` on the machine closest to the target capture source.
2. Let that host perform live capture, batching, pause or resume, model changes, and Aperture requests locally.
3. Open the printed `https://<machine>.<tailnet>.ts.net/thresher/` URL from another device on the same tailnet to watch the session and use the browser controls remotely.

This does not change the decoded packet substrate. Wrapper-derived fields such as `path_id`, nested `inner` traffic, and `disco_meta` still come from the same local capture and analysis flow.

## Manual Verification

1. Run `thresher analyze --model gpt-4o`
2. Confirm the session starts without requiring local keys or auth
3. Verify the full-screen UI opens and keeps session status visible while packets and analysis update
4. Verify packet batches are uploaded and analysis output appears incrementally in the analysis pane
5. Verify configured session limits stop uploads before excessive volume is sent and surface a clear limit state
6. If model discovery is available, verify model switching updates the active model in the UI
7. Run `thresher analyze web --model gpt-4o` and confirm the printed URL is localhost-only by default
8. Run `thresher analyze web --model gpt-4o --web-access tailnet` and confirm the printed URL is the host's existing tailnet identity under `/thresher/`, is tailnet-reachable, and is only usable by peers with `lbrlabs.com/cap/thresher`

### Verify Aperture Grouping

1. Start a live console or web run against an Aperture-compatible endpoint with a small batch limit, for example `thresher analyze --endpoint http://ai --endpoint-style chat-completions --model gpt-4o --batch-packets 2`. Use an approved capture source and session limits large enough for several uploads.
2. Record the displayed Session ID and Chat fingerprint. Let at least two batches complete, then open Aperture Logs at `http://ai/admin/logs` (or your endpoint's equivalent). Filter by the run's time window, model, and `thresher/<version>` user agent, and check the captured tracking fields against the table above. Confirm the uploads appear in one grouped session. Aperture's stored session key can differ from the submitted tracking value; for example, a messages request carrying `metadata.user_id = session_<uuid>` can appear under an `ancc_...` key. Use the stored key from the log entry for session API lookups, rather than assuming the displayed Thresher ID is that key.
3. Pause and resume the running analysis. If discovery provides another supported model, switch to it and allow another upload. Confirm both displayed identifiers remain unchanged and later uploads remain in the same Aperture group.
4. Inspect requests using Aperture's request view or a controlled test endpoint. Verify the tracking field matches the table above, `/v1/models` carries no session tracking, and packet context still includes the same `path_id`, `snat`, `dnat`, `payload_preview`, and DISCO details as the decoded input where applicable.
5. Quit and repeat with the same arguments. Confirm the new run has a different ID and fingerprint and appears as a separate Aperture session. Repeat for the other endpoint styles with compatible models, and verify the metadata is visible in both console and web modes.
