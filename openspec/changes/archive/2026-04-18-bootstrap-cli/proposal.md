## Why

The `thresher` project has no runnable binary yet. Before any packet-decoding logic can be built, the CLI skeleton must exist: a root command, command registration wiring, and a `version` subcommand. Without this foundation none of the planned `read` or `capture` commands can be added.

## What Changes

- Introduce `main.go` entry point under `cmd/thresher/`
- Introduce root cobra command with cobra/viper wiring and persistent flags (log level)
- Introduce `version` subcommand backed by `github.com/jaxxstorm/vers` to print the local repository version
- Root command prints a short "hello world" banner via lipgloss when run with no subcommand
- Add all required Go module dependencies (`cobra`, `viper`, `jaxxstorm/log`, `jaxxstorm/vers`, `lipgloss`)

## Non-goals

- No packet reading or decoding in this change
- No `read` or `capture` subcommands
- No config file loading beyond wiring viper to the root command
- No shell completion or man-page generation

## Capabilities

### New Capabilities

- `cli-root`: Root cobra command, viper integration, persistent `--log-level` flag, and lipgloss-styled default output when invoked with no subcommand
- `cli-version`: `version` subcommand that prints the local repository version via `github.com/jaxxstorm/vers`

### Modified Capabilities

_(none — no existing specs)_

## Impact

- **New files**: `main.go`, `cmd/root.go`, `cmd/version.go`
- **Go module**: `go.mod` / `go.sum` will gain direct dependencies on `cobra`, `viper`, `jaxxstorm/log`, `jaxxstorm/vers`, `charmbracelet/lipgloss`
- **No existing code is modified** — this is a greenfield bootstrap
