# Local Development

The following is a guide to setting up a local development environment for the executor within a Kubernetes cluster.

## Prerequisites

- Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

### Docker Desktop

- Install Docker Desktop
- Enable Kubernetes in Docker Desktop
  - Settings > Kubernetes > Enable Kubernetes

### MiniKube

- Install [MiniKube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
- Start MiniKube
  - `minikube start`
- Load the images
  - `minikube image load executor-kubernetes:latest`
  - `minikube image load sourcegraph/batcheshelper:insiders`

### Memory

If you run `codeintel`, you may need to tinker with the `EXECUTOR_KUBERNETES_RESOURCE_REQUEST_MEMORY`
and `EXECUTOR_MAXIMUM_NUM_JOBS` to ensure that each Job has enough memory to run and that the node does not run out of
memory.

## Local Development

You can use `sg` to run Executors with either `batches` or `codeintel`.

```bash
sg start batches-kubernetes
```

```bash
sg start codeintel-kubernetes
```

Any changes to Executor code will cause `sg` to rebuild the Executor image and restart the Executor pod.

## Building Images

To run Executors in Kubernetes you will need to build the `executor-kubernetes` image. If you are running Server Side
Batch Changes, you will also need to build the `batcheshelper` image.

### Executor

Run the following command to build the executor image.

```bash
# Build the image in cmd/executor-kubernetes
IMAGE=executor-kubernetes ../../executor-kubernetes/build.sh
```

### Batches Helper

If you are running Server Side Batch Changes, you will need to build the batches helper image.

```bash
IMAGE=sourcegraph/batcheshelper:insiders ../../batcheshelper/build.sh
```

## Secrets

The frontend password should be stored in a Kubernetes secret. Run the following command to create the secret.

```bash
kubectl create secret generic executor-frontend-password --from-literal=EXECUTOR_FRONTEND_PASSWORD=hunter2hunter2hunter2
```

## Deploy

Run the following command in either the `batches` or `codeintel` directory to deploy the executor.

```bash
kubectl apply -f .
```

## Verify

An executor pod should now be created. Confirm this by running the following command.

```bash
kubectl get pods
```

You should see an executor pod in the `Running` state.

You can also check the **Site admin** Page to see the registered executor.

## Cleanup

Run the following command to delete the executor.

```bash
kubectl delete -f .
```
Hello World
