## Context

`thresher analyze` currently uses a single Cobra command with `--mode` and `--web-access` flags to select between console and web behavior. That keeps all analyze flags in one place, but it also makes the CLI harder to scan because web-specific controls appear alongside console usage and the primary workflows are hidden behind flag combinations instead of being represented as commands.

The requested change is to make the workflows explicit:
- `thresher analyze console`
- `thresher analyze local` as an alias of `console`
- `thresher analyze web`

At the same time, bare `thresher analyze` must continue to launch the console workflow so the current default behavior remains intact. The change needs to preserve the existing analysis engine, packet batching, model discovery, tailnet web access support, and packet-wrapper semantics.

## Goals / Non-Goals

**Goals:**
- Replace `--mode` with explicit `console` and `web` subcommands.
- Keep `thresher analyze` as a valid shorthand for the console workflow.
- Move mode-specific flags onto the subcommand that actually uses them.
- Preserve shared analysis flags and behavior for endpoint selection, model choice, input files, batching, limits, and packet-derived prompts.

**Non-Goals:**
- Rewriting the console or web presenters.
- Changing packet decoding, wrapper fields, or Aperture request bodies.
- Changing the semantics of tailnet access, pause or resume, model selection, or versioned `User-Agent` handling.
- Introducing a separate implementation path for `local`; it is only an alias of `console`.

## Decisions

### Model analyze as a command group with a console default action
`analyze` should become a parent command with subcommands for `console` and `web`, but the parent command should still execute the console flow when invoked directly with no subcommand. This keeps the user-visible workflows explicit without breaking the current habit of running `thresher analyze`.

Alternative considered: make `analyze` a pure container command that requires a subcommand. This was rejected because the user explicitly wants bare `thresher analyze` to remain the console entrypoint.

### Split mode-specific flags by subcommand and keep shared flags common
Shared analysis flags such as `--endpoint`, `--model`, `--input`, batching limits, token limits, and endpoint style should stay available to both subcommands. Web-only controls such as `--web-access` should move onto `analyze web`, while console-only controls should live on `analyze console` if any are added later.

This preserves existing shared behavior while cleaning up help output and preventing web-only flags from appearing in console usage.

Alternative considered: keep all flags on the parent command and only change invocation syntax. This was rejected because it would not actually solve the discoverability and help-surface problems that motivated the change.

### Make `local` an alias of the console subcommand, not a separate workflow
`analyze local` should resolve to the exact same command implementation as `analyze console`. This keeps the command surface simple and avoids introducing a third branch of behavior.

Alternative considered: treat `local` as a different execution mode. This was rejected because the request frames it as an alias and there is no separate semantic difference to preserve.

### Remove `--mode` rather than silently supporting both shapes long-term
The command surface should migrate to explicit subcommands and stop advertising `--mode`. Tests and docs should shift to the new syntax so future changes treat the subcommands as canonical.

Alternative considered: leave `--mode` as a hidden compatibility flag indefinitely. This was rejected because it preserves two overlapping entry contracts and makes future help and documentation more ambiguous.

## Risks / Trade-offs

- [Users with scripts that pass `--mode` will break] -> Keep bare `thresher analyze` working for console and update docs/help text to make the new syntax obvious.
- [Flag registration can become inconsistent across the parent and subcommands] -> Centralize shared flag wiring and apply it to both subcommands from one helper.
- [Changing Cobra command structure can affect tests and aliases in subtle ways] -> Add command-level tests for direct `analyze`, `analyze console`, `analyze local`, and `analyze web`.
- [Web-only flags may accidentally remain reachable from the wrong command] -> Register them only on the web subcommand and explicitly test help/error behavior.

## Migration Plan

Implementation should first restructure the Cobra command tree so `analyze` has explicit `console` and `web` subcommands while preserving direct `analyze` execution as console. Then move flag registration to shared and mode-specific helpers, update tests and docs, and remove references to `--mode` from the supported interface. Rollback is straightforward: restore the single-command shape with `--mode`.

## Open Questions

- Should `--mode` be rejected immediately with an invalid-flag error, or kept temporarily as a hidden compatibility shim for one release?
