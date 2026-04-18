## 1. Module and Dependencies

- [x] 1.1 Add `github.com/spf13/cobra` to `go.mod` via `go get`
- [x] 1.2 Add `github.com/spf13/viper` to `go.mod` via `go get`
- [x] 1.3 Add `github.com/jaxxstorm/log` to `go.mod` via `go get`
- [x] 1.4 Add `github.com/jaxxstorm/vers` to `go.mod` via `go get`
- [x] 1.5 Add `github.com/charmbracelet/lipgloss` to `go.mod` via `go get`
- [x] 1.6 Run `go mod tidy` and verify `go.sum` is consistent

## 2. Root Command (`cmd/root.go`)

- [x] 2.1 Create `cmd/root.go` declaring `package cmd` with a `rootCmd` `*cobra.Command`
- [x] 2.2 Set `Use`, `Short`, and `Long` on the root command
- [x] 2.3 Add persistent `--log-level` string flag (default `"info"`) to the root command
- [x] 2.4 Wire `PersistentPreRunE` to initialise `github.com/jaxxstorm/log` from the flag value
- [x] 2.5 Add `cobra.OnInitialize` stub for viper config-file loading (comment-only, no file searched yet)
- [x] 2.6 Implement `RunE` on root command: print a lipgloss-styled banner to stdout, return nil
- [x] 2.7 Export an `Execute()` function that calls `rootCmd.Execute()` and returns any error
- [x] 2.8 Write `cmd/root_test.go`: verify bare invocation exits 0 and banner is non-empty

## 3. Version Command (`cmd/version.go`)

- [x] 3.1 Create `cmd/version.go` declaring `package cmd`
- [x] 3.2 Use `github.com/jaxxstorm/vers` in the version command to calculate and print the local repository version
- [x] 3.3 Register the version command on `rootCmd` via `rootCmd.AddCommand` in an `init()` function
- [x] 3.4 Write `cmd/version_test.go`: verify `thresher version` exits 0 and output contains a version token

## 4. Entry Point (`main.go`)

- [x] 4.1 Create `main.go` at the repo root declaring `package main`
- [x] 4.2 Call `cmd.Execute()` from `main()`; on non-nil error log at error level and `os.Exit(1)`
- [x] 4.3 Verify `go build -o thresher .` produces a working binary
- [x] 4.4 Smoke-test: run `./thresher` and confirm banner appears; run `./thresher version` and confirm output; run `./thresher --help` and confirm `version` is listed
