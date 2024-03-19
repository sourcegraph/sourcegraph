<style>
.estimator label {
    display: flex;
}

.estimator .errorInput {
    font-style: italic;
    color: var(--critical-badge-color);
    border-color: var(--critical-badge-color);
}

.estimator .radioInput label {
    display: inline-flex;
    align-items: center;
    margin-left: .5rem;
}

.estimator .radioInput label span {
    margin-left: .25rem;
    margin-right: .25rem;
}

.estimator input[type=range] {
    width: 15rem;
}

.estimator .post-label {
    font-size: 16px;
    margin-left: 0.5rem;
}

.estimator .copy-as-markdown {
    width: 100%;
    height: 8rem;
}

.estimator a[title]:hover:after {
  content: attr(title);
  background: red;
  position: relative;
  z-index: 1000;
  top: 16px;
  left: 0;
}

</style>

<script src="https://storage.googleapis.com/sourcegraph-resource-estimator/go_1_18_wasm_exec.js"></script>
<script src="https://storage.googleapis.com/sourcegraph-resource-estimator/launch_script.js?v2" version="5dd901d"></script>

# Sourcegraph resource estimator

Updating the form below will recalculate an estimate for the resources you can use to configure your Sourcegraph deployment.

<form id="root"></form>

**Questions or concerns?** [Get help from an engineer](mailto:support@sourcegraph.com)

---

## Additional information

#### Limitations

- The estimator does not support instances with more than 50,000 users or 5,000,000 repositories.

#### How to update a resource in your deployment?

- For docker-compose deployments, edit your [docker-compose.yml file](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml) and set cpus and mem_limit to the limits shown above.
- For Helm deployments, create an [override file](https://github.com/sourcegraph/deploy-sourcegraph-helm/blob/main/charts/sourcegraph/examples/common-modifications/override.yaml) (or update your existing override file) with the new values shown above.
- For non-Helm Kubernetes deployments, we recommend using Kustomize to generate manifests with the values shown above. Please refer to our [Kustomize overlay for resources update](https://github.com/sourcegraph/deploy-sourcegraph/tree/master/overlays/resources-update) for detail.

#### What is engagement rate?

- (Deprecated) Engagement rate refers to the percentage of users who use Sourcegraph regularly. It is generally used for existing deployments to estimate resources.

#### If you plan to enforce repository permissions on Sourcegraph

- Repository permissions on Sourcegraph can have a noticeable impact on search performance if you have a large number of users and/or repositories on your code host. We suggest setting your authorization ttl values as high as you are comfortable setting it in order to reduce the chance of this (e.g. to 72h) in the repository permission configuration.

#### What kind of data can be regenerated without backup?

- See our docs on [Persistent data backup in Kubernetes](https://docs.sourcegraph.com/admin/deploy/migrate-backup#persistent-data-backup-in-kubernetes) for more detail.

#### How does Sourcegraph scale?

- [Click here to learn more about how each Sourcegraph service scales.](scale.md)
