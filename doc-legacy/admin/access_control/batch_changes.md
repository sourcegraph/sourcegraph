# Access control for Batch Changes

Granular controls for who can access [Batch Changes](../../batch_changes/index.md) can be configured by site admins by tuning the roles assigned to users and the permissions granted to those roles. This page describes the permission types available for Batch Changes, and whether they are granted by default to the **User** [system role](./index.md#system-roles). All permissions are granted to the **Site Administrator** system role by default.

Name      | Description | Granted to **User** by default?
--------- | ----------- | :-:
`batch_changes:read` | **_Coming soon!_**<!--<ul><li>User can view batch changes in the open or closed state, belonging to themselves or other users or orgs.</li><li>User can view most details about a batch change, including its latest batch spec, the changesets for repositories they have access to, the burndown chart, and bulk action logs.</li></ul>--> | ✓
`batch_changes:write` | <ul><li>User can create, update, close, or delete batch changes.</li><li>User can create, execute, and apply batch specs.</li><li>User can perform bulk operations on changesets such as publishing, commenting on, closing, or merging them.</ul> | ✓
