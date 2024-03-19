# Troubleshoot Sourcegraph with Kubernetes

If [Sourcegraph with Kubernetes](./index.md) does not start up or shows unexpected behavior, there are a variety of ways you can determine the root cause of the failure.

See our [operations guide](./operations.md) for more useful commands and operations.

## Common errors

#### Error: Error from server (Forbidden): error when creating "base/frontend/sourcegraph-frontend.Role.yaml": roles.rbac.authorization.k8s.io "sourcegraph-frontend" is forbidden: attempt to grant extra privileges.

The account you are using to apply the Kubernetes configuration doesn't have sufficient permissions to create roles, which can be resolved by creating a cluster-admin role for your user with the following command:

```bash
$ kubectl create clusterrolebinding cluster-admin-binding \
  --clusterrole cluster-admin \
  --user $YOUR_EMAIL
  --namespace $YOUR_NAMESPACE
```

#### "kubectl get pv" shows no Persistent Volumes, and/or "kubectl get events" shows a `Failed to provision volume with StorageClass "sourcegraph"` error.

Make sure a storage class named "sourcegraph" exists in your cluster within the same zone.

```bash
$ kubectl get storageclass sourcegraph -o=yaml \
  --namespace $YOUR_NAMESPACE
```

> NOTE: Google Cloud Platform users may need to [request an increase in storage quota](https://cloud.google.com/compute/quotas).


#### Error: error retrieving RESTMappings to prune: invalid resource networking.k8s.io/v1, Kind=Ingress, Namespaced=true: no matches for kind "Ingress" in version "networking.k8s.io/v1".

Run `kubectl version` to verify the __Client Version__ matches the __Server Version__.

Run `kubectl get ingresses -A` to check if there is more than one ingress for `sourcegraph-frontend`. You can delete the duplicate with `kubectl delete ingress sourcegraph-frontend --namespace $YOUR_NAMESPACE`

> NOTE: See our ["configuration guide"](configure.md#security-configure-network-access) for more information on network access.


#### Error: error when creating "base/cadvisor/cadvisor.ClusterRoleBinding.yaml": subjects[0].namespace: Required value

Add `namespace: default` to the [base/cadvisor/cadvisor.ClusterRoleBinding.yaml](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/base/cadvisor/cadvisor.ClusterRoleBinding.yaml) file under `subjects`.


#### Multiple pods are stuck in Pending.

Lack of resources could be a contributing factor. Dump current cluster state and look for error messages. Below is an example of a message that indicates the cluster is currently under provisioned.

```sh
# dump.txt
  "Reason": "FailedScheduling",
  "Message": "0/3 nodes are available: 1 Insufficient memory, 3 Insufficient cpu.",
```

> NOTE: The default node type for clusters on Google Cloud Platform is `n1-standard-1`, a machine with only one CPU, while some components require a 2-CPU node. We recommend setting machine-type to `n1-standard-16`.

#### ImagePullBackOff / 429 Too Many Requests Errors.

This indicates the instance is getting rate-limited by Docker Hub([link](https://www.docker.com/increase-rate-limits)), where our images are stored, as unauthenticated users are limited to 100 image pulls within a 6 hour period. Possible solutions included:

1. Create a Docker Hub account with a higher rate limit
2. Configure an `ImagePullSecrets` K8S object with your Docker Hub service that contains your docker credentials ([link to tutorial](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/))
3. Add these credentials to the default service account within the same namespace as your Sourcegraph deployment ([link to tutorial](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account))

Alternatively, you can wait until the rate limits are reset.

[**OPTIONAL**] You can also upgrade your account to a Docker Pro or Team subscription with higher rate-limits. ([See Docker Hub for more information](https://www.docker.com/increase-rate-limits)).


#### Irrelevant cAdvisor metrics are causing strange alerts and performance issues.

This is most likely due to cAdvisor picking up other metrics from the cluster.
A workaround is available: [Filtering cAdvisor metrics](./configure.md#filtering-cadvisor-metrics).

#### I don't see any metrics on my Grafana Dashboard.

Missing metrics indicate Sourcegraph is having issues connecting to the Kubernetes API. For instance, running a Sourcegraph instance as non-privileged prevents services from picking up metrics through the Kubernetes API. One of the potential solutions is to grant Prometheus and cAdvisor root access.


#### Which metrics are using the most resources?

1. Access the UI for Prometheus temporarily with port-forward:
    ```bash
    $ kubectl port-forward svc/prometheus 9090:30090
    ```
2. Open [http://localhost:9090/](http://localhost:9090/) in your browser
    ```bash
    $ open http://localhost:9090
    ```
3. Run `topk(10, count by (__name__)({__name__=~".+"}))` to check the values


#### You can't access Sourcegraph.

Make sure the namespace of the ingress-controller is `ingress-nginx`. See the [Troubleshooting ingress-nginx docs](https://kubernetes.github.io/ingress-nginx/troubleshooting/) for more information.

#### Healthcheck failing with Strconv.Atoi: parsing "{$portName}": invalid syntax error

This can occur when the Readiness or Liveness probe is referring to a port that is not defined. Please ensure the port name is consistent with upstream. Foe example:

```yaml
ports:
  - containerPort: 3188
    name: minio
...
livenessProbe:
  httpGet:
    path: /minio/health/live
    port: minio   #this port name MUST exist in the same spec
```

## Service mesh

Known issues when using a service mesh (e.g. Istio, Linkerd, etc.)

#### Error message: `Git command [git rev-parse HEAD] failed (stderr: ""): strconv.Atoi: parsing "": invalid syntax`

<img class="screenshot w-100" src="https://user-images.githubusercontent.com/68532117/178506378-3d047bc5-d672-487a-920f-8f228ae5cb27.png"/>

This error occurs because Envoy, the proxy used by Istio, [drops proxied trailers for the requests made over HTTP/1.1 protocol by default](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/core/v3/protocol.proto#config-core-v3-http1protocoloptions). To resolve this issue, enable trailers in your instance following the examples provided for [Kubernetes](https://github.com/sourcegraph/deploy-sourcegraph/tree/master/overlays) and [Kubernetes with Helm](https://github.com/sourcegraph/deploy-sourcegraph-helm/tree/main/charts/sourcegraph/examples).

#### Symbols sidebar and hovers are not working

<img class="screenshot w-100" src="https://user-images.githubusercontent.com/68532117/212372086-4c53e817-be3d-46b6-9cc1-fc34e695d30c.png"/>

In a service mesh like Istio, communication between services is secured using a feature called mutual Transport Layer Security (mTLS). mTLS relies on services communicating with each other using DNS names, rather than IP addresses, to identify the specific services or pods that the communication is intended for.

To illustrate this, consider the following examples of communication flows between the "frontend" component and the "symbols" component:

Example 1: Approved Communication Flow

1. Frontend sends a request to `http://symbol_pod_ip:3184`
2. The Envoy sidecar intercepts the request
3. Envoy looks up the upstream service using the DNS name "symbols"
4. Envoy forwards the request to the symbols component

Example 2: Disapproved Communication Flow

1. Frontend sends a request to `http://symbol_pod_ip:3184`
2. The Envoy sidecar intercepts the request
3. Envoy tries to look up the upstream service using the IP address `symbol_pod_ip`
4. Envoy is unable to find the upstream service because it's an IP address not a DNS name
5. Envoy will not forward the request to the symbols component

> NOTE: When using mTLS, communication between services must be made using the DNS names of the services, rather than their IP addresses. This is to ensure that the service mesh can properly identify and secure the communication.

To resolve this issue, the solution is to redeploy the frontend after specifying the service address for symbols by setting the SYMBOLS_URL environment variable in frontend. 

Please make sure the old frontend pods are removed.

```yaml
SYMBOLS_URL=http:symbols:3184
```

> WARNING: **This option is recommended only for symbols with a single replica**. Enabling this option will negatively impact the performance of the symbols service when it has multiple replicas, as it will no longer be able to distribute requests by repository/commit.

#### Squirrel.LocalCodeIntel http status 502

<img class="screenshot w-100" src="https://user-images.githubusercontent.com/68532117/212374098-dc2dfe69-4d26-4f5e-a78b-37a53c19ef22.png"/>
The issue described is related to the Code Intel hover feature, where it may get stuck in a loading state or return a 502 error with the message `Squirrel.LocalCodeIntel http status 502`. This is caused by the same issue described in [Symbols sidebar and hovers are not working](#symbols-sidebar-and-hovers-are-not-working"). See that section for solution.

## Help request

Still need additional help? Please contact us using one of the methods listed below:
- Twitter [@sourcegraph](https://twitter.com/sourcegraph)
- Email us at [support@sourcegraph.com](mailto:support@sourcegraph.com)
- File issues with our [public issue tracker](https://github.com/sourcegraph/issues/issues)
