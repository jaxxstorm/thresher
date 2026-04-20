## Why

`thresher analyze` currently mixes console and web behavior behind `--mode` and `--web-access` flags on a single command. That makes the CLI harder to discover, leaves mode-specific flags visible in the wrong context, and forces the user to reason about presentation mode as an option instead of as an explicit workflow.

## What Changes

- Replace the `--mode` selector with explicit analyze subcommands:
  - `thresher analyze console`
  - `thresher analyze local` as an alias of `console`
  - `thresher analyze web`
- Make bare `thresher analyze` invoke the console workflow so the current default behavior remains intact.
- Move mode-specific flags onto the subcommands that actually use them, so console-only and web-only options are no longer exposed together on the root analyze command.
- Keep shared analysis flags such as endpoint, model, batching, limits, and input handling available where they apply, without changing packet decoding, wrapper fields, or Aperture request semantics.
- **BREAKING**: remove `--mode` from the analyze command surface in favor of explicit subcommands.

## Non-goals

- Changing the packet wrapper format, decoded JSONL schema, or any wire-level fields such as `path_id`, `inner`, `snat`, `dnat`, or `disco_meta`.
- Replacing the existing console or web analysis implementations.
- Redesigning analyze batching, model discovery, or tailnet capability behavior beyond moving those controls onto the appropriate commands.

## Capabilities

### New Capabilities

### Modified Capabilities
- `aperture-analysis`: Change the analyze command contract from a `--mode`-based selector to explicit `console` and `web` subcommands while preserving bare `analyze` as the console entrypoint.
- `analysis-web-ui`: Change the web-mode entry contract so the browser workflow is started through `thresher analyze web` instead of `thresher analyze --mode web`.

## Impact

- Affected code will include `cmd/analyze.go`, command tests, and any analyze help or documentation that references `--mode` or mode-specific flags.
- The CLI surface changes for users and scripts, so docs and help text need to clearly describe the new subcommands and the retained default behavior for bare `thresher analyze`.
- No new dependencies or packet-format changes are required.
