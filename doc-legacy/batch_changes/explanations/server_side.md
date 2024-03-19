# Running batch changes server-side

By default, Batch Changes uses a command line interface in your local environment to [compute diffs](how_src_executes_a_batch_spec.md) and create changesets. This can be impractical for creating batch changes affecting hundreds or thousands of repositories, with large numbers of workspaces, or if the batch change steps require CPU, memory, or disk resources that are unavailable locally.

Instead of computing Batch Changes locally using `src-cli`, you can offload this task to one or many remote server called an [executor](../../admin/executors/deploy_executors.md). Executors are also required to enable code navigation [auto-indexing](../../code_navigation/explanations/auto_indexing.md).

This allows to:

- run large-scale batch changes that would be impractical to compute locally
- speed up batch change creation time by distributing batch change computing over several executors
- reduce the setup time required to onboard new users to Batch Changes
- get a better experience: (nearly) real-time logs, integration with other Sourcegraph features (upcoming!), creating batch changes from templates in just a few clicks (also upcoming!)

## Setup

This is a one-time process. Once a site-admin of the Sourcegraph instance sets up executors and enables running batch changes server-side, all users of the Sourcegraph instance can get started with no additional setup required.

Make sure that [executors are deployed and are online](../../admin/executors/deploy_executors.md).

## Explanations

- [Getting started with running batch changes server-side](server_side_getting_started.md)
- [Debugging batch changes ran server-side](server_side_debugging.md)

## Limitations

This feature is experimental. In particular, it comes with the following limitations, that we plan to resolve before GA.

- Running batch changes server-side requires setting up executors. Executors are configured ready-to-use on Sourcegraph Cloud.
- The execution UX is work in progress and will change a lot before the GA release.
- Documentation is minimal and will change a lot before the GA release.
- Running batch changes server-side will be limited to user namespaces in this release.
- The newly introduced APIs for server-side are still experimental and will likely change as we evolve the product towards GA.
- Executors can only be deployed using Terraform (AWS or GCP) or using pre-built binaries (see [deploying executors](../../admin/executors/deploy_executors.md)).

Running batch changes server-side has been tested to run a simple 45k changeset batch change. Actual performance and setup requirements depend on the complexity of the batch change.

Feedback on running batch changes server-side is very welcome, feel free to open an [issue](https://github.com/sourcegraph/sourcegraph/issues), reach out through your usual support channel, or send a [direct message](https://twitter.com/MaloMarrec).

### Note for Apple Silicon Mac users

By default, docker on mac will build docker images for `linux/arm64`, which will result in errors when running server-side because executors provide `linux/amd64` hosts. If you're creating your own images to run in batch changes, this can be a problem. Use the `--platform linux/amd64` flag with `docker build` to build images compatible with the server-side host.

## FAQ

### Can large batch changes execution be distributed on multiple executors?

They can! Each changeset that is computed can be assigned to a separate executor, provided there are enough executors available.

### What additional resources do I need to provision to run batch changes server-side?

See [deploying executors](../../admin/executors/deploy_executors.md) page. The short answer is: as little as a single compute instance and a docker registry mirror if you just want to process batch changes at a small scale; an autoscaling group of instances if you want to process large batch changes very fast.

### Can someone accidentally take down the Sourcegraph instance if they run too big a batch change?

No. Executors have been designed for the Sourcegraph instance to offload resource-intensive tasks. The Sourcegraph instance itself only queues up batch changes for processing, tracks execution, then uses the resulting diffs to open and track changesets just as it would for batch changes created locally using the `src-cli`.

### I have several machines configured as executors, and they don't have the same specs (eg. memory). Can I submit some batch changes specifically to a given machine?

No, for now all executors are equal in the eyes of Sourcegraph. We suggest using only one type of machine.

### What happens if the execution of a step fails?

If the execution of a step on a given repository fails, that repository will be skipped, and execution on the other repositories will continue. Standard error and output will be available to the user for debugging purposes.

### How do executors interact with code hosts? Will they clone repos directly? 

Executors do not interact directly with code hosts. They behave in a way [similar to src CLI](how_src_executes_a_batch_spec.md) today: executors interact with the Sourcegraph instance, the Sourcegraph instance interacts with the code host. In particular, executors download code from the Sourcegraph instance and executors do not need to access code hosts credentials directly.
