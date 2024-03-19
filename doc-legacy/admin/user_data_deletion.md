# User data deletion

As a site administrator, you have the ability to delete users and their associated data on the **Admin** -> **Users** page (`https://sourcegraph.example.com/site-admin/users`).

On this page, you are presented two options:

- "Delete": the user and ALL associated data is marked as deleted in the DB and never served again.
- "Delete forever": the user and ALL associated data is permanently removed from the DB (you CANNOT undo this).

When deleting a user with either option, the following information is removed:

- All user data (access tokens, email addresses, external account info, repository permissions, survey responses, etc.)
- Organization membership information (which organizations the user is a part of, any invitations created by or targeting the user).
- Sourcegraph extensions published by the user on the instance the deletion request is sent to.
- User, Organization, or Global settings authored or modified by the user.
- Bulk operations on changesets and batch changes created by the user.

> WARNING: If deleted and recreated a user will no longer have access to the configurations and customizations they may have made for their old account. The database will store information about their previous configuration, but a user associated with a new `id` in the `users` table in PostgreSQL will be created. For example, site admin privileges will have to be re-assigned if a once site admin user is deleted and recreated. 

## Entities deletion

When a user or organization is deleted (for both "delete" or "delete forever"), here is what happens to their entities: 

| Entity | Deleted when you delete the creator? | Details |
| ------ | ------------------------------------ | ------- |
| Batch Change | Yes | If you delete the user or organization that owns the batch change, the batch change and all associated information gets deleted from our database and no longer appears in the UI. |
| Code Insight chart | Sometimes | Private insights never shared with others are deleted. Insights shared to org-visible or global dashboards persist as long as the org and global dashboard continue to exist. |
| Code Insight dashboard | Sometimes | If the dashboard was private to the user it is deleted on the user deletion; if it was private to the org it is deleted on the org deletion. If it was a global dashboard, it will continue to exist. |
| Code Monitor | Yes | If the user that created the code monitor is deleted, their code monitors are deleted and will no longer trigger new actions. |
| Repository Permissions | Yes | If you delete the user, the associated repository permissions will be deleted as well via a database trigger. If the user is revived later, the permissions need to be synced again. |
| Search Notebook | Yes | If you delete the user or organization that owns the notebook, it no longer appears in the UI. In the organization-owned case, it's still preserved in the database. |
| Search Context | Yes | If you delete the user or organization that owns the context, it no longer appears in the UI.  In cases where it was an organization notebook, it is preserved in the database. |
| Settings file (extensions, experimental features, defaults) | Yes | If you delete the user or organization, the associated settings file is deleted. |
