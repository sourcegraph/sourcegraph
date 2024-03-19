# How to deploy a new executor image

This guide documents how to deploy a new image of [executors](../../../admin/executors/index.md) to the following [Sourcegraph instances](https://handbook.sourcegraph.com/departments/product-engineering/engineering/process/deployments/instances):

* [Sourcegraph.com](https://sourcegraph.com)
* [k8s.sgdev.org](https://k8s.sgdev.org)

## Requirements

* Clone of [`infrastructure`](https://github.com/sourcegraph/infrastructure) repository
* `terraform` in the version specified in [executors/.tool-versions](https://github.com/sourcegraph/infrastructure/blob/main/executors/.tool-versions)
  * Using `asdf`: `asdf install terraform x.x.x`
* Authenticated with GCP: `gcloud auth application-default login`
* AWS credentials set as env vars: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`

## Steps

1. Make a change to the `executor` code so that the buildkite build results in new `executor` AWS/GCP images being built & published. Or push to a branch with the [`executor-patch-notest/`](https://github.com/sourcegraph/sourcegraph/blob/882ed49014bc470a3be17ce74764a856f82bee4e/dev/ci/internal/ci/runtype.go#L65-L67) prefix to trigger the build.
1. Look at the buildkite build. Example: https://buildkite.com/sourcegraph/sourcegraph/builds/116966

    There are two steps with `executor-image` in here. The first one builds and uploads image. Second step releases it.

    Try to find the GCP image name, i.e. `executor-cc28c728e5-116966`

    Try to find the AWS AMI name, i.e. `ami-0fb21656aeba5eb7c`

2. In the `infrastructure` repository:
  * Open `executors/(dogfood|cloud)/gcp.tf` and update the image at the top in `gcp_executor_machine_image`.
  * open `executors/dogfood/aws.tf` and update the image at the top in `aws_executor_ami` (only for dogfood).
3. Create a pull request with that change.
4. Get approval for PR.
5. In that PR branch, in the `executors/(dogfood|cloud)` folders, run: `terraform apply`.
6. Open https://k8s.sgdev.org/site-admin/executors to see check the new version is used.
7. Merge PR.

## Releasing a new terraform module version with these newly built images

When we only dogfood a new image in between Sourcegraph releases using our own enviroments,
this step can be skipped. Otherwise, we need to update the variable defaults for those images
as well. This, for example, should be done before every release cut to ensure the terraform
module version for the to-be-cut Sourcegraph version exists.

1. Clone both [github.com/sourcegraph/terraform-google-executors](https://github.com/sourcegraph/terraform-google-executors) and [github.com/sourcegraph/terraform-aws-executors](https://github.com/sourcegraph/terraform-aws-executors).
1. In each module, replace the variable default for the machine image both in the root module `variables.tf` and in the `./modules/executors` module's `variables.tf`.
1. Open a PR with this change.
1. Get approval and merge this PR.
1. If necessary: Cut a new release of the modules. For that, run `./release.sh` from the two repos. See [the section on compatibility with Sourcegraph](https://github.com/sourcegraph/terraform-google-executors#compatibility-with-sourcegraph) for versioning constraints.
1. Update the executor modules in our infrastructure repo to validate and dogfood those new version tags.
