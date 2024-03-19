# Site Administration Quickstart 
Administrating and managing a Sourcegraph instance is handled by Site Admins. These admins are typically responsible for deploying, managing, and configuring Sourcegraph for users on their instance. Site Admins have [elevated permissions](../privileges.md) within their Sourcegraph instance. 

This guide will walk you through the features and functionalities available to you as a Site Admin and show how you can get started managing and maintaining your Sourcegraph instance. For detailed, in-depth information, please reference the [administration guides and docs](index.md).

## Getting Started:

### What is the best deployment option for me?
We recommend Docker Compose for most initial production deployments. You can [migrate to a different deployment method](../updates/index.md#migrating-to-a-new-deployment-type) later on if needed.

If you need a deployment option that offers a higher level of scalability and availability, the [Kubernetes deployment](../deploy/kubernetes/index.md) is recommended. 

To help give you a starting point on choosing a deployment option and allocating resources to it, check out our [resource estimator](../deploy/resource_estimator.md).

For a comphrensive deployment guide for each option, check out our in-depth documentation for both [Docker Compose](../deploy/docker-compose/index.md) and [Kubernetes](../deploy/kubernetes/index.md).

### Deployment options 

Our recommended deployment type is [Kubernetes with Helm](../deploy/kubernetes/helm.md). If this is not a viable option, we also support a number of other deployment types which are described in the [Deployment overview](../deploy/index.md).

### Self-hosted vs. Managed instances
Regardless of the deployment option you choose, Sourcegraph can be self-hosted locally or with the cloud provider of your choice. We also offer [managed instances](../../cloud/index.md) (we handle deployment, updates, and management of the instance for you). Please [contact us](https://sourcegraph.com/contact/sales) if you are interested in learning more about managed instances. 


## Updating your instance 
New versions of Sourcegraph are released monthly (with patches released in between, as needed). New updates are announced in the [Sourcegraph blog](https://sourcegraph.com/blog), and comprehensive update notes are available in the [changelog](https://docs.sourcegraph.com/CHANGELOG). 

To check the current version of your instance, go to **User menu > Site admin > Updates**.

For more details on updating your instance, please refer to the [update docs](../updates/index.md).

## Configuration
As a Site Admin, you have the ability to control and configure the various aspects of your instance. Including the code host connection(s), SSO, repository indexing, and the functionality of the instance itself. 

### Site configuration 
At the heart of managing your Sourcegraph instance is Site configuration. Site config is a JSON file that defines how the various features and functionality within Sourcegraph are set up and configured. 

To access site config, go to **User menu > Site admin > Site configuration**.

### Admin users configuration 
If you need to add additional site admins, you can do so on the `/site-admin/users` page, under the actions for an individual user account. Any admin can revoke a user's admin privileges later using the same actions menu. 

### Connecting to code hosts
Sourcegraph supports connections to and repository syncing from any Git code host. Once connected, Sourcegraph will clone and index your repos so that users can search and navigate across them. To get started, go to **User menu > Site Admin > Manage code hosts > Add code host**.

Please reference the code host [documentation](../external_service/index.md) for detailed instructions on connecting to your code host.

### Setting up user authentication and repository permissions
In addition to connecting to your repositories, you can also configure Sourcegraph to use your preferred SSO or sign-in method and inherit and enforce repository permissions for users.

#### Auth & SSO:
Sourcegraph supports several different authentication methods: OAuth (for GitHub or GitLab), OpenID Connect (Google Workspace), and SAML; and also provides a built-in authentication method via email, if needed. 

To get started setting up user authentication and SSO, please reference our [auth documentation](../auth/index.md).

#### Repository Permissions:
In addition to configuring user authentication to Sourcegraph, you may also want to ensure that users can only view repositories that they would have access to on your code host. Sourcegraph supports the ability to inherit and enforce these repository permissions on a per-user basis and can be configured for connections to GitHub, GitLab, and Bitbucket Server / Bitbucket Data Center.

For more info, check out our complete [repository permissions documentation.](../permissions/index.md)


### External services 
By default, Sourcegraph bundles the services it needs to operate into installations. These services include PostgreSQL, Redis, and blobstore. 

Your Sourcegraph instance can also be configured to use existing external services if you wish. For more information on configuring Sourcegraph to use your external services, please reference this [documentation.](../external_services/index.md)

## Observability 
One key component to managing a Sourcegraph instance is having the ability to observe, monitor, and analyze the health of your instance. Sourcegraph ships with [Grafana](https://grafana.com/) for dashboards; [Prometheus](https://prometheus.io/) for metrics and alerting; as well as a [built-in alerting system.](../observability/alerting.md)

### Viewing instance health and metrics
Alerts and metrics can be viewed and monitored in Grafana. To access the Grafana dashboard bundled with your Sourcegraph instance, go to **User menu > Site admin > Monitoring**.

We also have an exhaustive [reference guide](../observability/dashboards.md) for understanding the available dashboards, and an [alert solutions guide](../observability/alerts.md) with descriptions and possible solutions for each alert that fires in Grafana. 

