## ADDED Requirements

### Requirement: Version subcommand prints the local repository version
The binary SHALL expose a `version` subcommand that uses `github.com/jaxxstorm/vers` to calculate and print the local repository version.

#### Scenario: Version output in a git repository
- **WHEN** the user runs `thresher version` from a git repository
- **THEN** the command prints the local repository version and exits 0

#### Scenario: Version output outside a git repository
- **WHEN** the user runs `thresher version` from outside a git repository or version calculation fails
- **THEN** the command prints a fallback development version and exits 0

### Requirement: Version command registered on root
The `version` subcommand SHALL be registered as a child of the root cobra command so that `thresher help` lists it.

#### Scenario: Version appears in help output
- **WHEN** the user runs `thresher --help`
- **THEN** `version` appears in the list of available commands
