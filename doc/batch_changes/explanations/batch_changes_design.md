# Batch Changes design

Why is the [Batch Changes](../../../batch_changes/index.md) feature designed the way it is?

## Principles

- **Declarative API** (not imperative). You declare your intent, such as "lint files in all repositories with a `package.json` file"<!-- TODO(sqs): thorsten had a suggestion to make this quote use non-imperative language but I can't find the comment -->. Batch Changes figures out how to achieve your desired state. The external state (of repositories, changesets, code hosts, access tokens, etc.) can change at any time, and temporary errors frequently occur when reading and writing to code hosts. These factors would make an imperative API very cumbersome because each API client would need to handle the complexity of the distributed system.
- **Define a batch change in a file** (not some online API). The source of truth of a batch change's definition is a file that can be stored in version control, reviewed in code review, and re-applied by CI. This is in the same spirit as IaaC (infrastructure as code; e.g., storing your Terraform/Kubernetes/etc. files in Git). We prefer this approach over a (worse) alternative where you define a batch change in a UI with a bunch of text fields, checkboxes, buttons, etc., and need to write a custom API client to import/export the batch change definition.
- **Shareable and portable.** You can share your batch specs, and it's easy for other people to use them. A batch spec expresses an intent that's high-level enough to (usually) not be specific to your own particular repositories. You declare and inject configuration and secrets to customize it instead of hard-coding those values.
- **Large-scale.** You can run batch changes across 10,000s of repositories. It might take a while to compute and push everything, and the current implementation might cap out lower than that, but the fundamental design scales well.
- **Accommodates a variety of code hosts and review/merge processes.** Specifically, we don't want Batch Changes to only work for GitHub pull requests. (See [current support list](../../../batch_changes/index.md#supported-code-hosts-and-changeset-types).)

## Comparison to other distributed systems

Kubernetes is a distributed system with an API that many people are familiar with. Batch Changes is also a distributed system. All APIs for distributed systems need to handle a similar set of concerns around robustness, consistency, etc. Here's a comparison showing how these concerns are handled for a Kubernetes [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) and a batch change. In some cases, we've found Kubernetes to be a good source of inspiration for the Batch Changes API, but resembling Kubernetes is **not** an explicit goal.

<table>
  <tr>
    <td/>
    <th>Kubernetes <a href="https://kubernetes.io/docs/concepts/workloads/controllers/deployment/">Deployment</a></th>
    <th>Sourcegraph Batch Changes</th>
  </tr>
  <tr>
    <th>What underlying thing does this API manage?</th>
    <td>Pods running on many (possibly unreliable) nodes</td>
    <td>Branches and changesets on many repositories that can be rate-limited and externally modified (and our authorization can change)</td>
  </tr>
  <tr>
    <th>Spec YAML</th>
    <td>
      <pre><code><em># File: foo.Deployment.yaml</em>
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:<div style="padding:0.5rem;margin:0 -0.5rem;background-color:rgba(0,0,255,0.25)"><strong>  # Evaluate this to enumerate instances of...</strong>
  replicas: 2</div>
<div style="padding:0.5rem;margin:0 -0.5rem;background-color:rgba(255,0,255,0.25)"><strong>  # ...this template.</strong>
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80</div></pre></code>
    </td>
    <td>
      <pre><code><em># File: hello-world.batch.yaml</em>
name: hello-world
description: Add Hello World to READMEs

<div style="padding:0.5rem;margin:0 -0.5rem;background-color:rgba(0,0,255,0.25)"><strong># Evaluate this to enumerate instances of...</strong>
on:
  - repositoriesMatchingQuery: file:README.md

steps:
  - run: echo Hello | tee -a $(find -name '*.md')
    container: alpine:3
</div>
<div style="padding:0.5rem;margin:0 -0.5rem;background-color:rgba(255,0,255,0.25)"><strong># ...this template.</strong>
changesetTemplate:
  title: Hello World
  body: My first batch change!
  branch: hello-world
  commit:
    message: Append Hello to .md files
  published: false</pre></code>
    </td>
  </tr>
  <tr>
    <th>How desired state is computed</th>
    <td>
      <ol>
        <li>Evaluate <code>replicas</code>, etc. (blue) to determine pod count and other template inputs</li>
        <li>Instantiate <code>template</code> (pink) once for each pod to produce PodSpecs</li>
      </ol>
    </td>
    <td>
      <ol>
        <li>Evaluate <code>on</code>, <code>steps</code> (blue) to determine list of patches</li>
        <li>Instantiate <code>changesetTemplate</code> (purple) once for each patch to produce ChangesetSpecs
</li>
      </ol>
    </td>
  </tr>
  <tr>
    <th>Desired state consists of...</th>
    <td>
      <ul>
        <li>DeploymentSpec file (the YAML above)</li>
        <li>List of PodSpecs (template instantiations)</li>
      </ul>
    </td>
    <td>
      <ul>
        <li>batch spec file (the YAML above)</li>
        <li>List of ChangesetSpecs (template instantiations)</li>
      </ul>
    </td>
  </tr>
  <tr>
    <th>Where is the desired state computed?</th>
    <td>The deployment controller (part of the Kubernetes cluster) consults the DeploymentSpec and continuously computes the desired state.</td>
    <td>
      <p>The <a href="https://github.com/sourcegraph/src-cli">Sourcegraph CLI</a> (running on your local machine, not on the Sourcegraph server) consults the batch spec and computes the desired state when you invoke <code>src batch apply</code>.</p>
      <p><strong>Difference vs. Kubernetes:</strong> A batch change's desired state is computed locally, not on the server. It requires executing arbitrary commands, which is not yet supported by the Sourcegraph server. See Batch Changes known issue "<a href="../../../batch_changes#server-execution">Batch Changes steps are run locally...</a>".</p>
    </td>
  </tr>
  <tr>
    <th>Reconciling desired state vs. actual state</th>
    <td>The "deployment controller" reconciles the resulting PodSpecs against the current actual PodSpecs (and does smart things like rolling deploy).</td>
    <td>The "Batch Changes controller" (i.e., our backend) reconciles the resulting ChangesetSpecs against the current actual changesets (and does smart things like gradual roll-out/publishing and auto-merging when checks pass).</td>
  </tr>
</table>

These docs explain more about Kubernetes' design:

- [Kubernetes Object Management](https://kubernetes.io/docs/concepts/overview/working-with-objects/object-management/)
- [Kubernetes Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
  - [Desired versus current state](https://kubernetes.io/docs/concepts/architecture/controller/#desired-vs-current)
- [Kubernetes Architecture](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/architecture/architecture.md)
- [Kubernetes General Configuration Tips](https://kubernetes.io/docs/concepts/configuration/overview/#general-configuration-tips)
- [Kubernetes Design and Development Explained](https://thenewstack.io/kubernetes-design-and-development-explained/)
