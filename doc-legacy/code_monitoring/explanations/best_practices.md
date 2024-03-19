# Best practices

There are some best practices we recommend when creating code monitors.

## Privacy and visibility

### Do not include confidential information in monitor names

Every code monitor has a name that will be shown wherever the monitor is referenced. In notification actions this name is likely to be the only information about the event, so it’s important for identifying what was triggered, but also has to be “safe” to expose in plain text emails.

### Do not include results when the notification destination untrusted

Each code monitor action has the ability to include the result contents when sending a notification. This is often convenient because it lets you immediately see which results triggered the notification. However, because the result contents include the code that matched the search query, they may contain sensitive information. Care should be taken to only send result contents if the destination is secure.

For example, if sending the results to a Slack channel, every user that can view that channel will also be able to view the notification messages. The channel should be properly restricted to users who should be able to view that code.

## Scale

Code monitors have been designed to be performant even for large Sourcegraph instances. There are no hard limits on the number of monitors or the volume of code monitored. However, depending on a number of factors such as the number of code monitors, the number of repos monitored, the frequency of commits, and the resources allocated to your instance, it's still possible to hit soft limits. If this happens, your code monitor will continue to work reliably, but it may execute more infrequently.
