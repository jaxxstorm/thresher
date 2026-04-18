# Analyze Command Guide

`thresher analyze` sends decoded packet capture context to an Aperture-served LLM endpoint and renders ongoing analysis in an interactive session.

## Endpoint Model

`thresher` does not manage API keys or talk to model vendors directly for analysis.
You must point it at an Aperture endpoint.

Example:

```bash
go run . analyze --endpoint http://ai --model gpt-4o
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
  endpoint_style: auto
  batch_packets: 20
  batch_bytes: 65536
  session_packets: 500
  session_bytes: 2097152
  max_tokens: 300
```

Flags override config values.

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
go run . analyze --endpoint http://ai --model gpt-4o
```

Analyze a saved JSONL packet stream:

```bash
go run . analyze --endpoint http://ai --model gpt-4o --input capture.jsonl
```

## Session UI

The interactive analysis session shows:

- active model
- packet count
- status messages for batching and uploads
- live analysis output from the model
- available models when Aperture exposes `/v1/models`

Basic controls:

- `m`: cycle models when available
- `p`: pause/resume analysis state in the UI
- `q`: quit the session

## Manual Verification

1. Run `go run . analyze --endpoint http://ai --model gpt-4o`
2. Confirm the session starts without requiring local keys or auth
3. Verify packet batches are uploaded and analysis output appears incrementally
4. Verify configured session limits stop uploads before excessive volume is sent
5. If model discovery is available, verify model switching updates the active model in the UI
