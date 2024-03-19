# Set up local Sourcegraph monitoring development

This guide documents how to spin up and develop Sourcegraph's monitoring stack locally.
Sourcegraph employees should also refer to the [handbook's monitoring section](https://handbook.sourcegraph.com/engineering/observability/monitoring) for Sourcegraph-specific documentation.
The [developing observability page](../background-information/observability/index.md) contains relevant documentation as well, including background about the components listed here.

> NOTE: For how to *use* Sourcegraph's observability and an overview of our observability features, refer to the [observability for site administrators documentation](../../admin/observability/index.md).

## Running monitoring components

### With all services

The monitoring stack is not included in `sg start` (or `sg start enterprise`) scripts.
It needs to be started separately with `sg start monitoring`.
Learn more about these in the [general development getting started guide](../setup/index.md).

### Without all services

For convenience, there are a number of ways to spin up Sourcegraph's monitoring services *without* having to start up every other service as well.

You can follow the instructions below for spinning up individual monitoring components, or use one of the following:

- `sg start monitoring`: Spin up just monitoring components
- `sg start monitoring-alerts`: Spin up frontend components as well as some monitoring components to test out the [alerting integration](../../../admin/observability/alerting.md#setting-up-alerting).

#### Grafana

Running just Grafana is a convenient way to validate dashboards.

When doing so, you may wish to connect Grafana to a remote Prometheus instance that you have administrator access to (such as [Sourcegraph's instances](https://handbook.sourcegraph.com/engineering/deployments/instances)), to show more real data than is available on your dev server.

For Kubernetes deployments, you can accomplish this by creating a [`sg.config.overwrite.yaml` file](../background-information/sg/index.md#Configuration) that replaces your local Prometheus instance with a `kubectl` command that port-forwards traffic from the Prometheus service on the Kubernetes cluster that you're currently connected to:

```yaml
# sg.config.overwrite.yaml
commands:
  prometheus:
    # install can just be set up gcloud credentials for a cluster
    # e.g. https://handbook.sourcegraph.com/departments/product-engineering/engineering/process/deployments/instances
    install: gcloud container clusters get-credentials ...
    # make remote prometheus accessible to local grafana
    cmd: kubectl port-forward svc/prometheus 9090:30090
  monitoring-generator:
    env:
      # don't reload your production Prometheus!
      PROMETHEUS_DIR: ''
```

Then, you can start up the local dev monitoring stack by using:

```sh
sg start monitoring
```

Grafana dashboards will be available at `localhost:3370`.

> NOTE:  If you are on Linux and are also running [ufw](https://wiki.archlinux.org/title/Uncomplicated_Firewall) for your firewall, the Grafana dashboard might show a `Bad gateway` error. Although not recommended, disabling the firewall is a quick hack to make this work but it should be possible to get `ufw` to play along with `docker` nicely with some research (not covered in this document).

Note that instead of `kubectl`, you can replace the command in the `sg.config.overwrite.yaml` above to use whichever port-forwarding mechanism you wish to use to connect to a remote Prometheus instance (as long as Prometheus is available on port `9090` locally).
The dev targets for Grafana are defined in the following files:

* Non-Linux: [`dev/grafana/all/datasources.yaml`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/grafana/all/datasources.yaml)
* Linux: [`dev/grafana/linux/datasources.yaml`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/grafana/linux/datasources.yaml)

#### Prometheus

Running just Prometheus is a convenient way to validate the generated recording and alert rules.
You can start up a standalone Prometheus using:

```sh
sg run prometheus
```

The loaded generated recording and alert rules are available at `http://localhost:9090/rules`.
The bundled Alertmanager is available at `http://localhost:9090/alertmanager`.

Some configuration options are available:

* `DISABLE_SOURCEGRAPH_CONFIG`: when `true`, disables the prom-wrapper's [integration with the Sourcegraph frontend](#frontend-integration).
* `DISABLE_ALERTMANAGER`: when `true`, disables the bundled Alertmanager entirely.
  This includes the behaviour of `DISABLE_SOURCEGRAPH_CONFIG=true`.

Note that without services to scrape, running a standalone Prometheus will not provide any metrics—if you need to test metrics as well, you should [start all services](#with-all-services) instead.
The dev targets for Prometheus are defined in the following files:

* Non-Linux: [`dev/prometheus/all/prometheus_targets.yml`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/prometheus/all/prometheus_targets.yml)
* Linux: [`dev/prometheus/linux/prometheus_targets.yml`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/prometheus/linux/prometheus_targets.yml)

##### Frontend integration

The Sourcegraph Prometheus service features an integration with the Sourcegraph frontend that requires a frontend instance to be running to develop or test these features.
Note that the Prometheus service will still run without additional configuration even if no frontend is accessible.

One way to do this is to [start up Prometheus alongside all Sourcegraph services](#with-all-services).
You can alternatively spin up just the frontend separately:

```sh
sg run enterprise-frontend # or: sg run frontend
```

This should be sufficient to access the frontend API and the admin console (`/site-admin`), which is where most of the integration is.

#### Docsite

The docsite is used to serve generated monitoring documentation, such as the [alert solutions reference](../../../admin/observability/alerts.md).
You can spin it up by running:

```sh
sg run docsite
```

Learn more about docsite development in the [product documentation implementation guide](documentation_implementation.md).

## Using the monitoring generator

> NOTE: Looking to add monitoring first? Refer to the [how to add monitoring](add_monitoring.md) guide!

The dev startup scripts used in this guide all mount relevant configuration directories into each monitoring service.
This means that you can:

* Update your monitoring definitions
* Run the generator to regenerate and reload monitoring services
* Validate the result of your changes immediately (for example, by checking Prometheus rules in `/rules` or Grafana dashboards in `/-/debug/grafana`)

To run the generator and trigger a reload on changes:

```sh
sg run monitoring-generator
```

Make sure to provide the following parameters as well, where relevant:

* `GRAFANA_DIR=''`, if you are *not* running Grafana
* `PROMETHEUS_DIR=''`, if you are *not* running Prometheus
* `SRC_LOG_LEVEL=dbug` to enable potentially helpful output for debugging issues
