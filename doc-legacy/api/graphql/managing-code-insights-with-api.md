# Managing Code Insights with the API

Learn how to manage [Code Insights](../../code_insights/index.md) on private Sourcegraph instances with the API. If you haven't used the API before, learn more about [the GraphQL API and how to use it](index.md).

This page is meant as a guide for common use cases. You can find all of GraphQL documentation in the [API Console](./index.md#api-console).

## Prerequisites

The Code Insights GraphQL API is available on Sourcegraph versions 3.35.1+.

Note: If Code Insights setting storage is enabled, (`ENABLE_CODE_INSIGHTS_SETTINGS_STORAGE:true`) any changes made via the API will be periodically overwritten by insights stored in settings. This is an unlikely scenario.

## Permissions and visibility

Note: there are no separate read/write permissions at this time, so if a user has permission to view an insight they can also edit and delete it.

When a user creates a Code Insight, that user will automatically be granted permission to view that insight. Besides this one case of insight-level permissions, all permissions exist on [dashboards](../../code_insights/explanations/viewing_code_insights.md#insights-dashboards). This means that in order for another user to view an insight via the API, that insight must first be attached to a dashboard with either organization or global permissions.

See [Managing Dashboards](#managing-dashboards) below for more information.

## Just-in-time vs persisted insights

Some insights generate and persist time series data, while others calculate their data just-in-time on page load. Currently, line chart insights will be persisted, while the language statistics pie charts that run over a single repository will be generated just-in-time.

## Creating a persisted insight

To create a Code Insight that will generate and persist time series data, use the mutation below.

Important: Specify the list of repositories that the insight should run over in `repositoryScope.repositories` or leave it empty to specify that the query should be run across all repositories.

```gql
mutation CreateLineChartSearchInsight($input: LineChartSearchInsightInput!) {
  createLineChartSearchInsight(input: $input) {
    view {
      id
    }
  }
}
```

Example variables:

```json
{
  "input": {
    "options": {
      "title": "Javascript to Typescript migration"
    },
    "dataSeries": [{
      "query": "lang:javascript",
      "options": {
        "label": "javascript",
        "lineColor": "#6495ED"
      },
      "repositoryScope": {
        "repositories": []
      },
      "timeScope": {
        "stepInterval": {
          "unit": "MONTH",
          "value": 1
        }
      }
    },
    {
      "query": "lang:typescript",
      "options": {
        "label": "typescript",
        "lineColor": "#DE3163"
      },
      "repositoryScope": {
        "repositories": []
      },
      "timeScope": {
        "stepInterval": {
          "unit": "MONTH",
          "value": 1
        }
      }
    }]
  }
}
```

## Creating a pie chart insight

Pie chart insights show language usage across a specified repository. Because this type of chart has not yet been generalized to other use cases, the `query` field in the input is not used. To create one, use the mutation below.

```gql
mutation CreatePieChartSearchInsight($input: PieChartSearchInsightInput!) {
  createPieChartSearchInsight(input: $input) {
    view {
      id
    }
  }
}
```

Example variables:

```json
{
  "input": {
    "query": "",
    "repositoryScope": {
      "repositories": ["sourcegraph/sourcegraph"]
    },
    "presentationOptions": {
      "title": "Language usage for Sourcegraph",
      "otherThreshold": 0.3
    }
  }
}
```

## Reading a single Code Insight

Use the query below to read a Code Insight by `id`. `filters` are optional, and if provided will filter the aggregated time series to specific repositories.

You can find an insight's `id` if you visit the edit page for the insight. The edit page URL will be of the form `https://sourcegraph.yourcompany.com/insights/edit/aW5zaWdodF92aWV3OiIyM2hiYzNNclB2bDBtajlLTTZTUlBpWVlhZWQi?dashboardId=all` where the `id` is `aW5zaWdodF92aWV3OiIyM2hiYzNNclB2bDBtajlLTTZTUlBpWVlhZWQi`. Alternatively, you can list all insights using this graphQL API. 

Notes on the return object:

- `dataSeries.status` is useful to guage the progress of the series point generation. More information can be found in the [API Console Documentation](./index.md#api-console)
- The `dataSeries`, `dataSeriesDefinitions` and `seriesPresentation` arrays each store different information about the same series. The `seriesId` field on each can be used to match them up.
  - `dataSeries` contains the calculated time series data
  - `dataSeriesDefinitions` contains the definition for the series, such as the query and time interval
  - `seriesPresentation` contains presentation options such as the title and line color

```gql
query InsightViews($id: ID, $filters: InsightViewFiltersInput) {
  insightViews(id: $id, filters: $filters) {
    nodes {
      id,
      dataSeries {
        label,
        points {
          dateTime,
          value
        },
        status {
          pendingJobs,
          completedJobs,
          failedJobs,
          backfillQueuedAt
        }
      }
      dataSeriesDefinitions {
        ... on SearchInsightDataSeriesDefinition {
          seriesId,
          query,
          repositoryScope {
            repositories
          }
          timeScope {
            ... on InsightIntervalTimeScope {
              unit,
              value
            }
          }
        }
      }
      presentation {
        ... on LineChartInsightViewPresentation {
          title,
          seriesPresentation {
            seriesId,
            label,
            color
          }
        },
        ... on PieChartInsightViewPresentation {
          title,
          otherThreshold
        }
      }
    }
  }
}
```

Example variables:

```json
{ 
  "id": "aW5zaWdodF92aWV3OiIyMkVIR2pXOTFkSzNOanpmM2hyWnU3WDJwMlgi",
  "filters": {
    "includeRepoRegex": "sourcegraph/sourcegraph",
    "excludeRepoRegex": "sourcegraph/handbook"
  } 
}
```

## List Code Insights

The query below will list all of the Code Insights that you can see based on [permissions](#permissions-and-visibility). The query and return object is the same as for reading a single Code Insight. All input parameters are optional, and can be used for cursor-based pagination and repository filtering.

```gql
query InsightViews($first: Int, $after: String, $filters: InsightViewFiltersInput) {
  insightViews(first: $first, after: $after, filters: $filters) {
    nodes {
      id
    },
    pageInfo {
      endCursor,
      hasNextPage
    }
  }
}
```

Example variables:

```json
{
  "first": 10,
  "after": "aW5zaWdodF92aWV3OiIyM1l3WHpmSkVhY2Juc0RvWVE5N0FtRU9Wbkki"
}
```

## Updating a Code Insight

Below is a GraphQL mutation that updates an existing Code Insight. The format is almost identical to the creation mutation, except that it now takes an `id`. The input object must be complete as it will completely overwrite the existing Code Insight.

Important: series are added, updated, and deleted from the insight as follows:

- A `dataSeries` without a `seriesId` or with an unknown `seriesId` will be treated as a new series and be added to the insight.
- A `dataSeries` with a `seriesId` that already exists for this insight will be updated in place.
- Any `dataSeries` that previously existed on the insight but are NOT included in the update **will be deleted.**

```gql
mutation UpdateLineChartSearchInsight($id: ID!, $input: UpdateLineChartSearchInsightInput!) {
  updateLineChartSearchInsight(id: $id, input: $input) {
    view {
      id
    }
  }
}
```

Example variables:

This is an example of updating the Code Insight from the creation mutation example. This updates `options.title`, the `dataSeries.timeScope.stepInterval` of the existing javascript series, and deletes the typescript series by omiting it.

```json
{
  "id": "[INSIGHT_ID]",
  "input": {
    "presentationOptions": {
      "title": "Javascript weekly"
    },
    "viewControls": {
      "seriesDisplayOptions": {},
      "filters": {}
    },
    "dataSeries": [
      {
        "seriesId": "[SERIES_ID]",
        "query": "lang:javascript",
        "options": {
          "label": "javascript",
          "lineColor": "#6495ED"
        },
        "repositoryScope": {
          "repositories": []
        },
        "timeScope": {
          "stepInterval": {
            "unit": "WEEK",
            "value": 1
          }
        }
      }
    ]
  }
}
```

## Deleting a Code Insight

Below is a GraphQL mutation that deletes a Code Insight by ID.

```gql
mutation DeleteInsightView($id: ID!) {
  deleteInsightView(id: $id) {
    alwaysNil
  }
}
```

Example variables:

```json
{ 
  "id": "aW5zaWdodF92aWV3OiIyMkVIR2pXOTFkSzNOanpmM2hyWnU3WDJwMlgi" 
}
```

## Managing dashboards

### Creating a dashboard

Below is a GraphQL mutation that creates a dashboard with global permissions, meaning all users can view this dashboard and all of its insights.

```gql
mutation CreateInsightsDashboard($input: CreateInsightsDashboardInput!) {
  createInsightsDashboard(input: $input) {
    dashboard { 
      id
    }
  }
}
```

Example variables:

```json
{ 
  "input": {
    "title": "Global insights dashboard",
    "grants": {
      "users": [],
      "organizations": [],
      "global": true
    }
  }
}
```

### Adding and removing Code Insights from a dashboard

Use the following mutations to add and remove insights from dashboards:

```gql
mutation AddInsightViewToDashboard($input: AddInsightViewToDashboardInput!) {
  addInsightViewToDashboard(input: $input) {
    dashboard { 
      id
    }
  }
}
```

```gql
mutation RemoveInsightViewFromDashboard($input: RemoveInsightViewFromDashboardInput!) {
  removeInsightViewFromDashboard(input: $input) {
    dashboard { 
      id
    }
  }
}
```

Example variables:

```json
{ 
  "input": {
    "insightViewId": "aW5zaWdodF92aWV3OiIyMkVIR2pXOTFkSzNOanpmM2hyWnU3WDJwMlgi",
    "dashboardId": "ZGFzaGJvYXJkOnsiSWRUeXBlIjoiY3VzdG9tIiwiQXJnIjoxNDV9"
  }
}
```
