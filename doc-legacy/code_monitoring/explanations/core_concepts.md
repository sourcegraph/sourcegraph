# Core concepts

Code monitors allow you to keep track of and get notified about changes in your code. Some use cases for code monitors include getting notifications for potential secrets, anti-patterns, or common typos committed to your codebase.

Code monitors are made up of two main elements: **Triggers** and **Actions**.

## Triggers

A _trigger_ is an event which causes execution of an action. Currently, code monitoring supports one kind of trigger: "When new search results are detected" for a particular search query. When creating a code monitor, users will be asked to specify a query as part of the trigger.

Sourcegraph will run the search query over every new commit for the searched repositories, and when new results for the query are detected, a trigger event is emitted. In response to the trigger event, any _actions_ attached to the code monitor will be executed.

**Query requirements**

A query used in a "When new search results are detected" trigger must be a diff or commit search. In other words, the query must contain `type:commit` or `type:diff`. This allows Sourcegraph to detect new search results periodically.

## Actions

An _action_ is executed in response to a trigger event. Currently, code monitoring supports three different actions:

* Sending a notification email to the owner of the code monitor
* <span class="badge badge-beta">Beta</span> Sending a Slack message to a preconfigured channel
* <span class="badge badge-beta">Beta</span> Sending a webhook event to an endpoint of your choosing

## Current flow

To put it all together, a code monitor has a flow similar to the following: 

A user creates a code monitor, which consists of: 

  * a name for the monitor
  * a trigger, which consists of a search query to run periodically,
  * and an action, which is sending an email, sending a Slack message, or sending a webhook event

Sourcegraph runs the query periodically over new commits. When new results are detected, a notification will be sent with the configured action. It will either contain a link to the search that provided new results, or if the "Include results" setting is enabled, it will include the result contents.
