# Health checks

An application health check status endpoint is available at the URL path `/healthz`. It returns HTTP 200 if and only if the main frontend server and databases (PostgreSQL and Redis) are available, and also returns the version of the instance. 

The [Kubernetes cluster deployment option](../deploy/kubernetes/index.md) ships with comprehensive health checks for each Kubernetes deployment.
