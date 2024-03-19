# How to troubleshoot pod evictions in Sourcegraph Kubernetes deployments
This document will take you through how to solve for pod eviction that can cause data loss in ephemeral storage.

This document will take you step-by-step through the tasks required to perform troubleshooting to understand why this occurrence took place and eventually solve for it.

## Prerequisites
This document assumes that you have deployed Sourcegraph on Kubernetes and you are a site admin for your organization.
## Steps to troubleshoot
1. Run `kubectl describe pod $EVICTEDPOD `
2. Check the `Message` object
3. If the error is: `Pod ephemeral local storage usage exceeds the total limit of containers xGi.`
4. Check on the:
`ephemeral-storage` Limits and Requests, for example `ephemeral-storage: xGi`. Also, check the cache size for the pod where`$PODNAME_CACHE_SIZE_MB>:x0000`, (x is an integer).
5. In the `$PODNAME.Deployment.yaml`, raise the `ephemeral-storage` figures to a preferred storage size for your node and set the `CACHE_SIZE_MB` to a size lower than the ephemeral storage limit.
6. Enable auto scaling by increasing the number of replicas(if preferred)

## Further resources

- [Sourcegraph - Alert solutions](https://docs.sourcegraph.com/admin/observability/alerts)
- [Kubernetes Eviction docs](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/)
