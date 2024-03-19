# Private Resources exposed via alternate public load balancers

<p>Please contact Sourcegraph directly via <a href="https://about.sourcegraph.com/contact">prefered contact method</a> for more informations</p>
</aside>

As part of the [Enterprise tier](https://about.sourcegraph.com/pricing), Sourcegraph Cloud supports connecting customer private dependecies from customer data center via public load balancer on customer side.

> For private dependecies in AWS or GCP, please refer to [other deployment methods](./index.md#private-connectivity)

## How it works

Sourcegraph Cloud is a managed service hosted on GCP. Customer will expose private resource via load balancer with IP allowlist for 2 static IPs provided by Sourcegraph. Sourcegraph will then be able to access the private resource over HTTPS through the load balancer from the GCP project hosting Sourcegraph Cloud. Sourcegraph recommends to setup passthrough TCP network load balancer to avoid maintaining valid TLS certificate on the network load balancer

[link](https://link.excalidraw.com/readonly/gc6P8SDOEMCcrIl9cl64)

<iframe src="https://link.excalidraw.com/readonly/gc6P8SDOEMCcrIl9cl64" width="100%" height="100%" style="border: none;"></iframe>

## Steps

### Initiate the process

Customer should reach out to their account manager to initiate the process. The account manager will work with the customer to collect the required information and initiate the process, including but not limited to:

- The private DNS name of the private resource, e.g. `github.internal.company.net`. Notes, this is the DNS name customer users interact on a daily basis.
- The public DNS name of the network load balancer exposing the private resource, e.g. `github-public-nlb.company.net`.

Sourcegraph will provide 2 static IPs for customer to allowlist ingress traffic for load balancer.

## FAQ

### Why passthrough TCP network load balancer?

With passthrough network load balancer, the load balancer acts as a simple network proxy to forward traffic to the backend private resource without terminating TLS. This avoids the need to install additional TLS certificate on the network load balancer, reducing opertional overhead.

In the event you have to use a proxy network load balancer or an application (L7) load balancer with a TLS listener, the load balancer must meet the following requirements:

- Present valid TLS certificates valid for both public and private dns name.
- Forward traffic to the private resource regardless public or private dns name is used to access the load balancer

Assuming your private resources is a web service listening at port `443`, you can validate your setup:

```sh
curl --connect-to github.internal.company.net:443:github-public-nlb.company.net:443 https://github.internal.company.net
```

### Can I use my internal dns name for artifact registry?

Yes, customer can expose their private registry with internal DNS name. Sourcegraph will provision dns-proxy, which translates customer private domain to public customer load balancer domain. No changes in customer configuration are required.

### How secure is the connection?

This connection depends on IP allowlisting to ensure private resources are only accessible by Sourcegraph Cloud. Sourcegraph will provide 2 static IPs for customer to allowlist ingress traffic for load balancer.
In transit, these resources are encrypted via the TLS certificate.

### What are the next steps when code host connectivity is working?

Once the connection is established, the customer can create the [code host connection](../../admin/external_service/index.md) on their Sourcegraph Cloud instance using private dns name.

### What are the next steps when artifact registry connectivity is working?

If private artifact registry is protected by authentication, the customer will need to:

- Create executor secrets containing credentials for Sourcegraph to access the private artifact registry - [how to configure executor secrets](../admin/executors/executor_secrets.md#executor-secrets)
- Update auto-indexing inference configuration to create additional files from executor secrets for given programing language - [how to configure auto-indexing](../code_navigation/references/inference_configuration.md)

### Can I use self-signed TLS certificate for my private resources?

Yes. Please work with your account team to add the certificate chain of your internal CA to [site configuration](https://docs.sourcegraph.com/admin/config/site_config#experimentalFeatures) at `experimentalFeatures.tls.external.certificates`.
