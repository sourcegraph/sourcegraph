<style>

.markdown-body aside p:before {
  content: '';
  display: inline-block;
  height: 1.2em;
  width: 1em;
  background-size: contain;
  background-repeat: no-repeat;
  background-image: url(../code_monitoring/file-icon.svg);
  margin-right: 0.2em;
  margin-bottom: -0.29em;
}

</style>

# Deployment Overview

Sourcegraph offers multiple deployment options to suit different needs. The appropriate option for your organization depends on your goals and requirements, as well as the technical expertise and resources available. The following sections overview the available options and their associated investments and technical demands.

## Deployment types

Carefully consider your organization's needs and technical expertise when selecting a Sourcegraph deployment method. The method you choose cannot be changed for a running instance, so make an informed decision. The available methods have different capabilities, and the following sections provide recommendations to help you choose.

### [Sourcegraph Cloud](https://sourcegraph.com/get-started?t=enterprise)

**For Enterprises looking for a managed Cloud solution.**

A cloud instance hosted and maintained by Sourcegraph

<div>
  <a class="cloud-cta" href="https://sourcegraph.com/get-started?t=enterprise" target="_blank" rel="noopener noreferrer">
    <div class="cloud-cta-copy">
      <h2>Get Sourcegraph on your code.</h2>
      <h3>A single-tenant instance managed by Sourcegraph.</h3>
      <p>Sign up for a 30 day trial for your team.</p>
    </div>
    <div class="cloud-cta-btn-container">
      <div class="visual-btn">Get free trial now</div>
    </div>
  </a>
</div>
  
### [Machine Images](machine-images/index.md) 

**For Enterprises looking for a self-hosted solution on the Cloud infrastructure of their choice.** 

An option to run Sourcegraph on your own infrastructure using pre-configured machine images.

Customized machine images allow you to spin up a preconfigured and customized Sourcegraph instance with just a few clicks, all in less than 10 minutes. Currently available in the following hosts:

<div class="getting-started">
  <a class="btn btn-secondary text-center" href="machine-images/aws-ami"><span>AWS AMIs</span></a>
  <a class="btn btn-secondary text-center" href="machine-images/azure"><span>Azure Images</span></a>
  <a class="btn btn-secondary text-center" href="machine-images/gce"><span>Google Compute Images</span></a>
</div>

### [Install-script](single-node/script.md)

Sourcegraph provides an install script that can deploy Sourcegraph instances to Linux-based virtual machines. This method is recommended for:

- On-premises deployments (your own infrastructure)
- Deployments to unsupported cloud providers (non-officially supported)

>NOTE: Deploying with machine images requires technical expertise and the ability to maintain and manage your own infrastructure.

### [Kubernetes](kubernetes/index.md)

**For large Enterprises that require a multi-node, self-hosted solution.**

- **Kustomize** utilizes the built-in features of kubectl to provide maximum flexibility in configuring your deployment
- **Helm** offers a simpler deployment process but with less customization flexibility

We highly recommend deploying Sourcegraph on Kubernetes with Kustomize due to the flexibility it provides.

<div class="getting-started">
  <a class="btn btn-secondary text-center" href="kubernetes/index"><span>Kustomize</span></a>
  <a class="btn btn-secondary text-center" href="kubernetes/helm"><span>Helm</span></a>
</div>

>NOTE: Given the technical knowledge required to deploy and maintain on Kubernetes, teams without these resources should contact their Sourcegraph representative at [sales@sourcegraph.com](mailto:sales@sourcegraph.com) to discuss alternative deployment options.

### Local machines

**For setting up non-production environments on local machines.**

  - [Docker Compose](docker-compose/index.md) - Install Sourcegraph on Docker Compose
  - [Docker Single Container](docker-single-container/index.md) - Install Sourcegraph using a single Docker container

### ARM / ARM64 support

Running Sourcegraph on ARM / ARM64 images is not supported for production deployments.
