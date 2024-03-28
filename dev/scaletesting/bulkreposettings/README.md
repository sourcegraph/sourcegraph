# Bulkreposettings

A CLI tool to update an organization repositories settings in batch.

Supported operations:

- set visibility to _private_.

Supported codehosts:

- GitHub and GitHub enterprise.

## Usage

`go run ./dev/scaletesting/bulkreposettings [flags...]`

Flags:

- Authenticating:
  - `github.token`: GHE Token to create the repositories (required).
  - `github.url`: Base URL to the GHE instance (ex: `https://ghe.sgdev.org`) (required).
  - `github.org`: Existing organization to create the repositories in (required).
- Managing the workload
  - `state`: sqlite database name to create or resume from (default `state.db`)
  - `retry`: Number of times to retry pushind (can be tedious at high concurrency)
Hello World
