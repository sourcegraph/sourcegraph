# Testing in Dogfood

We have a [dogfood instance](https://handbook.sourcegraph.com/departments/product-engineering/engineering/process/deployments/instances#k8ssgdevorg) at [k8s.sgdev.org](https://k8s.sgdev.org). It can be used to safely test out new features in a larger setting than a local dev environment, without the risk of negatively impacting production.

Depending on the feature, you can turn it on either in the [dogfood config](https://github.com/sourcegraph/deploy-sourcegraph-dogfood-k8s) or in the web UI.

- Make a change in the configuration if the corresponding config file is present in the repository above. This can be done using a GitHub pull request.
- Otherwise, make the change using the UI.

For example, a `SITE_CONFIG_FILE` is not used for k8s, it is only used for Cloud and managed instances. So if you need to make changes to the site config, you can do that in the web UI.

To make a change to the site configuration in the web UI, first [create an account and ask someone to mark you as a site admin](https://handbook.sourcegraph.com/departments/product-engineering/engineering/process/deployments/playbooks#manage-users-in-k8ssgdevorg). After this, you should be able to make changes similar to a local development environment.

Other resources:

- [Debugging issues in dogfood](https://handbook.sourcegraph.com/departments/product-engineering/engineering/process/deployments/debugging/tutorial)
