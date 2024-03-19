# Sourcegraph Deployment Best Practices

## What does "best practice" mean to us?

Sourcegraph is a highly scalable and configurable application. As an open source company we hope our customers will feel empowered to customize Sourcegraph to meet their unique needs, but we cannot guarantee whether deviations from the below guidelines will work or be supportable by Sourcegraph. If in doubt, please contact your Customer Engineer or [reach out to support](../index.md#get-help).

## Sourcegraph Performance Dependencies

- number of users associated to instance
- user's engagement level
- number and size of code repositories synced to Sourcegraph.

_To get a better idea of your resource requirements for your instance use our_ [_resource estimator_](deploy/resource_estimator.md)_._

## Deployment Best Practices

A comparison table of supported self-hosted deployment methodologies can be [found here](deploy/index.md#deployment-types).

### Docker Compose

- Be sure your deployment meets our [Docker Compose requirements](deploy/docker-compose/index.md#requirements).
- Review the [configuration section](deploy/docker-compose/index.md#configuration) of our [Docker Compose deployment docs](deploy/docker-compose/index.md).

### Kubernetes

Kubernetes deployments may be customized in a variety of ways, we consider the following best practice:

- Users should configure and deploy using Helm, as covered in our guide to [using Helm with Sourcegraph](deploy/kubernetes/helm.md).
  -  If Helm cannot be used, [Kustomize can be used to apply configuration changes](deploy/kubernetes/kustomize.md).
  -  As a last resort, the [manifests can be edited in a forked copy of the Sourcegraph repository](deploy/kubernetes/index.md).
- The suggested Kubernetes version is the current [GKE Stable release version](https://cloud.google.com/kubernetes-engine/docs/release-notes-stable)
- We attempt to support new versions of Kubernetes 2-3 months after their release.
- Users are expected to run a compliant Kubernetes version ([a CNCF certified Kubernetes distribution](https://github.com/cncf/k8s-conformance))
- The cluster must have access to persistent SSD storage
- We test against Google Kubernetes Engine

_Unless scale, resiliency, or some other legitimate need exists that necessitates the use of Kubernetes (over a much simpler Docker Compose installation), it's recommended that Docker-Compose be used._

_Any major modifications outside of what we ship in the [standard deployment](https://github.com/sourcegraph/deploy-sourcegraph) are the responsibility of the user to manage, including but not limited to: Helm templates, Terraform configuration, and other ops/infrastructure tooling._

### Sourcegraph Server (single Docker container)

Sourcegraph Server is best used for trying out Sourcegraph. It's not intended for enterprise production deployments for the following reasons:

- Limited logging information for debugging
- Performance issues with:
  - more than 100 repositories
  - more than 10 active users
- Some Sourcegraph features do not have full functionality (Ex: Code Insights)

_It is possible to migrate your data to a Docker-Compose or Kubernetes deployment, contact your Customer Engineer or reach out to support and we'll be happy to assist you in upgrading your deployment._

## Additional support information

### Precise code navigation and Batch Changes

- The list of languages currently supported for precise code navigation can be found [here](https://docs.sourcegraph.com/code_navigation/references/indexers)
- Requirements to set-up Batch Changes can be found [here](https://docs.sourcegraph.com/batch_changes/references/requirements)

### Browser extensions

- Sourcegraph and its extensions are supported on the latest versions of Chrome, Firefox, and Safari.

### Editor extensions

Only the latest versions of IDEs are generally supported, but most versions within a few months up-to-date generally work.

- VS Code: [https://github.com/sourcegraph/sourcegraph/tree/main/client/vscode](https://github.com/sourcegraph/sourcegraph/tree/main/client/vscode); we don't yet support VSCodium
- JetBrains IDEs: [https://github.com/sourcegraph/sourcegraph/tree/main/client/jetbrains](https://github.com/sourcegraph/sourcegraph/tree/main/client/jetbrains) – we mainly test the plugin with IntelliJ IDEA, but it should work with no issues in all JetBrains IDEs, including:
  - IntelliJ IDEA
  - IntelliJ IDEA Community Edition
  - PhpStorm
  - WebStorm
  - PyCharm
  - PyCharm Community Edition
  - RubyMine
  - AppCode
  - CLion
  - GoLand
  - DataGrip
  - Rider
  - Android Studio
