# Terraform style guide

Also see: general Terraform [styleguide](https://www.terraform.io/docs/configuration/style.html)

## Terraform versioning

We use [asdf](https://asdf-vm.com/#/) to manage Terraform versions. After installing Asdf into your shell, run `asdf plugin add terraform && asdf install && asdf reshim` to pickup the Terraform version defined in the `.tool-version` file for the specific Terraform module.

## State

State must be stored using a [GCS Terraform state backend](https://www.terraform.io/docs/backends/types/gcs.html).

Example configuration
```
terraform {
  required_version = "0.12.26"

  backend "gcs" {
    bucket = "sourcegraph-tfstate"
    prefix = "infrastructure/dns"
  }
}
```

### State for state buckets

Because we need to create state buckets as code, we also need to store the state of the code that creates the state bucket. Given this code rarely changes and that moving it to be stored in a remote location creates a chicken and egg situation, we will store state bucket creation's state in Git.

### Bucket

State for all Sourcegraph resources is stored in [sourcegraph-tfstate bucket](https://github.com/sourcegraph/infrastructure/tree/master/terraform-state).

Managed instances resources will be stored on a per customer bucket following the pattern: `sourcegraph-managed-${NAME}`.

### Prefix

State for different pieces of infrastructure require separate state files. To facilitate matching state to resources and code, we will use the following pattern: `${REPOSITORY_NAME}/${INFRASTRUCTURE_NAME}`.

## Formatting

- Format all code using `terraform fmt`
- Remove duplicate empty new lines
  > Terraform > `0.12`, the `fmt` command lost the ability (as it was too aggressive) to remove multiple empty new-lines, etc.

## General

- Use **lowercase**
- Use [snake_case](https://en.wikipedia.org/wiki/Snake_case) for all resources and names
- Pin Terraform version
- Pin all providers to a major version

## Resources

- Avoid duplicating the resource type in its name
  > good: `resource "google_storage_bucket" "foo"`
  > bad: `resource "google_storage_bucket" "foo_bucket"`

## Variables and Outputs

- Remove any unused variables
- Include a `type`
- Include a `description` if the intent of the variable is not obvious from its name
- Names should reflect the attribute or argument they reference in its suffix
  > good: `foo_bar_name` good: `foo_bar_id` bad: `foo_bar`
- Use plural form in names of variables that expect a collection of items
  > multiple: `foo_bars` single: `foo_bar`

## Basic Project Layout

```
├── main.tf       # Main terraform configurations
├── foo.tf        # Terraform resources for `foo`
├── output.tf     # Output definitions
├── provider.tf   # Providers and Terraform configuration
└── variables.tf  # Variables definitions
```

### `providers.tf`

Contains all `providers` and `terraform` blocks.

### `main.tf`

Contains Terraform `resources`, `data` and `locals` resources. On larger projects, keep generic `resource`, `data` and `locals` definitions in this file and split the rest to other files.

### `output.tf`

Contains all `output` definitions.

### `variables.tf`

Contains all `variable` definitions.

## Playbooks

### Statefile surgery

_Note: these are provided as internal reference documents. You should rarely need to use this and please reach out to the distribution team or ask in the #dev-ops channel before performing these steps._

To debug some more involved terraform problems you may need to directly modify the state file.

1. Use `TF_DEBUG` environment variable with `WARN`, `DEBUG` logs enabled to locate a possible source of the issue.

1. Use `terraform state pull` to examine the statefile locally.

1. If possible, use `terraform state rm`/`terraform state mv X Y` to make the necessary state modifications.
   Otherwise, manually edit the state file and run `terraform state push` to update the remote state.

    1. In cases where a corrupted state has been pushed, you can pull a previous object version with [gsutil](https://cloud.google.com/storage/docs/using-object-versioning)

1. File bugs as appropriate, statefile modification should typically not be necessary
