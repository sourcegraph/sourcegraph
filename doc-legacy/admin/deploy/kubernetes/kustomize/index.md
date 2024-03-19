# Kustomize for Sourcegraph

An introduction to Kustomize created for Sourcegraph.

<div class="getting-started">
  <a class="btn text-center" href="../index">Installation</a>
  <a class="btn btn-primary text-center" href="#">★ Introduction</a>
  <a class="btn text-center" href="../configure">Configuration</a>
  <a class="btn text-center" href="../operations">Maintenance</a>
</div>

## Overview

 Kustomize enables us to decompose our **[base](#base)** application into smaller building blocks, with multiple versions of each block preconfigured as **[components](#components)** for various use cases. This modular approach enables the mixing and matching of the building blocks to construct a customized version of the application by creating **[overlays](#overlay)**. This feature provides a high degree of flexibility and facilitates the maintenance and evolution of the application over time.

 ## Quick Start

 To deploy Sourcegraph into the `ns-sourcegraph`:

```bash
$ kubectl apply --prune -l deploy=sourcegraph -k https://github.com/sourcegraph/deploy-sourcegraph-k8s/examples/base/xs?ref=v4.5.1
```

## Build process

During the build process, Kustomize will:

1. First build the resources from the base layer of the application.
2. If generators are used, it will then create ConfigMaps and Secrets. These resources can be generated from files, or from data stored in ConfigMaps, or from image metadata.
3. Next, Kustomize will apply patches specified by the components to selectively overwrite resources in the base layer. Patching allows you to modify the resources defined in the base layer without changing the original source files. This is useful for making small, targeted changes to the resources that are needed for your specific deployment.
4. Finally, Kustomize will perform validation to ensure that the modified resources are valid and conform to the Kubernetes API. This is to ensure that the customized deployment is ready for use.
   
Once the validation is passed, the modified resources are grouped into a single file, known as the output. After that, you can use kubectl to apply the overlaid resources to your cluster.

## Deployment repository

You can find all the configuration files and components needed to deploy Sourcegraph with Kustomize in the [deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s) repository.

Here is the file structure:

```bash
# github.com/sourcegraph/deploy-sourcegraph-k8s
├── base
│   ├── sourcegraph
│   └── monitoring
├── components
├── examples
└── instances
    └── template
        └── buildConfig.template.yaml
        └── kustomization.template.yaml

```

> WARNING: Please create your own sets of overlays within the 'instances' directory and refrain from making changes to the other directories to prevent potential merge conflicts during future updates.

## Base

**Base** refers to a set of YAML files created for the purpose of deploying a Sourcegraph instance to a Kubernetes cluster. These files come preconfigured with default values that can be used to quickly deploy a Sourcegraph instance. 

However, deploying with these default settings may not be suitable for all environments and specific use cases. For example, the default resource allocation may not match the requirements for your specific instance size, or the default deployments may include RBAC resources that you would like to remove. To address these issues, creating a Kustomize overlay can be an effective solution. It allows you to customize the resources defined in the base layer to suit the specific requirements of the deployment.

## Overlays

The **instances directory** is used to store customizations specific to your deployment environment. It allows you to create different overlays for different instances for different purposes, such as an instance for production and another for staging. It is best practice to avoid making changes to files outside of the **instances directory** in order to prevent merge conflicts during future updates.

An [overlay](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/#bases-and-overlays) acts as a **customization layer** that contains a [kustomization file](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/), where components are defined as the **configuration layer** to include a set of instructions for Kustomize to generate and apply configurations to the **base layer**.

Here is an example of a `kustomization` file from one of our [examples](#examples-overlays) that is configured for deploying a size XS instance to a k3s cluster:

```yaml
# examples/k3s/xs/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: sourcegraph-example
resources:
  - ../../base/sourcegraph # Deploy Sourcegraph main stack
  - ../../base/monitoring # Deploy Sourcegraph monitoring stack
components:
  # Configurs the deployment for k3s cluster
  - ../../components/clusters/k3s
  # Configurs the resources we added above for a size XS instance
  - ../../components/sizes/xs
```

### Examples overlays

 In addition to providing a template for creating new overlays, we also provide a set of examples that are pre-configured for different environments. These pre-built overlays can be found inside the [examples](https://github.com/sourcegraph/deploy-sourcegraph-k8s/tree/master/examples) directory. 

## Overlay

In this section, we will take a quick look at the essential part that make up a Kustomize overlay tailored for Sourcegraph.

### Template

An overlay is a directory that contains various files used to configure a deployment for a specific scenario. These files include the [kustomization.yaml file](#kustomization-yaml), which is used to specify how the resources defined in the base manifests should be customized and configured, as well as other files such as environment variable files, configuration files, and patches.

File structure:

```bash
# github.com/sourcegraph/deploy-sourcegraph-k8s
└── instances
  ├── $INSTANCE_NAME
  │   ├── kustomization.yaml
  │   ├── buildConfig.yaml
  │   └── patches
  │       └── additional config files go here...
  └── template
      └── buildConfig.template.yaml
      └── kustomization.template.yaml
```

All custom overlays built for a specific instance should be stored in the [instances directory](https://github.com/sourcegraph/deploy-sourcegraph-k8s/tree/master/instances), where you can find the [instances/template folder](https://github.com/sourcegraph/deploy-sourcegraph-k8s/tree/master/instances/template). This folder contains a [kustomization.template.yaml file](#kustomization-yaml) that is preconfigured to construct an overlay for deploying Sourcegraph, and a [buildConfig.template.yaml](#buildconfig-yaml).

### kustomization.yaml

The [kustomization.yaml file](#kustomization-yaml) is a fundamental element of a Kustomize overlay. It is situated in the root directory of the overlay and serves as a means of customizing and configuring the resources defined in the base manifests, as outlined in our [configuration documentation](../configure.md).

To correctly configure your Sourcegraph deployment, it is crucial to create an overlay using the `kustomization.template.yaml` file provided. This [kustomization.yaml file](#kustomization-yaml) is specifically designed for Sourcegraph deployments, making the configuration process more manageable. The file includes various options and sections, allowing for the creation of a Sourcegraph instance that is tailored to the specific environment.

#### components-list

The order of components in the [kustomization.template.yaml file](#kustomization-yaml) is important and should be maintained. The components are listed in a specific order to ensure proper dependency management and compatibility between components. Reordering components can introduce conflicts or prevent components from interacting as expected. Only modify the component order if explicitly instructed to do so by the documentation. Otherwise, leave the component order as-is to avoid issues.

### buildConfig.yaml

Some Kustomize components may require additional configuration. These inputs typically specify environment/use-case-specific settings. For example, the name of your private registry to update images.
Only update the values inside the `buildConfig.yaml` file if a component's documentation explicitly instructs you to do so. Not all components need extra configuration, and some have suitable defaults.
Modifying `buildConfig.yaml` unnecessarily can cause errors or unintended behavior. Always check the [configuration docs](../configure.md) or comments in [kustomization.yaml](#kustomization-yaml) before changing this file.

### patches directory

The `patches directory` is designated to store configuration files that Kustomize uses to customize your deployment. These files can include Kustomize overlays, supplementary kustomization.yaml files, modified ConfigMaps, copies of base manifests, and other configuration files necessary for patching the base cluster.

When instructed by the configuration docs to set up the necessary files for configuring your Sourcegraph instance:

1. Create a directory called 'patches': `mkdir patches`
2. Create the required files within the 'patches' directory
  
This will ensure the files are in the correct location for the configuration process to access them.

> NOTE: Creating the patches directory is not mandatory unless instructed by the components defined in your overlay.

### Create a Sourcegraph overlay

The [instances/template](#template) directory serves as a starting point for creating a custom overlay for your deployment. It includes the template files that includes a list of components that are commonly used in Sourcegraph deployments. To create a new overlay, you can copy this directory to a new directory. Then, you can enable or disable specific components by commenting or uncommenting them in the overlay file `kustomization.yaml` inside the new directory. This allows you to customize your deployment to suit your specific needs.


**Step 1**: Set up a directory for your instance

Create a copy of the [instances/template](#template) directory within the `instances` subdirectory.

The name of this directory, `$INSTANCE_NAME`, serves as the name of your overlay for the specific instance, for example, `dev`, `prod`, `staging`, etc.

```bash
# from the root of the deploy-sourcegraph-k8s repository
$ export INSTANCE_NAME=staging # Update 'staging' to your instance name
$ cp -R instances/template instances/$INSTANCE_NAME
```

**Step 2**: Set up the configuration files

As described above, you can find two configuration files within the `$INSTANCE_NAME` directory:

1. The `kustomization.yaml` file is used to configure your Sourcegraph instance. 
2. The `buildConfig.yaml` file is used to configure components included in your `kustomization` file when required.

Follow the steps listed below to set up the configuration files for your instance overlay: `$INSTANCE_NAME`. 

#### kustomization.yaml

Rename the [kustomization.template.yaml](#kustomization-yaml) file in `instances/$INSTANCE_NAME` to `kustomization.yaml`:

```bash
  $ mv instances/template/kustomization.template.yaml instances/$INSTANCE_NAME/kustomization.yaml
```

#### buildConfig.yaml

Rename the [buildConfig.template.yaml](#buildconfig-yaml) file in `instances/$INSTANCE_NAME` to `buildConfig.yaml`:

```bash
  $ mv instances/template/buildConfig.template.yaml instances/$INSTANCE_NAME/buildConfig.yaml
```
**Step 3**: You can begin customizing your Sourcegraph deployment by updating the [kustomization.yaml file](#kustomization-yaml) inside your overlay, following our [configuration guides](../configure.md) for guidance.

## Components

An overlay in Kustomize is a set of configuration files that are used to customize the base resources. To understand an overlay, it's important to examine its components, which are listed under the components field inside the [kustomization.yaml file](#kustomization-yaml) of the overlay.

Most of our components are designed to be reusable for different environments and use cases. They can be used to add common labels and annotations, apply common configurations, or even generate resources. By using these components, you can minimize the amount of duplicated code in your overlays and make them more maintainable.

### Rule of thumbs

It is important to understand how each component covered in the [configuration guide](../configure.md) is used to configure your Sourcegraph deployment. Each component has specific configuration options and settings that need to be configured correctly in order for your deployment to function properly. By reading the details and understanding how each component is used, you can make informed decisions about which components to enable or disable in your overlay file, and how to configure them to meet your needs. It also helps to learn how to troubleshoot if something goes wrong.

Here are some **rule of thumbs** to follow when combining different components to ensure that they work together seamlessly and avoid any conflicts:

- Understand the dependencies between components: Some components may depend on others to function properly. For example, if you include a component to remove a daemonset, you should also include the monitoring component to make sure that there is something for the component to remove. If you don't, the overlay build process will fail because there is nothing for the component to remove.

- Be aware of the configuration settings of each component: Each component has its own configuration settings that need to be configured correctly. For example, if you include a component that adds RBAC resources to your deployment when your cluster is RBAC-disabled, it will cause the overlay build process to fail.

- Understand the resources each component creates: Each component creates its own set of resources that need to be managed. For example, if you include a component that creates a service and another component that creates a deployment, you need to make sure that the service points to the deployment.

- Be careful when disabling components: Some components may depend on others to function properly. When disabling a component, you need to consider the impact it may have on other components.

By following these rule of thumbs, you can ensure that the components you include in your overlay work together seamlessly and avoid any conflicts. It is also a good practice to review the manifests generated by the overlay before deploying them to the production environment, to make sure that the overlay is configured as desired.

## Remote build

Remote build feature allows you to deploy an overlay using a git URL, but it should be noted that it does not support custom configurations as the resources are hosted remotely. 

To create manifests using a remote overlay, you can use the following command:

```bash
# Replace the $REMOTE_OVERLAY_URL with a URL of an overlays.
$ kubectl kustomize $REMOTE_OVERLAY_URL -o cluster.yaml
```

The command above will download the overlay specified in the $REMOTE_OVERLAY_URL and apply the customizations to the base resources and output the resulting customized manifests to the file cluster.yaml. This command allows you to preview the resources before running the apply command below to deploy using the remote overlay.

```bash
$ kubectl apply --prune -l deploy=sourcegraph -f cluster.yaml
```

## Preview manifests

To create a customized deployment using your overlay, run the following command from the root directory of your deployment repository.

```bash
$ kubectl kustomize $PATH_TO_OVERLAY -o cluster.yaml
```

This command will apply the customizations specified in the overlay located at $PATH_TO_OVERLAY to the base resources and output the customized manifests to the file `cluster.yaml`.

The $PATH_TO_OVERLAY path can be a local path or remote path. For example:

```bash
# Local
$ kubectl kustomize examples/k3s/xs -o cluster.yaml
# Remote
$ kubectl kustomize https://github.com/sourcegraph/deploy-sourcegraph-k8s/examples/k3s/xs -o cluster.yaml
```

> NOTE: This command will only generate the customized manifests and will not apply them to the cluster. It does not affect your current deployment until you run the apply command.

## Kustomize with Helm

Kustomize can be used in conjunction with Helm to configure Sourcegraph, as outlined in [this guidance](../helm.md#integrate-kustomize-with-helm-chart). However, this approach is only recommended as a temporary workaround while Sourcegraph adds support for previously unsupported customizations in its Helm chart. This means that using Kustomize with Helm is not a long-term solution.

## Deprecated

The previous Kustomize structure we built for our Kubernetes deployments depends on scripting to create deployment manifests. It does not provide flexibility and requires direct changes made to the base manifests.

With the new Kustomize we have introduced in this documentation, these issues can now be avoided. The previous version of the Sourcegraph Kustomize Overlays are still available, but they should not be used for any new Kubernetes deployment.

See the [old deployment docs for deploying Sourcegraph on Kubernetes](https://docs.sourcegraph.com/@v4.4.2/admin/deploy/kubernetes).

> NOTE: The latest version of our Kustomize overlays does not work on instances that are older than v4.5.0.
