# Exposing services

In Go, that looks like this:
```
http.ListenAndServe(":80", nil)
```
The above code will bind to all TCP interfaces. Since Kubernetes does support dual-stack IPv4 & IPv6 our services should bind to all interfaces (we do not currently have IPv6 services).

If you must specify an ip address to expose your service on, choosing `0.0.0.0:port` is typically a good choice. What this does can be OS and platform-specific.

Binding to `localhost:port` or `127.0.0.1:port` is binding to a local-only interface that constrained to the same "host". In Kubernetes, other containers within the Pod may still communicate with this service AND you may port-forward this container and associated service
[(Why?)](#How-can-I-port-forward-a-local-only-service?).

This may be preferred in a sidecar pattern where you do not want a container accessible outside a Pod or when you don't want expose your laptop to the cofeeshop wifi but generally, code should not be merged that binds to `localhost` or `127.0.0.1`.

### How can I port-forward a local only service?

Due to the way that kube-proxy works when you port-forward a pod or a service kube-proxy opens a tunnel to that pod (or a pod that backs that service). This can make debugging why a service is accessible in the pod but not outside of the pod hard to [understand](https://github.com/sourcegraph/zoekt/pull/46/files).

You can also use `kubectl port-forward --address 0.0.0.0` if you need to expose a port-forwarded service outside of your local machine (it defaults to `127.0.0.1`). [link](https://github.com/kubernetes/kubernetes/issues/40053) :exploding_head:

###  CI

We also have CI test for this [here](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/blob/dev/check/no-localhost-guard.sh)

### References

- https://stackoverflow.com/questions/20778771/what-is-the-difference-between-0-0-0-0-127-0-0-1-and-localhost
- https://serverfault.com/questions/21657/semantics-of-and-0-0-0-0-in-dual-stack-oses/39561#39561
- https://stackoverflow.com/questions/49067160/what-is-the-difference-in-listening-on-0-0-0-080-and-80
