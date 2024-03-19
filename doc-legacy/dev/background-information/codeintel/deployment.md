# Deploying code navigation services

Most of the code navigation logic lives inside of the Sourcegraph instance and are deployed via [docker](https://github.com/sourcegraph/deploy-sourcegraph-docker), [docker-compose](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/master/docker-compose), or [Kubernetes configuration](https://github.com/sourcegraph/deploy-sourcegraph).

The [executor](https://github.com/sourcegraph/sourcegraph/tree/main/cmd/executor) service, which runs user-supplied code to produce and upload code graph indexes, is deployed directly onto compute nodes in its own GCP project. This services requires certain Linux kernel extensions to operate, which are not available within a Kubernetes cluster. The [deployment](https://github.com/sourcegraph/infrastructure/tree/main/executors) for this service is managed through Terraform.
