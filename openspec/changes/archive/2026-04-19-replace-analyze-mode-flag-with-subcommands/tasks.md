## 1. Analyze Command Restructure

- [x] 1.1 Refactor `cmd/analyze.go` so `thresher analyze` becomes a command group with explicit `console` and `web` subcommands, `local` as an alias of `console`, and bare `thresher analyze` invoking the same console workflow by default without changing request payload content built from decoded packet fields such as `path_id`, `snat`, `dnat`, `inner`, and `disco_meta`
- [x] 1.2 Move mode-specific flags to the relevant subcommands in `cmd/analyze.go`, removing `--mode` from the supported command surface while keeping shared analysis flags available to both subcommands

## 2. Command-Level Verification

- [x] 2.1 Update `cmd/analyze_test.go` to cover bare `analyze`, `analyze console`, `analyze local`, and `analyze web`, verifying that `console` and `local` share the same workflow and that web-specific startup remains isolated to the `web` subcommand
- [x] 2.2 Add or update tests in `cmd/analyze_test.go` to verify web-only flags such as tailnet web access are accepted under `analyze web` and rejected from the console path, while shared analysis prompts still preserve wrapper-derived fields such as `path_id`, `snat`, `dnat`, and payload preview text

## 3. Documentation And Help

- [x] 3.1 Update `docs/analyze-command.md` and any related help text to replace `--mode` examples with `thresher analyze console`, `thresher analyze local`, and `thresher analyze web`, and document that bare `thresher analyze` runs the console workflow by default
- [x] 3.2 Run `go test ./cmd ./internal/analyze` and perform a manual smoke test of `go run . analyze`, `go run . analyze console`, and `go run . analyze web` to confirm the new command shape works without changing decoded packet semantics or wrapper-derived fields such as `path_id`, `inner`, and `disco_meta`
