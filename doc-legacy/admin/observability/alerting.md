# Alerting

Alerts can be configured to notify site admins when there is something wrong or noteworthy on the Sourcegraph instance.

## Understanding alerts

Alerts fall in one of two severity levels:

- <span class="badge badge-critical">critical</span>: something is _definitively_ wrong with Sourcegraph, in a way that is very likely to be noticeable to users. We suggest using a high-visibility notification channel for these alerts.
  - **Examples:** Database inaccessible, running out of disk space, running out of memory.
  - **Suggested action:** Page a site administrator to investigate.
- <span class="badge badge-warning">warning</span>: something _could_ be wrong with Sourcegraph. We suggest checking in on these periodically, or using a notification channel that will not bother anyone if it is spammed. Over time, as warning alerts become stable and reliable across many Sourcegraph deployments, they will also be promoted to critical alerts in an update by Sourcegraph.
  - **Examples:** High latency, high search timeouts.
  - **Suggested action:** Email a site administrator to investigate and monitor when convenient, and please let us know so that we can improve them.

Refer to the [alert solutions reference](alerts.md) for a complete list of Sourcegraph alerts, as well as possible solutions when these alerts are firing.
Learn more about metrics, dashboards, and alert labels in our [metrics guide](metrics.md).

## Setting up alerting

<span class="badge badge-note">Sourcegraph 3.17+</span>

Visit your site configuration (e.g. `https://sourcegraph.example.com/site-admin/configuration`) to configure alerts using the [`observability.alerts`](../config/site_config.md#observability-alerts) field. As always, you can use `Ctrl+Space` at any time to get hints about allowed fields as well as relevant documentation inside the configuration editor.

Once configured, Sourcegraph alerts will automatically be routed to the appropriate notification channels by severity level.

### Example notifiers

#### Slack

```json
"observability.alerts": [
  {
    "level": "critical",
    "notifier": {
      "type": "slack",
      // Slack incoming webhook URL.
      "url": "https://hooks.slack.com/services/xxxxxxxxx/xxxxxxxxxxx/xxxxxxxxxxxxxxxxxxxxxxxx",
    }
  }
]
```

> NOTE: Learn more about generating a [Slack incoming webhook URL](https://api.slack.com/messaging/webhooks)

#### PagerDuty

```json
"observability.alerts": [
  {
    "level": "critical",
    "notifier": {
      "type": "pagerduty",
      // Integration key for the PagerDuty Events API v2
      "integrationKey": "XXXXXXXX"
    }
  }
]
```

#### Opsgenie

```json
"observability.alerts": [
  {
    "level": "critical",
    "notifier": {
      "type": "opsgenie",
      // Opsgenie API key. This API key can also be set by the environment variable OPSGENIE_API_KEY on the prometheus container. Setting here takes precedence over the environment variable.
      "apiKey": "xxxxxx",
      "responders": [
        {
          "type": "team",
          "name": "my-team"
        }
      ]
    },
    "owners": [
      "my-team",
    ]
  }
]
```

#### Webhook

```json
"observability.alerts": [
  {
    "level": "critical",
    "notifier": {
      "type": "webhook",
      // Webhook URL.
      "url": "https://my.webhook.url"
    }
  }
]
```

Webhook events provide the following fields relevant for Sourcegraph alerts that we recommend you leverage:

<!-- Refer to `commonLabels` on receivers.go for labels we can guarantee will be provided in commonLabels -->

```json
{
  "status": "<resolved|firing>",
  "commonLabels": {
    "level": "<critical|warning>",
    // Use the service name and alert name to find solutions in https://docs.sourcegraph.com/admin/observability/alerts
    "service_name": "<string>",
    "name": "<string>",
    "description": "<string>",
    // This field can be provided to Sourcegraph to help direct support.
    "owner": "<string>"
  },
}
```

For the complete set of fields, please refer to the [Alertmanager webhook documentation](https://prometheus.io/docs/alerting/latest/configuration/#webhook_config).

#### Email

Note that to receive email notifications, the [`email.address`](../config/site_config.md#email-address) and [`email.smtp`](../config/site_config.md#email-smtp) fields must be configured in site configuration.

```json
"observability.alerts": [
  {
    "level": "critical",
    "notifier": {
      "type": "email",
      // Address where alerts will be sent
      "address": "sourcegraph@company.com"
    }
  }
]
```

### Testing alerts

<span class="badge badge-note">Sourcegraph 3.19+</span>

Configured alerts can be tested using the Sourcegraph GraphQL API. Visit your API Console (e.g. `https://sourcegraph.example.com/api/console`) and use the following mutation to trigger an alert:

```gql
mutation {
  triggerObservabilityTestAlert(
    level: "critical"
  ) { alwaysNil }
}
```

The test alert may take up to a minute to fire. The triggered alert will automatically resolve itself as well.

### Silencing alerts

<span class="badge badge-note">Sourcegraph 3.18+</span>

If there is an alert you are aware of and you wish to silence notifications (from the notification channels you have set up) for it, add an entry to the [`observability.silenceAlerts`](../config/site_config.md#observability-silenceAlerts)field. For example:

```json
{
  "observability.silenceAlerts": [
    "warning_gitserver_disk_space_remaining"
  ]
}
```

You can find the appropriate identifier for each alert in [alert solutions](./alerts.md).

> NOTE: You can still see the alerts on your [Grafana dashboard](./metrics.md#grafana).
