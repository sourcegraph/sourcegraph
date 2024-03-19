
# How to set a password for Redis using a ConfigMap


```
Note: This document was prepared as guidance for a customer support inquiry. It is intended to orient you to the required changes.
Please review the [Caveats](#caveats) to better understand the limitations of this guide.
```


This procedure allows a Kubernetes user to set a password for the Redis services included in the Sourcegraph deployment. 

In this approach, we store the redis.conf file for each service in a ConfigMap. The ConfigMap is then mounted as a volume into the corresponding Redis pod.

The Redis Docker image does not expose a dedicated environment variable to set a password. Due to this limitation, we must supply the configuration by supplying a custom /etc/redis/redis.conf file into the pod.

Reference Materials



* [Docs: Configure custom Redis](../deploy/kubernetes/configure.md#external-redis)
* [Docs: Using your own Redis server](../external_services/redis.md)


## Conventions



* **‘... truncated for brevity …’** indicates that a portion of the code listing has been removed to improve readability. This means that the code listing will not work by itself. The code listing is intended for you to identify the areas of your existing code that need to be modified.

* Items in **BOLD** indicate an addition to an existing code file. It should be considered safe to copy and paste the text in **BOLD **directly into your existing code.


## Caveats



* **Note:** This procedure uses a ConfigMap, which stores values in plaintext. In order to improve security, we recommend using Kubernetes Secrets. 
* **Note:** These instructions are generic and should be incorporated into your deployment tool (helm or Kustomize) as needed.
* **Note:** This example uses the password “demopasswordchangeme123”. Please change this to another value that meets your organization’s security requirements.


## Procedure



1. Locate the original redis-cache configuration file: [https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/docker-images/redis-cache/redis.conf](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/docker-images/redis-cache/redis.conf) Use this as the “Default redis.conf file” content in the ConfigMap. Adding `requirepass` sets the password for Redis authentication.
2. Create the redis-cache-conf ConfigMap:

<pre>
apiVersion: v1
data:
  redis.conf: |
    #############################
    ## Default redis.conf file           ##
    #############################
    # allow access from all instances
    protected-mode no
    # limit memory usage, discard unused keys when hitting limit
    maxmemory 6gb
    maxmemory-policy allkeys-lru
    # snapshots on disk every minute
    dir /redis-data/
    appendonly no
    save 60 1

    ###################
    ## Customization ##
    ###################
    <b>requirepass demopasswordchangeme123</b>

kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/component: redis-cache
    deploy: sourcegraph
  name: redis-cache-conf
</pre>


3. Modify the `apps_v1_deployment_redis-cache.yaml` manifest. Add the corresponding `volumeMounts:` and `volumes:`

```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-cache
spec:
… truncated for brevity … 
  template:
    metadata:
      labels:
        app: redis-cache
        deploy: sourcegraph
    spec:
      containers:
… truncated for brevity …
        volumeMounts:
        - mountPath: /redis-data
          name: redis-data
          subPathExpr: $(POD_NAME)
        - mountPath: /etc/redis/
          name: redis-cache-conf
… truncated for brevity …
      volumes:
      - configMap:
          name: redis-cache-conf
        name: redis-cache-conf

```


4. Locate the original redis-store configuration file. [https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/docker-images/redis-store/redis.conf](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/docker-images/redis-store/redis.conf) Use this as the “Default redis.conf file” content in the ConfigMap.

5. Create the redis-store-conf ConfigMap:

```
 apiVersion: v1
data:
  redis.conf: |
    #############################
    ## Default redis.conf file           ##
    #############################
    # allow access from all instances
    protected-mode no
    # limit memory usage, return error when hitting limit
    maxmemory 6gb
    maxmemory-policy noeviction
    # live commit log to disk, additionally snapshot every 5 minutes
    dir /redis-data/
    appendonly yes
    aof-use-rdb-preamble yes
    save 300 1

    ###################
    ## Customization ##
    ###################
    requirepass demopasswordchangeme123

kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/component: redis-store
    deploy: sourcegraph
  name: redis-store-conf
```


6. Modify the apps_v1_deployment_redis-store.yaml manifest. Add the corresponding volumeMounts: and volumes: \


<pre>
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-store
… truncated for brevity … 
  template:
    metadata:
      labels:
        app: redis-store
        deploy: sourcegraph
    spec:
      containers:
… truncated for brevity … 
<b>
        volumeMounts:
        - mountPath: /redis-data
          name: redis-data
          subPathExpr: $(POD_NAME)
        - mountPath: /etc/redis/
          name: redis-store-conf
… truncated for brevity …
      volumes:
      - configMap:
          name: redis-store-conf
        name: redis-store-conf
</b>
</pre>


7. Modify the manifests for all services listed in[ Configure custom Redis](https://docs.sourcegraph.com/admin/install/kubernetes/configure#external-redis). The listing below is an example of the two environment variables that must be added to the services listed in the documentation.


<pre>
apiVersion: apps/v1
kind: Deployment
… truncated for brevity …
    spec:
      containers:
… truncated for brevity …
<b>
        env:
        - name: REDIS_CACHE_ENDPOINT
          value: redis://:demopasswordchangeme123@redis-cache:6379
        - name: REDIS_STORE_ENDPOINT
          value: redis://:demopasswordchangeme123@redis-store:6379
</b>
</pre>



**Note:** Be sure to add both environment variables to all services listed in [Configure custom Redis](https://docs.sourcegraph.com/admin/install/kubernetes/configure#external-redis).



8. After making the necessary changes to the manifests, apply the changes. Kubernetes should recreate all pods that were modified.
9. Verify the sourcegraph-frontend pods are running and do not contain log messages that indicate that sourcegraph-frontend is unable to the redis services.
10. All services should be running and passing health checks. If there are any services that are in a CrashLoop or not running, ensure that the necessary environment variables are correct, captured in the Kubernetes manifests, and applied to your installation.
