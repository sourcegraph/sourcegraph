# Opting out of batch changes

Repository owners that are not interested in batch change changesets can opt out so that their repository will be skipped when a batch spec is executed.

To opt out: create a file called `.batchignore` at the root of the repository you wish to be skipped. `src batch [apply|preview]` will now skip that repository if it's yielded by the `on` part of the batch spec.

>NOTE: You can use the `-force-override-ignore` flag to override that behaviour and not skip any ignored repositories.
