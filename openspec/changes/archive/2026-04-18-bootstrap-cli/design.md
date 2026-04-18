## Context

`thresher` has a `go.mod` declaring the module but no source files. All subsequent features (pcap reading, Tailscale wrapper decoding, JSONL emission) require a compilable entry point and a command tree to hang them from. This change establishes that skeleton.

## Goals / Non-Goals

**Goals:**
- Compilable binary that exits cleanly
- Root command with lipgloss-styled "hello" output when run bare
- `version` subcommand via `github.com/jaxxstorm/vers`
- Persistent `--log-level` flag wired to `github.com/jaxxstorm/log`
- Viper wired to the root command for future config-file support
- All dependencies added to `go.mod`

**Non-Goals:**
- Packet decoding of any kind
- `read` or `capture` subcommands
- Config file loading (viper is wired but no config file is searched yet)
- Shell completion or man-page generation
- Cross-compilation or release tooling

## Decisions

### Project layout: flat `cmd/` with one package per command file

**Decision**: Use `cmd/root.go`, `cmd/version.go`, and `main.go` at the repo root.

**Rationale**: The project is a single-binary CLI. A flat `cmd/` package avoids unnecessary nesting. As subcommands grow each gets its own file in `cmd/`. Alternatives considered:
- `cmd/thresher/main.go` (nested) — adds path depth for no gain at this scale
- `internal/cmd/` — premature; `internal/` is for packages shared between binaries, not needed yet

### Version command: `github.com/jaxxstorm/vers`

**Decision**: Build a small cobra `version` subcommand that uses `vers.OpenRepository`, `vers.Calculate`, and `vers.GenerateFallbackVersion` to print the local repository version.

**Rationale**: `vers` is a version-calculation library, not a cobra command factory. Using it directly keeps the command small while matching the requested behaviour.

### Root command default behaviour: lipgloss banner, then exit 0

**Decision**: When `thresher` is invoked with no subcommand, print a short styled banner ("hello from thresher") and exit cleanly rather than printing usage or returning an error.

**Rationale**: The user asked for "hello world" as the default behaviour. Cobra's `RunE` on the root command handles this; `cobra.NoArgs` is set so extra arguments surface an error.

### Logging: `github.com/jaxxstorm/log`

**Decision**: Initialise the logger once in `cmd/root.go`'s `PersistentPreRunE`, using the value of `--log-level`.

**Rationale**: Keeps logging setup central; all future subcommands inherit it automatically via cobra's `PersistentPreRunE` chain.

### Viper binding

**Decision**: Bind viper to the root cobra command via `cobra.OnInitialize`. No config file is searched in this change; the hook is left as a stub comment for future expansion.

**Rationale**: Establishes the pattern early so future contributors know where config loading lives.

## Risks / Trade-offs

- **running `version` outside a git repository falls back to a development version** → use `vers.GenerateFallbackVersion()` so the command still succeeds.
- **Flat `cmd/` package grows large with many subcommands** → if the command tree becomes deep, split into sub-packages. Not a concern at this stage.
- **lipgloss adds a non-trivial dependency** → already accepted as a project convention; no mitigation needed.
