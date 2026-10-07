## Why

Per-packet JSON repeats endpoints, headers, and routine traffic, making LLM uploads unnecessarily expensive and consuming context that could hold useful diagnostic evidence. Users need a smaller, offline export that summarizes observed conversations and DNS activity while retaining selected evidence and explicitly disclosing omissions.

## What Changes

- Add opt-in `thresher convert <capture.pcap> --mode summary`; retain `--mode detailed` as the existing default export.
- Produce a versioned summary document containing capture totals, canonical conversation summaries, directional packet/byte counters, TCP observations, DNS statistics, and bounded examples of analyzer findings and decode errors.
- Use relative times, reusable endpoint identifiers, and selected structured fields instead of repeated packet records or raw payloads.
- Make output deterministic and bound retained conversation detail and evidence with explicit limits and omission counters. Preserve capture-wide totals even when detail is omitted.
- Reject `--include-raw` in summary mode; retain offline operation, privacy warnings, input validation, and atomic file publication.
- Document the limits of observed traffic and heuristic analysis, and test size reduction on a representative repetitive fixture.

## Non-goals

- Model-specific tokenization, a `--max-tokens` budget, automatic upload, or LLM-generated summaries.
- A compact per-packet mode, packet filtering, arbitrary PCAP formats, or packet reassembly.
- Diagnosing root causes, proving packet loss, identifying actual client/server roles, or inferring DNS timeouts.
- Changing live capture formatting, existing analysis heuristics, DNS matching, or the Tailscale USER0 wrapper wire format.
- Guaranteeing bounded memory for the existing analyzer; this change bounds the new summary accumulator and output detail, not upstream DNS correlation state.

## Capabilities

### New Capabilities
- `capture-summary-export`: Offline aggregate JSON export with deterministic bounded detail, evidence, omission accounting, and explicit observation semantics.

### Modified Capabilities

None. Existing live output and analysis requirements remain unchanged; no existing spec defines the offline convert command.

## Impact

- `cmd/convert.go`: mode selection and validation while retaining existing file handling.
- `internal/capture`: a summary accumulator/exporter consuming existing enriched records; no new runtime dependency or external service.
- Colocated tests for aggregation, bounded evidence, errors, determinism, CLI compatibility, and output reduction.
- README and capture-output documentation covering usage, schema semantics, sensitive metadata, omissions, and tradeoffs.
