# Sourcegraph monitoring generator

This page documents usage (running the generator) and development (of the generator itself).
For background and feature documentation, see [the generator overview](https://sourcegraph.com/docs/dev/background-information/observability/monitoring-generator).
To learn about how to find, add, and use monitoring, see the [Sourcegraph observability developer guide](https://sourcegraph.com/docs/dev/background-information/observability).

## Usage

From this directory:

```sh
go generate ./...
```

Logging output supports the [Sourcegraph log level flags](https://sourcegraph.com/docs/admin/observability/logs).
Other configuration options can be customized via flags declared in [`main.go`](./main.go).

## Development

The Sourcegraph monitoring generator consists of three components:

- The [main program](./main.go) - this is the primary entrypoint to the generator.
- _Definitions_, defined in the top-level [`monitoring/definitions` package](./definitions/).
  This is where the all service monitoring definitions lives.
  If you are editing monitoring, this is probably where you want to look - see the [Sourcegraph observability developer guide](https://sourcegraph.com/docs/dev/background-information/observability).
- _Generator_, defined in the nested [`monitoring/monitoring` package](./monitoring/README.md) package.
  This is where the API for service monitoring definitions is defined, as well as the generator code that provides [its features](https://sourcegraph.com/docs/dev/background-information/observability/monitoring-generator#features).

All features and capabilities for developed for the generator should align with the [Sourcegraph monitoring pillars](https://handbook.sourcegraph.com/engineering/observability/monitoring_pillars).
Hello World
