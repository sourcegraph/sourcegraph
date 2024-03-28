# Bundled Executor

The bundled executor is a simple executor that runs commands on the container instead of delegating to spawned
containers.

The runtime of this executor is `shell`.

## Configuration

Since the bundled executor does not spawn Docker Containers to run Jobs, `src` cannot be used for Batch Changes. To run
Batch Changes, Native Execution must be enabled.

### Native Execution

See [Documentation](../../../doc/admin/executors/native_execution.md) on how to enable Native Execution.

## Clean Workspace

To ensure a clean workspace, the executor will exit with code `0` once a Job is complete. It depends on the Platform to
create a new instance when it exits.

## Available Commands

- `batcheshelper`
- `xmlstarlet`
- Python 3
- `pip`
- Java 11 (OpenJDK)
- Maven 3.6.3
- `yq`
Hello World
