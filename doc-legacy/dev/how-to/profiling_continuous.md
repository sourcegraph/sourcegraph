# How to enable continuous profiling in production

GCP supports [continuous CPU and heap profiling](https://cloud.google.com/profiler). We already have it enabled for some services; you can see some flamegraphs in the [GCP profiler dashboard](https://console.cloud.google.com/profiler/frontend/cpu?project=sourcegraph-dev). Turning it for another service requires two small steps:

1. [Initialize the profiler](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/profiler/profiler.go?L12-L31) in the service. ([example PR](https://github.com/sourcegraph/sourcegraph/pull/30681))
2. Set the `SOURCEGRAPHDOTCOM_MODE` environment variable in the [production config](https://github.com/sourcegraph/deploy-sourcegraph-cloud/) to `"true"`. ([example PR](https://github.com/sourcegraph/deploy-sourcegraph-cloud/pull/15540))

Once the new configuration is deployed to production, you should be able to access profiles for your service in the dashboard.

Other resources:

- [GCP profiler documentation](https://cloud.google.com/profiler/docs/)
- [How to do one-off profiling for dogfood and production using pprof](./profiling_one-off.md)
