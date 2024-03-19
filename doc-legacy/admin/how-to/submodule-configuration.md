# How to configure submodules

This document will walk you through how to configure submodules in Sourcegraph. We use `.gitmodules` files to figure out where a submodule exists in the tree. When a user clicks on the module in the UI, they will be redirected to the repo for the module.

## Prerequisites

This document assumes that you have access and permission to create/edit the `.gitmodules` file for the repository.

## Steps
1. Make sure you have a `.gitmodules` file created in folder level
2. Setup the `.gitmodules` file following the format shown in our example below

## Example

URL should be used instead of relative path when setting up the `.gitmodules` file for GitLab repos:

```
[submodule "test-repo-2"]
        path = test-repo-2
        url = https://github.com/test-repos/test-repo-2
```

> WARNING: We currently do not support relative path setups for submodules in Sourcegraph.
