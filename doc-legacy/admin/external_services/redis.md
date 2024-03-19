# Using your own Redis server

**Version requirements**: We support any version *starting from 6.2*. Redis Cluster and Redis Sentinel are not supported.

Generally, there is no reason to do this as Sourcegraph only stores ephemeral cache and session data in Redis. However, if you want to use an external Redis server with Sourcegraph, you can follow the deployment specific guidance below:

## Single-container
Add the `REDIS_ENDPOINT` environment variable to your `docker run` command and Sourcegraph will use that Redis server instead of its built-in one. The string must either have the format `$HOST:PORT`
or follow the [IANA specification for Redis URLs](https://www.iana.org/assignments/uri-schemes/prov/redis) (e.g., `redis://:mypassword@host:6379/2`). For example:

<!--
  DO NOT CHANGE THIS TO A CODEBLOCK.
  We want line breaks for readability, but backslashes to escape them do not work cross-platform.
  This uses line breaks that are rendered but not copy-pasted to the clipboard.
-->
<pre class="pre-wrap"><code>docker run [...]<span class="virtual-br"></span>   -e REDIS_ENDPOINT=redis.mycompany.org:6379<span class="virtual-br"></span>   sourcegraph/server:5.2.5</code></pre>

> NOTE: On Mac/Windows, if trying to connect to a Redis server on the same host machine, remember that Sourcegraph is running inside a Docker container inside of the Docker virtual machine. You may need to specify your actual machine IP address and not `localhost` or `127.0.0.1` as that refers to the Docker VM itself.

If using Docker for Desktop, `host.docker.internal` will resolve to the host IP address.

## Kubernetes

### Kubernetes with Helm
[See the Helm Redis guidance here](../deploy/kubernetes/helm.md#using-external-redis-instances)

### Kubernetes without Helm
- See our documentation for Kubernetes [here](../deploy/kubernetes/configure.md#external-redis)
 - **Related:** [How to Set a Password for Redis using a ConfigMap](../how-to/redis_configmap.md)
