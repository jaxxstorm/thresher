## Why

The current capture plan still assumes a file-based workflow or external process handoff, but the desired user experience is a native `thresher capture` command that talks directly to a running `tailscaled`, parses packets in Go, and streams JSONL to stdout. This is needed now because capture is the cleanest end-to-end path for validating the Go parser without introducing Lua, Wireshark, or `os/exec` dependencies.

## What Changes

- Add a native `capture` command that connects to the Tailscale LocalAPI and reads the debug capture stream without spawning `tailscale debug capture`
- Parse the Tailscale debug wrapper and DISCO payloads in Go and emit one JSON object per packet to stdout
- Add an optional `-o` / `--output` flag that writes the JSONL stream to a file instead of stdout
- Decode non-DISCO packets into inner IP/TCP/UDP metadata using Go packet parsing
- Define runtime behavior when `tailscaled` is unavailable or the LocalAPI stream fails

## Non-goals

- No Lua generation or Wireshark integration
- No shelling out to `tailscale debug capture`
- No pcap file ingestion changes in this change unless required to share parser code with `capture`
- No rich terminal table or pretty output for capture; JSONL is the only output format in scope

## Capabilities

### New Capabilities

- `localapi-capture`: Connect to a local `tailscaled` instance, stream debug capture data, decode packets in Go, and emit JSONL to stdout or a file
- `go-packet-parser`: Parse the custom Tailscale capture wrapper, DISCO metadata, DISCO frame types, and inner IP/TCP/UDP packet metadata entirely in Go

### Modified Capabilities

- `cli-root`: Register a new `capture` subcommand in the CLI

## Impact

- **CLI**: new `thresher capture` command and output flag
- **Parser code**: new Go-native decoder for wrapper bytes, DISCO metadata, and inner packet decoding
- **Dependencies**: likely direct use of the Tailscale client/local API package plus existing or new packet-decoding libraries
- **Runtime**: command depends on a reachable local `tailscaled` instance for live capture
