# Incomplete data points

There are a few cases when a Code Insight may return incomplete data. 

In all of these cases, if data is returned at all, it will be an undercount. 

See the below situations for tips on avoiding and troubleshooting these errors. 

## Timeout errors

For searches that take a long time to complete, it's possible for them to timeout before the search ends, and before we can record the data value. 

To address this, try to minimize or avoid cases where your insight data series: 

1. Runs over extra large sets of repositories (scope your insight further to fewer repositories)
1. Uses many boolean combinations (consider splitting into multiple data series)
1. Runs a complex search over an especially large monorepo (consider optimizing your search query to be more specific or include more filters, like `lang:` or `file:`)

In addition: 

1. Timeout errors on points that have been backfilled before the creation date of the insight are more likely to occur on single, large repositories, because backfill points are calculated by running many searches, repository by repository, individually. 
1. Timeout errors on points that have been snapshot after the creation date of the insight are more likely to occur on insights running complex searches over large numbers of repositories, because snapshot points are calculated by running a single global search against the current index.
You can read more about this case in our [limitations](../explanations/current_limitations_of_code_insights.md#accuracy-considerations-for-an-insight-query-returning-a-large-result-set).

If the data is available, the error alert will inform you which times the search has timed out.

## Other errors

For other errors, please reach out to our support team through your usual channels or at support@sourcegraph.com. 
