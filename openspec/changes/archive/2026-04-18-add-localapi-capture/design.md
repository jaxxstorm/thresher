## Context

`thresher` already has a minimal CLI skeleton, but it does not yet implement live capture or any packet parsing. The target behavior for this change is a native `capture` command that connects to a currently running `tailscaled`, consumes the LocalAPI debug capture stream, decodes the Tailscale wrapper and nested packet metadata in Go, and emits JSONL without invoking Lua, Wireshark, or external binaries.

The Tailscale reference implementation obtains a stream by calling `localClient.StreamDebugCapture(ctx)`. The requested design is to follow that LocalAPI pattern directly rather than spawning `tailscale debug capture` with `os/exec`.

## Goals / Non-Goals

**Goals:**
- Add a `thresher capture` command that connects directly to the Tailscale LocalAPI
- Stream decoded packet records as JSONL to stdout by default
- Support `-o` / `--output` to write JSONL to a file instead of stdout
- Parse the custom debug wrapper, DISCO metadata, DISCO frames, and inner IP/TCP/UDP headers entirely in Go
- Share parser code cleanly enough that live capture and future file-based decoding can reuse the same decoder path
- Surface clear user-facing errors when `tailscaled` or LocalAPI access is unavailable

**Non-Goals:**
- Reintroducing Lua or Wireshark integration
- Spawning `tailscale debug capture` or any other helper process
- Adding capture-time filtering, pretty/table output, or packet aggregation
- Implementing configuration-based capture parsing behavior
- Supporting remote `tailscaled` instances in this change

## Decisions

### LocalAPI integration: use the Tailscale client stream directly

**Decision**: Use Tailscale's Go client to connect to the local daemon and call the LocalAPI capture stream directly.

**Rationale**: This matches the desired behavior and avoids shelling out. It keeps capture as a stream of bytes from a real `tailscaled` instance and preserves the same source of truth as `tailscale debug capture` without depending on CLI process management.

**Alternatives considered:**
- `os/exec` around `tailscale debug capture` — rejected because process management, CLI dependency, and extra failure modes are unnecessary
- Local pcap files only — rejected because the goal is live capture from the daemon

### Decoder structure: separate stream ingestion from packet decoding

**Decision**: Build a decoder package that accepts packet payload bytes plus per-packet metadata and returns a structured packet result. The capture command is responsible for reading the stream and emitting JSONL; the decoder is responsible for byte parsing.

**Rationale**: This keeps the parser testable with byte fixtures and reusable for later `read` support. It also limits command code to orchestration and output behavior.

### Packet parsing: decode the wrapper manually, then delegate inner IP parsing to Go packet libraries

**Decision**: Parse the wrapper and DISCO formats manually from byte slices using the documented field offsets, then use `gopacket` / protocol layers for the inner IPv4/IPv6 + TCP/UDP decode path.

**Rationale**: The wrapper and DISCO layout are custom, so manual parsing is the simplest and most explicit approach. Inner IP parsing is standard and benefits from existing libraries instead of bespoke parsing.

**Alternatives considered:**
- Full custom decoding for all inner packets — rejected because it adds avoidable protocol work
- Treating the entire payload as opaque bytes — rejected because the value of the tool is structured JSONL

### Output model: JSONL only for live capture

**Decision**: Emit one JSON object per captured packet to an `io.Writer`, defaulting to stdout. When `-o` is set, open the file and write the same JSONL stream there.

**Rationale**: The user explicitly wants a streaming JSONL artifact. Keeping a single output path avoids duplicated formatting logic and keeps the command deterministic.

### Failure handling: fail fast with contextual errors

**Decision**: If LocalAPI connection, stream setup, packet decode, or output write fails, return a contextual error from the command. Per-packet malformed data should either produce a best-effort JSON object with an error field or be logged and skipped, depending on where corruption is detected.

**Rationale**: Connection and stream setup failures are fatal. Per-packet corruption is more nuanced; preserving stream continuity is valuable, but the JSON contract should not silently lie about decode success.

## Risks / Trade-offs

- **LocalAPI types and capture framing may differ from assumptions** → Mitigation: inspect the actual Tailscale client API during implementation and adjust the design artifacts if the stream framing differs from pcap-style expectations.
- **Live capture is harder to test deterministically** → Mitigation: keep byte parsing in pure functions with fixture-driven tests; limit integration tests to command wiring.
- **Malformed packets may interrupt long-running capture** → Mitigation: distinguish fatal stream errors from per-packet decode errors and handle them separately.
- **Adding direct Tailscale client dependencies increases module surface** → Mitigation: keep LocalAPI usage isolated behind a small internal package.

## Migration Plan

- Add the capture command and parser behind normal CLI registration
- Verify with `go test ./...`
- Verify manually with `go run . capture` against a running local `tailscaled`
- No data migration or rollback plan is required; rollback is removing the command and parser code

## Open Questions

- What exact framing does the LocalAPI stream expose to the Go client: raw pcap bytes, packet records with metadata, or another envelope?
- Should per-packet decode errors be emitted in JSONL as structured error records or only logged?
- Is a reusable LocalAPI client helper warranted now, or should the first implementation live directly under the capture command until a second caller exists?
