# Usage statistics

> NOTE: With 3.42 we introduced an all new analytics experience for admins. More information can be found [here](./analytics.md).

Sourcegraph records basic per-user usage statistics. To view analytics, visit the **Site admin > Usage stats** page. (The URL is `https://sourcegraph.example.com/site-admin/usage-statistics`.) This information is available via the GraphQL API to all viewers (not just site admins).

Here you can see charts with counts of unique users by day, week, or months.

From this page, you can also see user-level activity, including counts of pageviews, searches, and code intelligence actions, and last active times. This user-level data is all stored locally on your Sourcegraph instance, and is never sent to Sourcegraph.com. The user-specific values are recorded over all time.

| Column                                   | Description                                                                             |
| -----------                              | ----------------------------------------------------------------------------------------|
| User                                     | This column displays all users of the sourcegraph instance.                             |
| Page views                               | This column displays the counts of page views of a particular user.                     |
| Search Queries                           | This column displays the count of searches a user makes.                                |
| Code intelligence actions                | This column displays the counts of code intelligence actions of a particular user       | 
| Last active                              | This column displays information about when last a user was actively using Sourcegraph. |
| Last active in code host  or code review | This column displays information about when last a particular user used Sourcegraph in the code host. For example, researching, reviewing code and code intel in the code host.                                                                         | 

```Note:```  This can also be filtered into active users today, this week and this month.

## See also 

- [User satisfaction surveys](user_surveys.md)
