# Audit Log

## Philosophy

The audit log will capture all critical events that affect entities of interest within Sourcegraph services. The audit log is built on top of our [logging standard](https://docs.sourcegraph.com/dev/how-to/add_logging), using structured logs as the base building block. Every captured entry is aligned with the following design mantra:

> Actor takes action on an entity within a context

Here's a sample audit log entry:

```
{
  "SeverityText": "INFO",
  "Timestamp": 1667210919544146000,
  "InstrumentationScope": "server.SecurityEvents",
  "Caller": "audit/audit.go:43",
  "Function": "github.com/sourcegraph/sourcegraph/internal/audit.Log",
  "Body": "AccessGranted (sampling immunity token: 7aacf0e8-d001-4aec-8b7d-20e46d34c8db)",
  "Resource": {
    "service.name": "frontend",
    "service.version": "0.0.0+dev",
    "service.instance.id": "Michals-MacBook-Pro-2.local"
  },
  "Attributes": {
    "audit": {
      "auditId": "7aacf0e8-d001-4aec-8b7d-20e46d34c8db",
      "entity": "security events",
      "actor": {
        "actorUID": "1",
        "ip": "127.0.0.1",
        "X-Forwarded-For": "127.0.0.1, 127.0.0.1"
      }
    },
    "event": {
      "URL": "",
      "source": "BACKEND",
      "argument": "{\"resource\":\"db.repo\",\"service\":\"frontend\",\"repo_ids\":[9]}",
      "version": "0.0.0+dev",
      "timestamp": "2022-10-31 10:08:39.542876 +0000 UTC"
    }
  }
}
```

Here's a word-by-word breakout to demonstrate how the captured entry aligns with the design mantra:

- **Actor** - `Attributes.audit.actor` field carries essential information about the actor who performed the action.
- **Action** - `Body` field carries the action description. This action is suffixed with a "sampling immunity token," which carries the unique audit log entry ID. The audit entry ID must be present in the `Body` so that the message is always unique and never gets dropped by the sampling mechanism (hence the sampling immunity token string).
- **Entity** - `Attributes.audit.entity` describes the audited entity. `Resource` field contains additional information about the audited resource as well.
- **Context** - Any non-`audit` child node of `Attributes`. This is represented by the `event` node in the example above.

### What is audited?

- [Security events](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/database/security_event_logs.go?L120-131)
- [Gitserver access](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/gitserver/internal/accesslog/accesslog.go?L100-104)
- [GraphQL requests](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/frontend/internal/httpapi/graphql.go?L226-244)

This list is expected to grow in the future.

### Target audience

Security specialists. We expect these to ingest the logs into their SIEM tools and define alert policies as they see fit. Site admins are currently not the target audience, but we'll likely offer an easy-to-use in-app audit log.

## Configuring

The audit log is currently configured using the site config. Here's the corresponding entry:

```
  "log": {
    "auditLog": {
      "internalTraffic": false,
      "graphQL": false,
      "gitserverAccess": false,
      "severityLevel": "INFO" // DEPRECATED, defaults to SRC_LOG_LEVEL
    }
    "securityEventLog": {
     "location": "auditlog" // option to set "database" or "all" as well, default to outputing as an audit log
  }
```

We believe the individual settings are self-explanatory, but here are a couple of notes:

- `securityEventLog` configures the destination of security events, logging to the database may result in performance issues
- `internalTraffic` is disabled by default and will result in security events from internal traffic not being logged

## Using

Audit logs are structured logs. As long as one can ingest logs, we assume one can also ingest audit logs.

### On Premises

There are two easy approaches to filtering the audit logs:

- JSON-based: look for the presence of the `Attributes.audit` node. Do not depend on the log level, as it can change based on `SRC_LOG_LEVEL`.
- Message-based: we recommend going the JSON route, but if there's no easy way of parsing JSON using your SIEM or data processing stack, you can filter based on the following string: `auditId`.

### Cloud
[Cloud](../cloud/index.md#audit-logs)


Audit Logs are a default feature for Cloud instances, with a standard retention policy of 30 days. Should you wish to
extend this period, please be aware that additional charges will apply. To request an extension, please contact
your assigned Customer Engineer (CE) or send an email to Sourcegraph Support at support@sourcegraph.com.

For requesting audit logs, please follow the above steps and contact your assigned Sourcegraph representative or our support team.


## Developing

The single entry point to the audit logging API is made via the [`audit.Log`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/audit/audit.go?L19) function. This internal function can be used from any place in the app, and nothing else needs to be done for the logged entry to appear in the audit log.

Example call:
```
audit.Log(ctx, logger, audit.Record{
  Entity: "security events",
  Action: string(event.Name),
  Fields: []log.Field{
    log.Object("event",
      log.String("URL", event.URL),
    ),
  },
})
```
- audit log checks the current settings via the cached `schema.SiteConfiguration`
- `ctx` parameter is required for acquiring `actor.Actor` and `requestclient.Client`
- `logger` parameter is used for performing the actual log call
- `audit.Record` carries all the information required for constructing a valid audit log entry

## FAQ

**How do I map actor ID to the Sourcegraph user?**

The `audit.actor` node carries ID of the user who performed the action (`actorUID`), but it’s not mapped into a full Sourcegraph user right now. You can, however, obtain the user details by following these steps:

1. Grab the user ID from the audit log
1. Base64 [encode](https://www.base64encode.org) the ID with a "User:" prefix. For example, for Actor with ID 71 use `User:71`, which encodes to `VXNlcjo3MQ==`
1. Navigate to Site Admin -> API Console and run the query below
1. Find the corresponding user by searching the query results for the encoded ID from above

GraphQL query:
```
{
  users {
    nodes {
      id
      username
    }
  }
}
```

### Excessive audit logging

If you are seeing a large number of logs in the format `frontend.SecurityEvents` or similar, these are securityEventLogs.

To disable them, in the site config set `log.securityEventLog.location` to `none`.

```json
 "log": {
    "securityEventLog": {
      "location": "none"
    }
}
```
