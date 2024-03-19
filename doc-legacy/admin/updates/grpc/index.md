# Sourcegraph 5.2 gRPC Configuration Guide

## Overview

As part of our continuous effort to enhance performance and reliability, in Sourcegraph 5.2 we’re introducing [gRPC](https://grpc.io/) as the primary communication method for our internal services. 

This guide will help you understand this change and its implications for your setup.

## Quick Overview

- **What’s changing?** We're transitioning to [gRPC](https://grpc.io/) for internal communication between Sourcegraph services.
- **Why?** gRPC, a high-performance Remote Procedure Call (RPC) framework by Google, brings about several benefits like a more efficient serialization format, faster speeds, and a better development experience.
- **Is any action needed?** If you don’t have restrictions on Sourcegraph’s **internal** (service to service) traffic, you shouldn't need to take any action—the change should be invisible. If you do have restrictions, some firewall or security configurations may be necessary. See the ["Who needs to Act"](#who-needs-to-act) section for more details.
- **Can I disable gRPC if something goes wrong?** In Sourcegraph version `5.2.X`, there's an option to [disable gRPC](#sourcegraph-52x-enabling--disabling-grpc) during this initial rollout. This option will be removed in `5.3.X`.

## gRPC: A Brief Intro

[gRPC](https://grpc.io/) is an open-source [RPC](https://en.wikipedia.org/wiki/Remote_procedure_call) framework developed by Google. Compared to [REST](https://en.wikipedia.org/wiki/REST), gRPC is faster, more efficient, has built-in backwards compatibility support, and offers a superior development experience.

## Key Changes

### 1. Internal Service Communication

From version `5.2.X` onwards, our microservices like `repo-updater` and `gitserver` will use mainly gRPC instead of REST for their internal traffic. This affects only communication *between* our services. Interactions you have with Sourcegraph's UI and external APIs remain unchanged.

### 2. Rollout Plan

| Version                         | gRPC Status                                                              |
|---------------------------------|--------------------------------------------------------------------------|
| `5.2.X` (Releasing October 4th, 2023) | On by default but can be disabled via a feature flag.               |
| `5.3.X` (Releasing December 14th, 2023)| Fully integrated and can't be turned off.                             |

## Preparing for the Change

### Who Needs to Act?

Our use of gRPC only affects traffic **_between_** our microservices (e.x. `searcher` ↔ `gitserver`). Traffic between the Sourcegraph Web UI and the rest of the application is unaffected (e.x. `sourcegraph.example.com` ↔ `frontend`’s GraphQL API).

**If Sourcegraph's internal traffic faces no security restrictions in your environment, no action is required.**

However, if you’ve applied security measures or have firewall restrictions on this traffic, adjustments might be needed to accommodate gRPC communication. The following is a more technical description of the protocol that can help you configure your security settings:

#### gRPC Technical Details

- **Protocol Description**: gRPC runs on-top of [HTTP/2](https://en.wikipedia.org/wiki/HTTP/2) (which, in turn, runs on top of [TCP](https://en.wikipedia.org/wiki/Transmission_Control_Protocol). It transfers (binary-encoded, not human-readable plain-text) [Protocol Buffer](https://protobuf.dev/) payloads. Our current gRPC implementation does not use any encryption.

- **List of services**: The following services will now _speak mainly gRPC in addition_ to their previous traffic:
  - [frontend](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/base/frontend/sourcegraph-frontend.Service.yaml)
  - [gitserver](https://github.com/sourcegraph/deploy-sourcegraph-cloud/blob/release/base/gitserver/gitserver.Service.yaml)
  - [zoekt-webserver](https://github.com/sourcegraph/deploy-sourcegraph-cloud/blob/release/base/indexed-search/indexed-search.StatefulSet.yaml)
  - [zoekt-indexserver](https://github.com/sourcegraph/deploy-sourcegraph-cloud/blob/release/base/indexed-search/indexed-search.StatefulSet.yaml)
  - [symbols](https://github.com/sourcegraph/deploy-sourcegraph-cloud/blob/release/base/symbols/symbols.Deployment.yaml)
  - [repo-updater](https://github.com/sourcegraph/deploy-sourcegraph-cloud/blob/release/base/repo-updater/repo-updater.Deployment.yaml)
  
- The following aspects about Sourcegraph’s networking configuration **aren’t changing**:
  - **Ports**: all Sourcegraph services will use the same ports as they were in the **5.1.X** release.
  - **External traffic**: gRPC only affects how Sourcegraph’s microservices communicate amongst themselves - **no new external traffic is sent via gRPC**.
  - **Service dependencies:** each Sourcegraph service will communicate with the same set of services regardless of whether gRPC is enabled.
    - Example: `searcher` will still need to communicate with `gitserver` to fetch repository data. Whether gRPC is enabled doesn’t matter.

#### Sourcegraph `5.2.X`: enabling / disabling GRPC

In the `5.2.x` release, you are able to use the following methods to enable / disable gRPC if a problem occurs.

_Note: In the `5.3.X` release, these options will be removed and gRPC will always be enabled. See “Rollout timeline” above for more details_

### All services besides `zoekt-indexserver`

Disabling gRPC on any service that is not `zoekt-indexserver` can be done by one of these options:

#### Option 1: disable via site-configuration

Set the `enableGRPC` experimental feature to `false` in the site configuration file:

```json
{
  "experimentalFeatures": {
    "enableGRPC": false // disabled
  }
}
```

#### Option 2: disable via environment variables

Set the environment variable `SG_FEATURE_FLAG_GRPC="false"` for every service.

### `zoekt-indexserver` service: disable via environment variable

Set the environment variable `GRPC_ENABLED="false"` on the `zoekt-indexserver` container. (See [https://github.com/sourcegraph/deploy-sourcegraph-cloud/blob/18e5f9e450878705b7a99ee7c3bcf74c3fb68514/base/indexed-search/indexed-search.StatefulSet.yaml#L105-L106](https://github.com/sourcegraph/deploy-sourcegraph-cloud/blob/18e5f9e450878705b7a99ee7c3bcf74c3fb68514/base/indexed-search/indexed-search.StatefulSet.yaml#L105-L106) for an example:

```yaml
- name: zoekt-indexserver
  env:
    - name: GRPC_ENABLED
      value: 'false'
  image: docker.io/sourcegraph/search-indexer:5.2.0
```

_zoekt-indexserver can’t read from Sourcegraph’s site configuration, so we can only use environment variables to communicate this setting._

If any issues arise with gRPC, admins have the option to disable it in version `5.2.X`. This will be phased out in `5.3.X`.

For disabling gRPC:

- **General Services**: You can toggle gRPC through site configuration or environment variables. For a smooth experience, ensure a consistent setting across all services.

- **`zoekt-indexserver`**: Adjust the environment variable on the `zoekt-indexserver` container. This service relies solely on environment variables due to its inability to access Sourcegraph’s site configuration.

For detailed instructions, please refer to the “enabling/disabling gRPC” section.

## Monitoring gRPC

To ensure the smooth operation of gRPC, we offer:

- **gRPC Grafana Dashboards**: For every gRPC service, we provide dedicated dashboards. These boards present request and error rates for every method, aiding in performance tracking. See our [dashboard documentation](../../observability/dashboards.md).

- **Internal Error Reporter**: For certain errors specifically from gRPC libraries or configurations, we've integrated an "internal error" reporter. Logs prefixed with `grpc.internal.error.reporter` signal issues with our gRPC execution and should be reported to customer support for more assistance.

## Need Help?

For any queries or concerns, reach out to our customer support team. We’re here to assist!
