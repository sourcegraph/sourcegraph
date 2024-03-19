# Code Insights data retention

> NOTE: This reference is only relevant from the 4.5 release.

On creation a Code Insight will show you 12 data points per series. 
Your Code Insight will then get an additional ephemeral data point daily, and a persisted additional data point at every interval that was specified on insight creation.

Prior to release 4.5, this growth was unbound. 
From 4.5, the oldest data points will be truncated and stored separately according to the sample size specified in the site configuration with the setting shown below.
This means that if you have an insight with 50 data points, and a maximum sample size of 30, the oldest 20 points will be truncated and archived in a separate table.

```json
{
  "insights.maximumSampleSize": 30 
}
```

The sample size value is set to 30 by default and can only be increased to a maximum of 90.  

A background routine will periodically go over Code Insights and move the oldest data points to a separate table. 
If you want to access this data, you are given the option to export all data for a Code Insight, which will include archived data.

> IMPORTANT: Your data is never deleted. It is only moved to a separate database table.

## Data exporting

You can download all data for a Code Insight, including data that has been archived. You can do this:

- From the insight card menu
- From the standalone page
- By curling the API endpoint 

```shell 
curl \
-H 'Authorization: token {SOURCEGRAPH_TOKEN}' \
https://yourinstance.sourcegraph.com/.api/insights/export/{YOUR_INSIGHT_ID} -O -J
```

The data will be exported as a CSV file. 
Only data that you are permitted to see will be excluded (i.e. repository permissions are enforced).

If you have filtered your Code Insight using repository filters or a search context, the data exported will be filtered according to those.

## Dynamic filtering

The option now exists on Code Insights filters to limit the number of samples loaded per series.
There is one setting per insight that applies to all series on that insight.
Adjusting this setting will only apply to one insight and will not have any impact on stored Code Insights data.

## Benefits

- Code Insights should be faster to load.
- Code Insights with a lot of data points that were previously hard to read or hover over will now be more legible.
- Code Insights data can now be exported in CSV format.

## Accessing this feature prior to 4.5

You can enable this retention procedure from Sourcegraph *4.4* if you are a site admin with the following setting:

```json
{
  "experimentalFeatures": {
    "insightsDataRetention": true
  }
}
```

You will however **only be able to export all the code insights data from the 4.5 release**, so use the experimental version at your own risk.