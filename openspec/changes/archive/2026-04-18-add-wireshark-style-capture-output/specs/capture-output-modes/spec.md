## ADDED Requirements

### Requirement: Capture output supports multiple presentation modes
The system SHALL support `jsonl`, `jsonl-compact`, and `summary` output modes for capture decoding so the same packet stream can be consumed as a full structured record, a normalized packet-row record, or a tshark-like line-oriented view.

#### Scenario: Detailed JSONL remains available
- **WHEN** the user selects `--format jsonl` or relies on the default detailed mode
- **THEN** the system emits one JSON object per packet with the full nested decode preserved

#### Scenario: Compact JSONL emits normalized packet rows
- **WHEN** the user selects `--format jsonl-compact`
- **THEN** the system emits one JSON object per packet with normalized top-level packet columns plus nested protocol detail

#### Scenario: Summary mode emits tshark-like rows
- **WHEN** the user selects `--format summary`
- **THEN** the system emits one human-readable line per packet containing the normalized packet-row columns and protocol-specific info string

### Requirement: Output modes derive from one decoded packet model
The system SHALL derive all supported output modes from the same enriched decoded packet record so packet semantics remain consistent across formats.

#### Scenario: Same packet fields drive all formats
- **WHEN** one packet is rendered in multiple output modes
- **THEN** the normalized columns, stream identifiers, and protocol-specific info are consistent across those modes except for formatting differences
