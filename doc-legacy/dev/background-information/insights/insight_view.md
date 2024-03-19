## Background
A Code Insight can be thought of as a few discrete components:
1. A visualization of time series data with metadata such as title and labels, as well as time range to display
2. A collection of data series comprised of sample recordings

![Example insight](https://raw.githubusercontent.com/sourcegraph/sourcegraph/1811a8392ab8fe85267c51a0f2a4ee2c9213b4ee/doc/dev/background-information/insights/diagrams/insight_example.png)

Typically Insights have been thought of as the coupling of these components. When a user creates an Insight using the creation UI, they are prompted to fill out a query to generate recordings as well as the visualization metadata. When these insights are stored, they are stored as a single object that directly couples metadata to the recording series.

## Insight Views
An Insight View can be thought of as the visualization component of the current insights, with metadata about which partitions of the underlying data to resolve. A view does not directly create or generate any new data series. Every data series will have at least one view, but many views may be created on top of any data series. These views are analogous to a database view, but include rendering information on top of the data selections as well as ownership and visibility information to support flexible permissions. In general, views are incredibly cheap compared to the cost of executing a Sourcegraph query across the entire search corpus in both time and space.

### Filtering
Insight Views provide support for building intuitive and reusable components when filtering the underlying data series. A simple filter of a slice of repositories can be created as a view. Adding one more repository to this set can be thought of as a new view.

A user that is exploring different filter options for an Insight could be thought as generating temporary views with different data selections.

### Persistence
A view can be trivially persisted, allowing a simple and fast mechanism for a user to generate dashboards of Sourcegraph queries visualized in different ways.

Persisted insight views are stored in the `insight_view` table.

### Sharing / Permissions
A view can support a sharing and permission model that otherwise would be difficult or impossible in Sourcegraph. A view could be shared from one user to another carrying the exact permissions derived from the first user. A different view could be created that resolves permissions based on the viewing user.

It would even be possible to create a view to share publicly for anonymous access.

These views allow users to share the resolved visualizations without revealing any information about the search query or underlying data used to generate them.

### Customization
A view that is shared across users can be thought of as a copy of the original view. Metadata such as graph type or line color could be trivially changed for each user, allowing individual customization across users, teams, and organizations. One user may choose to visualize an Insight with a time chart view, where another user might visualize the same data in a pie chart.

A limited set of series specific metadata is stored on the `insight_view_series` table.
