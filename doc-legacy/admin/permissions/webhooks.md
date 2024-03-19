# Webhooks for repository permissions

Sourcegraph allows customers to use webhooks to react to events that modify user permissions 
on the code host. Currently the only supported code host for webhooks 
is [Github](../external_service/github.md).

> NOTE: Using webhooks is the *recommended* way to get code host permissions to Sourcegraph

## How it works

Sourcegraph exposes endpoints to receive webhooks. These endpoints are authenticated, so we make sure the requests come from a trusted source.

1. Sourcegraph receives a webhook request with one of the supported events
1. Based on the event data, Sourcegraph schedules a permission sync job
1. Standard permission syncing mechanism handles the scheduled job, leading to a sync of permissions of the relevant user or repository from the code host

## SLA

Sourcegraph SLA is, that **p95 of webhook requests will be processed within 5 minutes**. This means, that 
when the permissions are changed on the code host, it takes at most 5 minutes for the same permissions to be reflected on the Sourcegraph side.

## Advantages

- the eventual consistency time is really low, see [SLA](#sla) above.
- least amount of resource usage (bandwidth, code host rate limit), as we only ask code host for permission data when there is an actual change
## Disadvantages

Webhooks are best effort and there is no 100% guarantee that a webhook will be 
fired from the code host side when the data change. Especially with permissions, this 
is something to be aware of, as important permission related webhooks might 
not be sent to Sourcegraph.

> NOTE: That's why we recommend to use webhooks alongside the permission syncing mechanism.

## Configuring webhooks

Please follow the link for [configuring permission syncing webhooks for Github](../config/webhooks/incoming.md#user-permissions).
