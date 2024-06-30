# Appliance development

This is currently focused on the Kubernetes resource reconciler (a.k.a. an
"operator"), which is the backend of the appliance. The appliance frontend is
in early development.

## Architecture overview

The appliance frontend and reconciler backend communicate via the Sourcegraph
ConfigMap. The frontend drives the reconciler via changes to that ConfigMap.

## Adding a new service

Note: the term "service" is overloaded. In this context, it refers to a
Sourcegraph component, e.g. blobstore, or repo-updater. Sometimes you will see
the word "component" used instead.

The next 4 sections aim to act as a rough quickstart. Please also look at recent
commits that touch this directory, and speak to authors if you like! We
recommend reading all 3 sections up-front, and re-shuffling / blending all 4
activities while developing your service config. For example, you could sketch
out config in a test, before you start writing your reconciler, then jump back
and forth between the two.

### Config

First, add a config element to SourcegraphSpec in spec.go (or modify an existing
one). Consider "how unique" this service is, and whether it could benefit from
embedding StandardConfig. Take a look at this struct, and the corresponding
StandardComponent interface, in the `config` subpackage.

By embedding StandardConfig, you'll get various features for almost-free:

- The ability to disable the service, which allows the use of
  `reconcileObject()`. This frees the developer from writing any upsert/delete
  logic at all, usually.
- Container resources
- Node selectors, tolerations, and affinities.
- Image pull secrets for use with private image registries.
- Service account annotations
  - This is an extremely common customization need, e.g. to enable GKE
    workload-identity bindings.
- Adding standard prometheus scrape directive annotations to Services (k8s
  v1.Service - there's that term overloading again).

The idea is that most services have a lot in common, and by making these aspects
framework-supported, we reduce the probability of errors. There's a big pile of
config to write, and errors can be like needles in a haystack.

Remember, everything is permanently WIP, never finished. It's possible some of
these standard features may be incomplete. Write tests, and inspect your golden
fixtures closely - did container resources propagate as expected? Do the
prometheus annotations appear? Fix / add standard features as required.

If adding or changing a standard feature, use the test suite in
`standard_config_test.go`. You can pick any arbitrary service that happens to
use this feature to test it.

### Reconciler

`reconcile.go` is the entrypoint to start at. The aim is to write a function
that converges the cluster onto some desired state based on the config input.
See the `reconcileXYZ` (e.g. `reconcileBlobstore`). Follow the patterns other
services use, to create/update/delete Kubernetes objects required for your
service.

`kubernetes.go` contains some generic functions for creating, updating, and
deleting kubernetes objects. It's worth having a quick look at these, and seeing
how other services make use of them.

### Golden tests

This package makes use of `kubernetes/controller-runtime/envtest` and the
concept of [golden fixtures](https://ro-che.info/articles/2017-12-04-golden-tests)
to examine the set of resources that our reconciler will deploy to a Kubernetes
API server in response to some config input.

At the time of writing, `testify/suite` is used to start the kube-apiserver and
etcd once, before all tests run, and stop it at the end. See `blobstore_test.go`
for an example of an existing service that makes use of golden tests.

See the "Running tests" section below for more info on generating golden
fixtures and running the tests.

### That's a lot of YAML, how do I even know this is what I want?

The `dev/compare-helm` subpackage contains a developer utility that allows you
to compare a golden fixture with the resources output by our Helm chart for the
same service. See the README in that package for more details.

We don't want to produce a 1:1 replica of Helm - with the appliance, we may even
remove customization that is not useful, and add customization that is - but the
diff is a useful guide.

Become familiar with the templates in `deploy-sourcegraph-helm` that pertain to
your service, and also the general helpers and values that feed into
if-statements and loops in those templates. You may want to pass values to Helm
(see -helm-template-extra-args docs in the README), and add corresponding
customization controls in your config element in the appliance.

## Running tests

To (re)generate golden fixtures, pass the following argument to `go test`:

```
go test -args appliance-update-golden-files
```

In order to run `go test` (with or without arguments), you must have
`setup-envtest` available:

```
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
```

This is not a requirement for the Bazel environment (`bazel test
:appliance_test` in this module).

## Useful Scripts

### Patch all test fixtures
```
find internal/appliance/reconciler/testdata/sg -name '*.yaml'  | xargs sed -Ei '/pgsql:/i\  openTelemetry:\n    disabled: true\n'
```
Example of appending a new element, openTelemetry , after pgsql
