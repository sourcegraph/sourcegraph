# Executor queue

The executor-queue maintains the executor work queues. Executor instances poll for and perform jobs from a particular queue. Executors and the executor-queue will periodically reconcile in-progress jobs via heartbeat requests.

## Work queues

- The `codeintel` queue contains unprocessed lsif_index records
- The `batches` queue contains unprocessed batch_spec_execution records
Hello World
