# Viewing Code Insights

## Insights dashboards

The main way to view code insights is on a dashboard page. Dashboards have a unique name and visibility level.

There are three possible visibility levels:

- Private: visible only to you
- Shared with [an organization](../../../admin/organizations.md): visible to everyone in the organization
- Global: visible to everyone on the Sourcegraph instance

## Sharing links to individual insights

You can share links to individual insights by clicking the three-dots context menu on an insight and selecting the shareable link. 

In order to share a code insight with someone else, the insight must already be on a dashboard visible to them: either a global or organization's dashboard. 

The share link box will indicate which other users will be able to view the insight, or if the insight is private and must first be added to a more public dashboard in order to share it. 

### Built-in dashboards

A default "dashboard" of all insights visible to a user appears as the homepage for the Insights navigation bar item. 

To add insights to your own custom dashboard, see [Creating a custom dashboard of code insights](../how-tos/creating_a_custom_dashboard_of_code_insights.md).

### Dashboard visibility

A Dashboard's visibility level owns the visibility of code insights on the dashboard. If you add an Insight that was on a private dashboard to an organization or global dashboard, now people with access to that dashboard can view the insight. When you first create an insight, it defaults to appearing on the dashboard from which you clicked the "create" button, and inherits that original dashboard's permissions. Everyone who can access an insight to view it can also edit it. 

If you create an insight directly on the create page or from the default dashboard, it will appear on the default dashboard and default to "private" permissions. There's one exception: if the instance has no code insights license, then private dashboards and insights are disallowed, and all limited access mode insights are global. 

### Insights still enforce individual permissions regardless of dashboard visibility

The dashboard visibility levels have no impact on the data in the insight itself that is displayed to an individual user: insights [enforce the viewer's permissions](administration_and_security_of_code_insights.md#code-insights-enforce-user-permissions).

This means that two organization users with different repo read permission sets might see different values for the same insights on the same dashboards, if it contains results from a repository only one user can view.

If you change a private-visible dashboard so that the dashboard is now visible to an organization (or globally), then the organization (or entire instance) can now see all insights on the dashboard, though the result counts will still be filtered to count from only repos that the individual organization member can view. 

### Insights can be on multiple dashboards

You can attach insights to multiple dashboards.

