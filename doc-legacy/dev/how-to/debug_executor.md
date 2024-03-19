# Debugging executors

This documents how to debug firecracker executors using GCP VMs since that doesn't work on Mac.

First, create a VM in GCP that allows nested virtualization: 

```bash
gcloud compute instances create \
  executors-test \
  --enable-nested-virtualization \
  --zone=us-central1-a \
  --min-cpu-platform="Intel Haswell" \
  --project <YOUR_PROJECT> \
  --boot-disk-size=50GB
```

Then, connect to the instance via ssh: 

```bash
gcloud compute ssh \
  --zone "us-central1-a" \
  --tunnel-through-iap \
  --project <YOUR_PROJECT> \
  executors-test
```

Configure go to cross-compile a binary for linux amd64:

```bash
export GO111MODULE=on
export GOARCH=amd64
export GOOS=linux
export CGO_ENABLED=0
```

Build a go binary of executor for linux: 

```bash
cd cmd/executor \
  && go build \
    -trimpath \
    -buildmode exe \
    -tags dist \
    -o executor github.com/sourcegraph/sourcegraph/cmd/executor
```

Copy the binary onto our new VM:

```bash
gcloud compute scp 
  --tunnel-through-iap \
  --project <YOUR_PROJECT> \
  --zone us-central1-a \
  executor <YOUR_USER>@executors-test:~/executor
```

There is some more general info on how to set up a generic VM with the executor binary in linux, 
using firecracker [here](https://docs.sourcegraph.com/admin/executors/deploy_executors_binary),
but it can be summarized as:
- Install docker, git, and binutils
- Set env vars `EXECUTOR_FRONTEND_URL`, `EXECUTOR_FRONTEND_PASSWORD`, `EXECUTOR_QUEUE_NAME`, `EXECUTOR_USE_FIRECRACKER`
- Run `executor install all` (until here, everything only needs to happen once, unless you change the setup)
- Run executor with `executor run`

At this point, if you go to the sourcegraph instance that it’s connected to,
you should see it appear in the executors site admin page, and also see it
successfully pick up jobs with firecracker VMs enabled. 

Pro tip: `executor test-vm` spins up a long lived firecracker VM for you the same way (networking,
disk, etc) it would during the job so you can inspect it, ssh into it and try
things out. Warning: they get pruned when the actual executor is running, as it
finds them as oprhaned VMs. 

Finally, to clean up after yourself when done: 

```bash
gcloud compute instances delete \
  --zone=us-central1-a \
  --project sourcegraph-dogfood \
  executors-test
```
