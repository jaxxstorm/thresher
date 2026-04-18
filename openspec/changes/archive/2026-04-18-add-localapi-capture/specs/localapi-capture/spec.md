## ADDED Requirements

### Requirement: Capture command streams JSONL from LocalAPI
The system SHALL expose a `thresher capture` command that connects to the local Tailscale daemon through the LocalAPI debug capture stream and emits one JSON object per packet as newline-delimited JSON.

#### Scenario: Default capture output goes to stdout
- **WHEN** the user runs `thresher capture` and the LocalAPI stream is available
- **THEN** the command writes JSONL packet records to stdout until the stream ends or the command is interrupted

#### Scenario: Capture output can be redirected to a file
- **WHEN** the user runs `thresher capture -o out.jsonl`
- **THEN** the command writes the JSONL packet stream to `out.jsonl` instead of stdout

### Requirement: Capture command depends on a local tailscaled
The capture command SHALL return a contextual error when it cannot connect to a local `tailscaled` instance or cannot open the LocalAPI debug capture stream.

#### Scenario: tailscaled is unavailable
- **WHEN** the user runs `thresher capture` and no reachable local `tailscaled` instance exists
- **THEN** the command exits with a non-zero status and prints an error explaining that live capture could not be started

### Requirement: Capture output is JSONL only
The capture command SHALL emit JSONL records only and MUST NOT invoke Lua, Wireshark, or `tailscale debug capture` as an external process.

#### Scenario: Native Go capture path
- **WHEN** the user runs `thresher capture`
- **THEN** packet capture and decoding are performed entirely in Go without `os/exec`
