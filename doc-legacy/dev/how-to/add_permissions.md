# Permissions

This doc is for engineers who want to add permissions for features they want to protect using the Access Control System. The permission referred to in this context differs from repository permissions; the latter concerns permissions from code hosts relating to repositories.

## Overview
The RBAC system is based on two concepts:
  * **Namespaces**: these refer to resources that are protected by the RBAC system.
  * **Actions**: these are operations that a user can perform in a given namespace.

The source of truth for the Access Control system is the [`schema.yaml`](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/rbac/schema.yaml) file, which contains the list of namespaces and the actions available to each namespace.

## How it works

When Sourcegraph starts, a [background job](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph@8b4e1cb4374d1449c918f695c21d2c933c5a1d15/-/blob/cmd/frontend/internal/cli/serve_cmd.go?L205:24) is started that syncs the namespaces and actions into the `permissions` table in the database.

Permissions are a tuple of a namespace and an action available in that namespace. The background jobs removes actions and namespaces that are in the database but no longer referenced in the [`schema.yaml`](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/rbac/schema.yaml) file, and adds permissions contained in the [`schema.yaml`](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/rbac/schema.yaml) file but not in the database.

Once the permissions are synced, they can be used anywhere in Sourcegraph to protect unauthorized access to resources.

## Adding Permissions
To add permissions for a new feature, follow these steps:

1. Add the namespace and action to [`schema.yaml`](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/rbac/schema.yaml). Namespace string must be unique.

2. Generate the access control constants with the command `sg gen`. This will generate access control constants for [Typescript](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph@8b4e1cb/-/blob/client/web/src/rbac/constants.ts) and [Go](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph@8b4e1cb4374d1449c918f695c21d2c933c5a1d15/-/blob/internal/rbac/constants.go).
    - Additionally modify [schema.graphql](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph@2badaeb3ccb0d9bf660234e8b5eddb4cba6598bb/-/blob/cmd/frontend/graphqlbackend/schema.graphql?L9729) by adding a new namespace enum

3. Once these constants have been generated, you can protect any resource using the access control system. 
    * In Go, you can do this by importing the [`CheckCurrentUserHasPermission`](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph@8b4e1cb/-/blob/internal/rbac/permission.go) method from the `internal/rbac` package. [Example](https://github.com/sourcegraph/sourcegraph/pull/49594/files).

    * In Typescript, you can do this by accessing the authenticated user's permissions and verifying the [permission](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph@8b4e1cb4374d1449c918f695c21d2c933c5a1d15/-/blob/client/web/src/rbac/constants.ts?L5:14-5:41) you require is contained in the array. [Example](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph@8b4e1cb4374d1449c918f695c21d2c933c5a1d15/-/blob/client/web/src/batches/utils.ts?L6)
