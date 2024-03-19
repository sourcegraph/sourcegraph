# How to determine why the precise-code-intel-worker is in CrashLoopBackOff status in Kubernetes deployment

This document will discuss one of the main reasons why the `precise-code-intel-worker` goes into a `CrashLoopBackOff` state in a Kubernetes. It does not attempt to solve *all* reasons why this state can happen.

This commonly happens when upgrading from a Sourcegraph version prior to 3.22 to a later version and failing to deploy the MinIO container. This is because in [3.21 -> 3.22 we removed the `code intel bundle manager` and replaced it with `MinIO`.](https://docs.sourcegraph.com/admin/updates/kubernetes#3-21-3-22)


## Symptoms

When running `kubectl get pods` you notice that the `precise-code-intel-worker` is in this state, example:

```bash
precise-code-intel-worker-9b69b5b59-jl6vd   0/1     CrashLoopBackOff   416        2d5h   
precise-code-intel-worker-9b69b5b59-z7xx4   0/1     CrashLoopBackOff   415        2d5h   
```

## Steps to resolve

1. Check what version of Sourcegraph you are on. If it is 3.22 or later, you will need to deploy `MinIO` because the `precise-code-intel-worker` depends on MinIO to function. If 3.4.2+, then minio is no longer used and instead `sourcegraph/blobstore` is used.

2. [Check what pods you have deployed and make sure MinIO is in the list.](../deploy/kubernetes/operations.md#list-pods-in-cluster)

	`kubectl get pods -o wide`

3. If MinIO is not deployed, create a fork of the [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph) repository and make sure you deploy MinIO (or blobstore in 3.4.2+).



## Further resources

* [Sourcegraph - Kubernetes Configuration](../deploy/kubernetes/configure.md)
* [Deploy Sourcegraph - blobstore](https://github.com/sourcegraph/deploy-sourcegraph/tree/master/base/blobstore)
