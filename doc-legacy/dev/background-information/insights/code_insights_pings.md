# Code Insights Pings

Code Insights pings allow us to quantitatively measure the usage and success of Code Insights. This page is a source of truth for detailed explanations, statuses, and implementations of our pings. 

We keep this docs page up to date because pings are a vital component of our product knowledge and prioritization process, and a broken or incorrect ping impacts 3-5 months of data (because that's how long a fix takes to propagate).

<!-- 
TEMPLATE 

**Type:** FE/BE event

**Intended purpose:** Why does this ping exist?

**Functional implementation:** When does this event fire?

**Other considerations:** Anything worth noting.

- Aggregation: e.g. total, weekly
- Event Code: link to a Sourcegraph search of this event code name
- **Version added:** (link to PR)
- **Version(s) broken:** (only add if required, link to fix PR)
-->

## Terminology

- **FE event** - log events that we send by calling standard telemetry service on the frontend. These pings live only in the `event_logs` table. These typically represent user actions, such as hovers.
- **BE capture** - pings that our BE sends to the ping store by checking/selecting data from database tables. Our backend periodically sends these pings to the `event_logs` table. These typically represent absolute counts across the entire instance. 

## Metrics

### Additions count, edits count, and removals count 

**Type:** FE event

**Intended purpose:** To track how many times customers have created, edited, and removed insights, by week. 

**Functional implementation:** We track insight creating/editing/deleting events in the creation UI form and insight context menu component with standard telemetry service calls.  

**Other considerations:** N/A

- Aggregation: By week 
- Event Code: [InsightAddition](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27InsightAddition%27&patternType=literal), [InsightEdit](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27InsightEdit%27&patternType=literal), [InsightRemoval](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27InsightRemoval%27&patternType=literal)
- PRs: [#17805](https://github.com/sourcegraph/sourcegraph/pull/17805/files)
- **Version Added:** 3.25
- **Version(s) broken:**  3.31-3.35.0 (does not count backend insights) ([fix PR](https://github.com/sourcegraph/sourcegraph/pull/25317))

### Hovers count

**Type:** FE event

**Intended purpose:** To track how many times users hover over a datapoint to see the tooltip on the graph, or "dig in" to the information. 

**Functional implementation:** This ping works by firing an event on the client when a user hovers over a datapoint on a code insight. 

**Other considerations:** N/A

- Aggregation: By week 
- Event Code: [InsightHover](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27InsightHover%27&patternType=literal) 
- PRs: [#17805](https://github.com/sourcegraph/sourcegraph/pull/17805/files)
- **Version Added:** 3.25
<!-- - **Known versions broken:** N/A -->

### UI customizations count

**Type:** FE event

**Intended purpose:** To track how many times users resize the insight graphs. 

**Functional implementation:** This ping works by firing an event on the client when a user resizes a Code Insights graph on the page. 

**Other considerations:** N/A

- Aggregation: By week 
- Event Code: [InsightUICustomization](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27InsightUICustomization%27&patternType=literal) 
- PRs: [#17805](https://github.com/sourcegraph/sourcegraph/pull/17805/files)
- **Version Added:** 3.25
<!-- - **Known versions broken:** N/A -->

### Data point clicks count

**Type:** FE event

**Intended purpose:** To track how many times users click a datapoint to get to a diff search. 

**Functional implementation:** This ping works by firing an event on the client when a user clicks an individual data point of an insight graph, which takes them to a diff search. 

**Other considerations:** Because this functionality does not yet exist for backend insights, it only tracks clicks on frontend insights. 

- Aggregation: By week 
- Event Code: [InsightDataPointClick](
https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27InsightDataPointClick%27&patternType=literal) 
- PRs: [#17805](https://github.com/sourcegraph/sourcegraph/pull/17805/files)
- **Version Added:** 3.25
<!-- - **Known versions broken:** N/A -->

### Page views count

**Type:** FE event

**Intended purpose:** To track how many times users view insights pages. 

**Functional implementation:** This ping works by firing an event on the client when a user views _any_ /insights page, whether it's creating or viewing insights.  

**Other considerations:** As we add new insights pages it's important to make sure we're adding pages to this counter. 

- Aggregation: By week 
- Event Code: [ViewInsights](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+ViewInsights&patternType=regexp), [StandaloneInsightPageViewed](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+StandaloneInsightPageViewed&patternType=regexp)
- PRs: [#17805](https://github.com/sourcegraph/sourcegraph/pull/17805/files)
- **Version Added:** 3.25
- **Version(s) broken:** 3.25-3.26 (not weekly)([fix PR](https://github.com/sourcegraph/sourcegraph/pull/20070/files)), 3.30 (broken when switching to dashboard pages, didn't track dashboard views)([fix PR](https://github.com/sourcegraph/sourcegraph/pull/24129/files))

### Unique page views count

**Type:** FE event

**Intended purpose:** To track how many unique users are viewing insights pages each week. 

**Functional implementation:** This ping works by firing an event on the client when a unique user views _any_ /insights page for the first time that week, whether it's creating or viewing insights.  

**Other considerations:** As we add new insights pages it's important to make sure we're adding pages to this counter. 

- Aggregation: By week 
- Event Code: [InsightsUniquePageView](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightsUniquePageView&patternType=regexp) 
- PRs: [#17805](https://github.com/sourcegraph/sourcegraph/pull/17805/files)
- **Version Added:** 3.25
- **Version(s) broken:** 3.25-3.26 (not weekly)([fix PR](https://github.com/sourcegraph/sourcegraph/pull/20070/files)), 3.30 (broken when switching to dashboard pages, didn't track dashboard views)([fix PR](https://github.com/sourcegraph/sourcegraph/pull/24129/files))

### Standalone insights page filters edits count

**Type:** FE event

**Intended purpose:** To track how many users actively re-filter insights through the standalone insight page's filter panel.

**Functional implementation:** This ping works by firing a telemetry event on the client when a user changes insights filters (include/exclude repository regexp, search context, etc).

**Other considerations:** N/A

- Aggregation: By week
- Event Code: [InsightFiltersChange](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27InsightFiltersChange%27&patternType=literal)
- PRs: [#37521](https://github.com/sourcegraph/sourcegraph/pull/37521)
- **Version Added:** 3.41

### Standalone insights page dashboard clicks count

**Type:** FE event

**Intended purpose:** To track how many users are discovering dashboards from this page.

**Functional implementation:** This ping works by firing a telemetry event on the client when a user clicks any dashboard pills on the standalone insight page.

**Other considerations:** N/A

- Aggregation: By week
- Event Code: [StandaloneInsightDashboardClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27StandaloneInsightDashboardClick%27&patternType=literal)
- PRs: [#37521](https://github.com/sourcegraph/sourcegraph/pull/37521)
- **Version Added:** 3.41

### Standalone insights page edit button clicks count

**Type:** FE event

**Intended purpose:** To track how many users are going to the edit page through the standalone insight page.

**Functional implementation:** This ping works by firing a telemetry event on the client when a user clicks on the edit button on the standalone insight page.

**Other considerations:** N/A

- Aggregation: By week
- Event Code: [StandaloneInsightPageEditClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27StandaloneInsightPageEditClick%27&patternType=literal)
- PRs: [#37521](https://github.com/sourcegraph/sourcegraph/pull/37521)
- **Version Added:** 3.41

### In-product landing page events (hover, data points click, template section clicks)

**Type:** FE events

**Intended purpose:** To track unique users' activity on the in-product (get started insights) and the cloud landing pages.

**Other considerations:** N/A.

- Aggregation: By week
- Event Codes: 
   - [InsightsGetStartedPage](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightsGetStartedPage&patternType=regexp) to track how many unique users are viewing get started page
   - [InsightsGetStartedPageQueryModification](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightsGetStartedPageQueryModification&patternType=regexp) to track how many users change their live insight example query field value
   - [InsightsGetStartedPageRepositoriesModification](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightsGetStartedPageRepositoriesModification&patternType=regexp) to track how many users change their live insight example repositories field value
   - [InsightsGetStartedPrimaryCTAClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightsGetStartedPrimaryCTAClick&patternType=regexp) to track how many users click "Create your first insight" (call to action) button
   - [InsightsGetStartedTabClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightsGetStartedTabClick&patternType=regexp) to track how many users browse different template tabs on the in-product landing page, it sends selected tab `title` in event's payload data.
   - [InsightGetStartedTemplateClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightGetStartedTemplateClick&patternType=regexp) to track how many users click on the explore/use template button.
   - [InsightsGetStartedTabMoreClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightsGetStartedTabMoreClick&patternType=regexp) to track how many users expand to full template section, it sends selected tab `title` in event's payload data.
   - [InsightsGetStartedDocsClicks](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightsGetStartedDocsClicks&patternType=regexp) to track clicks over the in-product page's documentation links.
- PRs: [#31048](https://github.com/sourcegraph/sourcegraph/pull/31048)
- **Version Added:** 3.37

### Org-visible insights count (Total) 

**Type:** BE capture

**Intended purpose:** To track how many insights are visible by more than just the creator of the insight. 

**Functional implementation:** we gather this on the backend by joining the `insight_view` and `insight_view_grants` tables and counting the insights with org level grants. 

**Other considerations:** N/A

- Aggregation: total time, by insight type
- Event Code: [InsightOrgVisible](https://sourcegraph.com/search?q=context:global+insightorgvisible+r:sourcegraph/sourcegraph%24&patternType=literal)
- PRs: [#21671](https://github.com/sourcegraph/sourcegraph/pull/21671/files)
- **Version Added:** 3.29
- **Version(s) broken:** 3.31-3.35.0 (doesn't handle backend insights) [fix PR](https://github.com/sourcegraph/sourcegraph/pull/28425)

### First time insight creators count

**Type:** FE event

**Intended purpose:** To track the week and count of the first time a user(s) creates a code insight, of any type, on an instance. The sum of first time insight creators count over all time is equal to the total number of unique creators who have made an insight.

**Functional implementation:** This metric queries the insight table for new addition events, then filters by unique IDs that appeared for the first time that week. 

**Other considerations:** TODO does this ping include creators who create via the API? 

- Aggregation: By week
- Event Code: [WeeklyFirstTimeInsightCreators](https://sourcegraph.com/search?q=context:global+WeeklyFirstTimeInsightCreators+r:sourcegraph/sourcegraph%24&patternType=regexp)
- **Version Added:** 3.25
- **Version(s) broken:** 3.31-3.35.0 (doesn't handle backend insights, other bugs)

### Total count of insights grouped by step size (days)

**Type:** BE capture

**Intended purpose:** To track the x-axis (time window) set by users on frontend insights, to help prioritize features related to setting time windows. 

**Functional implementation:** this metric runs on the backend over all the insights. 

**Other considerations:** N/A

- Aggregation: total 
- Event Code: [InsightTimeIntervals](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightTimeIntervals&patternType=literal)
- **Version added:** 3.29
- **Version(s) broken:** 3.31-3.35.0 [fix PR](https://github.com/sourcegraph/sourcegraph/pull/28425)

### Code Insights View/Click Creation Funnels

**Type:** FE event

**Intended purpose:** These pings allow us to both understand how the view/click/view/click conversion funnel works for the creation flows of all existing types of insights, as well as smell-check other pings. The reason we use both "view" and "button clicks" in this funnel is that it's possible to view a page without "funneling through" via the prior page's CTA (for example: you can reach the creation/edit screen by the "edit" button, which does not involve logging a click on the "create search insight" button).

**Functional implementation:** These events fire on the frontend when the user takes the below actions. 

**Other considerations:** 

- Aggregation: By week
- Event Code:
   - For the "search insight" funnel: (1) [ViewCodeInsightsCreationPage](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+ViewCodeInsightsCreationPage&patternType=regexp), (2) [CodeInsightCreateSearchBasedInsightClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+CodeInsightsCreateSearchBasedInsightClick&patternType=regexp), (3) [ViewSearchBasedCreationPage](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+SearchBasedCreationPage&patternType=regexp), (4.1) [SearchBasedCreationPageSubmitClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+SearchBasedCreationPageSubmit&patternType=regexp) OR (4.2) [SearchBasedCreationPageCancelClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+SearchBasedCreationPageCancelClick&patternType=regexp)
   - For the "language stats insight" funnel: (1) [ViewCodeInsightsCreationPage](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+ViewCodeInsightsCreationPage&patternType=regexp), (2) [CodeInsightsCreateCodeStatsInsightClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+CodeInsightsCreateCodeStatsInsightClick&patternType=regexp), (3) [ViewCodeInsightsCodeStatsCreationPage](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+CodeInsightsCodeStatsCreationPage&patternType=regexp), (4.1) [CodeInsightsCodeStatsCreationPageSubmitClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+CodeInsightsCodeStatsCreationPageSubmitClick&patternType=regexp) OR (4.2) [CodeInsightsCodeStatsCreationPageCancelClick](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+CodeInsightsCodeStatsCreationPageCancelClick&patternType=regexp)
- **Version added:** 3.29
<!-- - **Version(s) broken:**  -->

### View series counts

**Type:** BE capture

**Intended purpose:** To track the number of view series, grouped by presentation type and generation method. Note: a "view series" differs from a "series" by being attached to a particular insight. A series can be attached to more than one insight.

**Functional implementation:** This is calculated by joining the `insight_series`, `insight_view_series`, and `insight_view` tables.

**Other considerations:** N/A

- Aggregation: total 
- Event Code: [ViewSeriesCounts](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+ViewSeriesCounts&patternType=literal)
- **Version added:** 3.34
<!-- - **Version(s) broken:**  -->

### Series counts

**Type:** BE capture

**Intended purpose:** To track the number of series, grouped by generation method.

**Functional implementation:** This is calculated using the `insight_series` table. 

**Other considerations:** N/A

- Aggregation: total 
- Event Code: [SeriesCounts](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+SeriesCounts&patternType=literal)
- **Version added:** 3.34
<!-- - **Version(s) broken:**  -->

### View counts

**Type:** BE capture

**Intended purpose:** To track the number of insight views, grouped by presentation type.

**Functional implementation:** This is calculated using the `insight_view` table. Unlike critical telemetry which only [shows the number of unlocked insights](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/insights/background/pings/insights_ping_aggregators.go?L286) for customers without full access, this ping [shows the total number that are locked or unlocked](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/insights/background/pings/insights_ping_aggregators.go?L243-247) (and may have been created during a trial or free beta phase).  

**Other considerations:** N/A

- Aggregation: total 
- Event Code: [ViewCounts](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+ViewCounts&patternType=literal)
- **Version added:** 3.34
<!-- - **Version(s) broken:**  -->

### Total orgs with dashboards

**Type:** BE capture

**Intended purpose:** To track the number of orgs with at least one dashboard.

**Functional implementation:** This is calculated using the `dashboard_grants` table.

**Other considerations:** N/A

- Aggregation: total 
- Event Code: [TotalOrgsWithDashboard](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+TotalOrgsWithDashboard&patternType=literal)
- **Version added:** 3.38
<!-- - **Version(s) broken:**  -->

### Total dashboard count

**Type:** BE capture

**Intended purpose:** To track the total number of dashboards.

**Functional implementation:** This is calculated using the `dashboard` table.

**Other considerations:** N/A

- Aggregation: total 
- Event Code: [TotalDashboardCount](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+TotalDashboardCount&patternType=literal)
- **Version added:** 3.38
<!-- - **Version(s) broken:**  -->

### Insights per dashboard

**Type:** BE capture

**Intended purpose:** To track statistics (average, min, max, median, std dev,) about how many insights are on each dashboard. 

**Functional implementation:** These are calculated using the `dashboard_insight_view` table.

**Other considerations:** N/A

- Aggregation: total 
- Event Code: [InsightsPerDashboard](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+InsightsPerDashboard&patternType=literal)
- **Version added:** 3.38
<!-- - **Version(s) broken:**  -->

### Series backfill time

**Type:** BE capture

**Intended purpose:** To track how long on average it takes series to backfill.

**Functional implementation:** Exposes aggregate information using the backfill times found on `insight_series`.

**Other considerations:** N/A

- Aggregation: weekly
- Event Code: [WeeklySeriesBackfillTime](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+WeeklySeriesBackfillTime&patternType=standard)
- **Version added:** 4.1

### Data export requests

**Type:** BE capture

**Intended purpose:** To track usage of data exporting functionality.

**Functional implementation:** Telemetry events are fired when a request reaches the backend HTTP handler, whether that comes from the webapp or the CLI.

**Other considerations:** The ping name contains `click` but this does indeed also record events from the CLI.

- Aggregation: weekly
- Event Code: [WeeklyDataExportClicks](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+WeeklyDataExportClicks&patternType=standard), `InsightsDataExportRequest` in `event_logs`
- **Version added:** 5.0

## Search results aggregations metrics

### Information icon hovers

**Type:** FE event

**Intended purpose:** To track interest in the feature.

**Functional implementation:** This ping works by firing a telemetry evente on the client when a user hovers over the information icon.

- Aggregation: weekly
- Event Code: [WeeklyGroupResultsInfoIconHover](https://sourcegraph.com/search?q=context:%40sourcegraph/all+GroupResultsInfoIconHover&patternType=lucky)
- **Version added:** 4.0 ([#40977](https://github.com/sourcegraph/sourcegraph/pull/40977))


### Sidebar and expanded view events

**Type:** FE event

**Intended purpose:** To track how users are using the different aggregation UI modes.

**Functional implementation:** These pings work by firing telemetry events on the client when a user expands or collapses the sidebar or full view panel.

**Other considerations**: For the expanded UI mode events we record which aggregation mode was toggled.

- Aggregation: weekly
- Event Codes:
  - [WeeklyGroupResultsOpenSection](https://sourcegraph.com/search?q=context:%40sourcegraph/all+GroupResultsOpenSection&patternType=lucky)
  - [WeeklyGroupResultsCollapseSection](https://sourcegraph.com/search?q=context:%40sourcegraph/all+GroupResultsCollapseSection&patternType=lucky)
  - [WeeklyGroupResultsExpandedViewOpen](https://sourcegraph.com/search?q=context:%40sourcegraph/all+GroupResultsExpandedViewOpen&patternType=lucky)
  - [WeeklyGroupResultsExpandedViewCollapse](https://sourcegraph.com/search?q=context:%40sourcegraph/all+GroupResultsExpandedViewCollapse&patternType=lucky)
- **Version added:** 4.0 ([#40977](https://github.com/sourcegraph/sourcegraph/pull/40977))

### Aggregation modes clicks and hovers

**Type:** FE event

**Intended purpose:** To track aggregation mode usage and interest.

**Functional implementation:** These pings work by firing telemetry events on the client when a user clicks on a mode or hovers over a disabled mode.

**Other considerations:** The ping also includes data for current UI mode.

- Aggregation: weekly
- Event Codes: 
  - [WeeklyGroupResultsAggregationModeClicked](https://sourcegraph.com/search?q=context:%40sourcegraph/all+WeeklyGroupResultsAggregationModeClicked%7CGroupAggregationModeClicked&patternType=regexp)
  - [WeeklyGroupResultsAggregationModeDisabledHover](https://sourcegraph.com/search?q=context:%40sourcegraph/all+WeeklyGroupResultsAggregationModeDisabledHover%7CGroupAggregationModeDisabledHover&patternType=regexp)
- **Version added:** 4.0 ([#40997](https://github.com/sourcegraph/sourcegraph/pull/40997))

### Aggregation chart clicks and hovers 

**Type:** FE event

**Intended purpose:** To track if users are hovering over results and clicking through.

**Functional implementation:** These pings work by firing telemetry events on the client when a user hovers or clicks on a result.

**Other considerations:** The ping also includes data for the current UI mode and aggregation mode.

- Aggregation: weekly
- Event Codes:
  - [WeeklyGroupResultsChartBarClick](https://sourcegraph.com/search?q=context:%40sourcegraph/all+GroupResultsChartBarClick&patternType=regexp)
  - [WeeklyGroupResultsChartBarHover](https://sourcegraph.com/search?q=context:%40sourcegraph/all+GroupResultsChartBarHover&patternType=regexp)
- **Version added:** 4.0 ([#40977](https://github.com/sourcegraph/sourcegraph/pull/40977))

### Search mode (proactive/extended) success rate

**Type:** FE event

**Intended purpose:** To track the number of aggregation searches that succeed or hit limit in either a proactive or extended search. 

**Functional implementation:** These pings fire a telemetry event when an aggregation search completes or times out.

- Aggregation: weekly
- Event Code: [WeeklyGroupResultsSearches](https://sourcegraph.com/search?q=context:%40sourcegraph/all+WeeklyGroupResultsSearches&patternType=lucky)
- **Version added:** 4.1 ([#42554](https://github.com/sourcegraph/sourcegraph/pull/42554))
