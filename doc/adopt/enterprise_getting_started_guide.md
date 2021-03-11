# Enterprise Getting Started Guide

If you're deploying a new Enterprise instance, this page covers our most frequently referenced pieces of documentation. Admins will be interested in the documentation for their specific deployment method; users will want to check out info on our search syntax, operators, batch changes, and browser extension.

## Admin articles

### General
- [Resource estimator](../admin/install/index.md)
- [SAML config](../admin/auth/saml/index.md)
- [Built-in password authentication](../admin/auth/index.md#builtin-password-authentication)
- [GitHub authentication](../admin/auth/index.md#github)
- [GitLab authentication](../admin/auth/index.md#gitlab)
- [OpenID connect](../admin/auth/index.md#openid-connect)
- [HTTP authentication proxy](../admin/auth/index.md#http-authentication-proxies)
- [GitLab integration](../integration/gitlab.md)
- [GitHub integration](../integration/github.md)
- [All code host integrations (not GitLab or GitHub)](../integration/index.md#integrations)
- [Full guide to site config options](../admin/config/site_config.md#auth-sessionExpiry)
- [Changelog](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/CHANGELOG.md) to track releases and updates

### Docker-compose
- [Basic installation guide](../admin/install/docker-compose/index.md)
- [AWS installation](../admin/install/docker-compose/aws.md)
- [Digital Ocean installation](../admin/install/docker-compose/digitalocean.md)
- [Google Cloud installlation](../admin/install/docker-compose/google_cloud.md)

### Kube admin
- [Basic installation guide](../admin/install/kubernetes/index.md)
- [Provisioning a Kubernetes cluster](../admin/install/kubernetes/configure.md#configuring-sourcegraph)
- [Amazon EKS](../admin/install/kubernetes/eks.md)
- [Amazon EC2](https://kubernetes.io/docs/setup/)
- [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine/docs/quickstart)
- [Azure](../admin/install/kubernetes/azure.md)
- [Scaling](../admin/install/kubernetes/scale.md#improving-performance-with-a-large-number-of-repositories) 

## User articles
- [Search syntax](../code_search/reference/queries.md)
- [Search operators](../code_search/reference/queries.md#keywords-all-searches)
- [Example batch changes](../batch_changes/tutorials/index.md)
- [Sourcegraph browser extension](../integration/browser_extension.md)
