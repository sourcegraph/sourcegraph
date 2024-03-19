# Analytics

The analytics section helps Sourcegraph administrators understand user engagement across the various Sourcegraph features, identify power users, and convey value to internal leaders. Introduced in version 3.42, the section includes analytics breakdowns for our most common features such as Batch Changes, Search Notebooks and search, while also providing basic user-level analytics. 

## Data Visualizations

The goal of these pages is to help administrators answer any question they might have about how features are being used within their Sourcegraph instance. So far, we have introduced pages for Search, Batch Changes, Code Intel, Search, and Search Notebooks, as well as a general users page. 

Each page can visualize the past one week, the past one month, or the path three months of data. For graphs that show user data, the graph can toggle between total users or unique users.

These graphs pull directly from the event log table within the Sourcegraph instance they are running. There should not be an increase to the storage on disk of these tables due to these new features. Further, no data beyond published ping data is sent back to Sourcegraph. 

## Value Calculators

Each page also includes a total time saved value which can be used to measure the value Sourcegraph is bringing to your organization. This metric is derived from the configurable calculators below the total time saved value. Each calculator multiplies event log data (ex: number of precise code intel events such as a go-to-definition) by a configurable number of minutes saved per event to arrive at a time saved by the feature.

We designed this to be configurable by you because we want to... 
- help admins understand the value that is being seen by their organization today. 
- be customizable so admins can explore what changes will best increase developer time saved.

These calculators exist on the Search, Code Intel, Batch Changes, and Notebooks analytics pages. Each calculator looks different as they include metrics specifically designed for that part of the application. Please note that the calculator configuration does not save and will return to the default if you navigate away from the page.

If you have questions about how this works or about how to convey this value to leaders within your organization, please do not hesitate to reach out to your customer engineer. 

## FAQ 

**Who has access to see these improved analytics? Where can I find it?**

To see these new visualizations, you must be a site admin. You can find these under Site Admin section, under the Analytics section of the left-nav bar. 

**Do these improved analytics require sending data to Sourcegraph?** 

No! The processing happens entirely within a your instance so no data is sent in or out of your instance. Further, these improved analytics leverage data already being captured within the event log table of your instance so there is no additional storage or processing required for this change. Basically, customers should notice no perceivable difference to their infrastructure. 

**How often is the data updated?**

The data is updated approximately every 24 hours. 

**How does this work with the existing usage stats page?**

This new analytics experience has been redesigned from the ground up to provide the most value to administrators. In the future, we plan to deprecate the legacy usage stats page and statistics section once this functionality moved from experimental to generally available. 

Note: The new analytics experience is experimental. For billing information, use [usage stats](./usage_statistics.md).
