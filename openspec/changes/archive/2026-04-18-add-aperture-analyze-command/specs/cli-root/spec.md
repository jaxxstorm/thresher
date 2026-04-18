## MODIFIED Requirements

### Requirement: Root command compiles and exits cleanly
The binary SHALL compile from `go build` without errors and SHALL present registered subcommands cleanly when invoked with no arguments.

#### Scenario: Bare invocation prints help and exits 0
- **WHEN** the user runs `thresher` with no arguments
- **THEN** the command prints CLI help output including available subcommands and exits with code 0

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

### Requirement: Capture command is registered on the CLI root
The root command SHALL register a `capture` subcommand so that it appears in the CLI help output and can be invoked as `thresher capture`.

#### Scenario: Capture appears in help output
- **WHEN** the user runs `thresher --help`
- **THEN** `capture` appears in the list of available commands

### Requirement: Analyze command is registered on the CLI root
The root command SHALL register `analyze`, with `analyse` as an alias, so the analysis workflow is discoverable from CLI help output.

#### Scenario: Analyze appears in help output
- **WHEN** the user runs `thresher --help`
- **THEN** `analyze` appears in the list of available commands
- **AND** the alias `analyse` invokes the same workflow
