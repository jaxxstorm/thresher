## Purpose

Define the baseline behavior for the Thresher root CLI command.

## Requirements

### Requirement: Root command compiles and exits cleanly
The binary SHALL compile from `go build` without errors and exit with code 0 when invoked with no arguments or subcommand.

#### Scenario: Bare invocation prints banner and exits 0
- **WHEN** the user runs `thresher` with no arguments
- **THEN** a lipgloss-styled banner is printed to stdout and the process exits with code 0

#### Scenario: Unknown argument returns error
- **WHEN** the user runs `thresher` with an unrecognised argument (e.g. `thresher foobar`)
- **THEN** the process exits with a non-zero exit code and an error message is printed to stderr

### Requirement: Persistent log-level flag
The root command SHALL expose a `--log-level` flag (string, default `"info"`) that configures the `github.com/jaxxstorm/log` logger for all subcommands.

#### Scenario: Default log level is info
- **WHEN** the user runs any subcommand without `--log-level`
- **THEN** the logger is initialised at info level

#### Scenario: Log level override
- **WHEN** the user passes `--log-level debug`
- **THEN** the logger is initialised at debug level and debug messages are emitted

### Requirement: Viper integration stub
The root command SHALL initialise viper via `cobra.OnInitialize` so that future subcommands can read config values from environment variables or a config file.

#### Scenario: No config file present does not error
- **WHEN** no `thresher.yaml` or `.thresher.yaml` is present in the working directory or XDG paths
- **THEN** the binary starts without error
