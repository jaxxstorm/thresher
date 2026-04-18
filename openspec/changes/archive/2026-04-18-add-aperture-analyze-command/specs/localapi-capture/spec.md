## MODIFIED Requirements

### Requirement: Capture command streams JSONL from LocalAPI
The system SHALL expose a `thresher capture` command that connects to the local Tailscale daemon through the LocalAPI debug capture stream and emits packet records in the selected output format. The default output SHALL remain detailed JSONL.

#### Scenario: Default capture output goes to stdout
- **WHEN** the user runs `thresher capture` and the LocalAPI stream is available
- **THEN** the command writes detailed JSONL packet records to stdout until the stream ends or the command is interrupted

#### Scenario: Capture output can be redirected to a file
- **WHEN** the user runs `thresher capture -o out.jsonl`
- **THEN** the command writes the selected packet output stream to `out.jsonl` instead of stdout

#### Scenario: Capture output can feed analysis sessions
- **WHEN** the user starts an analysis session from live capture rather than from a saved file
- **THEN** the system reuses the same decoded packet stream produced from the LocalAPI capture workflow as the substrate for batching and analysis

### Requirement: Capture command depends on a local tailscaled
The capture command SHALL return a contextual error when it cannot connect to a local `tailscaled` instance or cannot open the LocalAPI debug capture stream.

#### Scenario: tailscaled is unavailable
- **WHEN** the user runs `thresher capture` and no reachable local `tailscaled` instance exists
- **THEN** the command exits with a non-zero status and prints an error explaining that live capture could not be started

### Requirement: Capture output is JSONL only
The capture command SHALL emit packet output from a native Go decoding path and MUST NOT invoke Lua, Wireshark, or `tailscale debug capture` as an external process.

#### Scenario: Native Go capture path
- **WHEN** the user runs `thresher capture`
- **THEN** packet capture and decoding are performed entirely in Go without `os/exec`

### Requirement: Capture command exposes format selection
The capture command SHALL expose a `--format` flag that selects between detailed JSONL, compact JSONL, and summary-oriented row output.

#### Scenario: Compact JSONL is selected
- **WHEN** the user runs `thresher capture --format jsonl-compact`
- **THEN** the command emits normalized top-level packet-row fields plus nested protocol detail for each packet

#### Scenario: Summary output is selected
- **WHEN** the user runs `thresher capture --format summary`
- **THEN** the command emits one tshark-like packet row per packet instead of JSON objects
