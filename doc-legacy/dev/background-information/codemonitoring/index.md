# Developing code monitoring

## What are code monitors?

In the simplest case, a code monitor runs a user-defined query and alerts the
user whenever a new search result appears. In more general terms, a code monitor
lets a user define one trigger and at least one action. A trigger defines a
condition and if that condition evaluates to true, the trigger triggers the
actions.

## Glossary
| term         | description                                                          | example                             |
|:-------------|:---------------------------------------------------------------------|-------------------------------------|
| code monitor | A code monitor is a group of 1 trigger and possibly multiple actions |                                     |
| trigger      | A trigger defines a condition which is checked regularly.            | New results for a diff/commit query |
| event        | If a condition evaluates to `true` we call this an event.            | New results found                   |
| action       | Events trigger actions.                                              | Send email                          |


## Starting up your environment

To run it locally you need to start Sourcegraph:

```bash
sg start # which defaults to `sg start enterprise`
```

Code monitoring is still experimental which means you have to enable it in the
settings to be visible in the UI. Open your local instance of Sourcegraph and go
to `settings > User account > settings`.

```json
{
  "experimentalFeatures": {
    "codeMonitoring": true
  }
}
```

## Database layout

| Table           | Description                                                                                                                                                   |
|:----------------|:--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| cm_monitors     | Holds metadata of a code monitor.                                                                                                                            |
| cm_queries      | Contains data for each (trigger) query.                                                                                                                       |
| cm_emails       | Contains data for each (action) email.                                                                                                                        |
| cm_recipients   | Each email action can have multiple recipients. Each recipient can either be a user or an organization. Each row in this table corresponds to one reciepient. |
| cm_trigger_jobs | Contains jobs (past, present, future) to run triggers. Trigger jobs are linked to their triggers via a foreign key.                                           |
| cm_actions_jobs | Contains jobs (past, present, future) to run actions. Actions jobs are linked to their action and to the event that triggered them  via foreign keys.         |

Each type of trigger or type of action is represented by its own table in the
database; queries are represented by `cm_queries`, and emails are represented by
`cm_emails` and `cm_recipients`. The job tables (`cm_trigger_jobs` and
`cm_action_jobs`) on the other hand contain the jobs for all types of triggers
and actions. 

For example: Each type of action is represented by a separate nullable column in
`cm_action_jobs`. The dequeue worker reads a record and dispatches based on
which of the columns is filled. The table below shows `cm_action_jobs`with two
jobs enqueued, one for sending emails (id=1) and one for posting to a webhook
(id=2). The details for each action are contained in the records linked to with
the foreign keys in columns `email` and `webhook`.

`cm_action_jobs`

| id | email | webhook | state  |
|:---|:------|:--------|:-------|
| 1  | 1     | null    | queued |
| 2  | null  | 1       | queued |

For more details, see
[schema.md](https://github.com/sourcegraph/sourcegraph/blob/main/internal/database/schema.md).

## Life of a code monitor

Let's follow the life of a code monitor with a query as a trigger and 1 email
action.

1. After you have created a code monitor, the following tables are filled:
    1. cm_monitors (1 entry)
    2. cm_queries (1 entry)
    3. cm_actions (at least 1 entry)
    4. cm_recipients (at least 1 entry)
3. Enqueue trigger: Periodically, a background job enqueues queries, i.e. for each
   active query (column `enabled=true` in `cm_queries`), we create an entry in
   `cm_trigger_jobs`.
4. Dequeue trigger/enqueue actions: Periodically, a background worker dequeues
   the trigger job and processes it. In our case the query is run. The
   `last_result` and `next_run` are logged to `cm_queries`, the `num_results`
   and the `query` are logged to `cm_query_jobs`. If the query returned at least
   1 result, we call it an **event**. For each event the corresponding actions
   are enqueued in `cm_actions_jobs`.
5. Dequeue actions: Periodically, a background worker dequeues the action jobs
   queued in `cm_action_jobs` and processes them. In our cases we retrieve all
   relevant information from `cm_monitors`, `cm_trigger_jobs`, `cm_query`,
   `cm_emails`, `cm_recipients` and send out an email to the recipients.
6. Clean-up: Job logs are deleted after a predefined retention period. Job logs
   without search results, are deleted soon after the trigger jobs ran.

## Architecture

The back end of code monitoring is split into two parts, the GraphQL API, running
on _frontend_, and the background workers, running on _repo-updater_. Both rely
on the
[store](https://github.com/sourcegraph/sourcegraph/blob/main/internal/codemonitors/store.go)
to access the database.

### GraphQL API

The GraphQL API is defined
[here](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/frontend/graphqlbackend/schema.graphql).
The interfaces and stub-resolvers are defined
[here](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/frontend/graphqlbackend/code_monitors.go),
while the resolvers are defined
[here](https://github.com/sourcegraph/sourcegraph/blob/main/internal/codemonitors/resolvers/resolvers.go).

### Background workers

The [background
workers](https://github.com/sourcegraph/sourcegraph/blob/main/internal/codemonitors/background/background.go)
utilize our `internal/workerutil` framework to [run as background jobs on
`repo-updater`](https://github.com/sourcegraph/sourcegraph/blob/main/enterprise/cmd/repo-updater/main.go#L49).

## Diving into the code as a backend developer

1. A good start is to [visualize the GraphQL
   schema](https://github.com/stefanhengl/sg-voyager) and interact with it [via
   the UI Console](https://sourcegraph.com/api/console). Start from the node
   `user` and go to `monitors` from there.
2. Check out the [interfaces and stub
   resolvers](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/frontend/graphqlbackend/code_monitors.go)
   and understand how they relate to the [GraphQL
   schema](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/frontend/graphqlbackend/code_monitors.go).
3. Do the same for the [
   resolvers](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/frontend/graphqlbackend/code_monitors.go).
4. Take a look at the [background
   workers](https://github.com/sourcegraph/sourcegraph/blob/main/internal/codemonitors/background/background.go)
   and look through each of the jobs that run in the background.
5. Start up Sourcegraph locally, connect to your local db instance, create a
   code monitor from the UI and follow its life cycle in the db. Start by
   looking at `cm_queries` and `cm_trigger_jobs`. Depending on the search query
   you defined you might have to wait a long time before the first action is
   enqueued in `cm_action_jobs`. You can accelerate the process by backdating
   columns `last_result` and `next_run` to the past.

        
