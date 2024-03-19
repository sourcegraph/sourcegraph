# How to export search results

You can export search results to a CSV file by pressing the 'Export results' button in the `Actions` menu above the search results.

For versions before 4.0, view the legacy docs at [sourcegraph/sourcegraph-search-export](https://github.com/sourcegraph/sourcegraph-search-export#sourcegraph-search-results-csv-export-extension)

## FAQs

#### The exported results do not match the results page
Before Sourcegraph 4.4.0, the search result export feature used the GraphQL API with result limits for fast resolution. Add `count:all` to your query to run an exhaustive search.

#### The number of exported results does not match the number of results displayed on Sourcegraph
This is expected, as all instances that match for a single file will be listed in the same entry column under the Search matches row. 

#### Network Error when downloading CSV
Check the browser's dev tools network tab for more details on the specific network error. The CSV file likely exceeds the browser's limit for data URI size. You can limit the size of search results exported with the `count:` filter in the search query.