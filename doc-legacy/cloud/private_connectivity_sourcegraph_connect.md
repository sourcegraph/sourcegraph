# Private Resources on on-prem data center via Sourcegraph Connect agent

<span class="badge badge-experimental">Experimental</span>

<p>Please contact Sourcegraph directly via <a href="https://sourcegraph.com/contact">prefered contact method</a> for more informations</p>
</aside>

As part of the [Enterprise tier](https://sourcegraph.com/pricing), Sourcegraph Cloud supports connecting private resouces on any on-prem private network by running Sourcegraph Connect tunne agent in customer infrastructure.

## How it works

Sourcegraph will set up a tunnel server in a customer dedicated GCP project. Customer will start the tunnel agent provided by Sourcegraph with the provided credential. After start, the agent will authenticate and establish a secure connection with Sourcegraph tunnel server.

Sourcegraph Connect consists of three major components:

Tunnel agent: deployed inside customer network, which uses its own identity and encrypts traffic between customer code host and client. Agent can only communicate with permitted customer code hosts inside the customer network. Only agents are allowed to establish secure connections with tunnel server, the server can only accept connections if agent identity is approved.

Tunnel server: a centralized broker between client and agent managed by Sourcegraph. Its purpose is to set up mTLS, proxy encrypted traffic between clients and agents and enforce ACL.

Tunnel client: forward proxy clients managed by sourcegraph. Every client has its own identity and it cannot establish a direct connection with the customer agent, and has to go through tunnel server.

[link](https://link.excalidraw.com/readonly/453uvY8infI8wskSecGJ)

<iframe src="https://link.excalidraw.com/readonly/453uvY8infI8wskSecGJ" width="100%" height="100%" style="border: none;"></iframe>

## Steps

### Initiate the process

Customer should reach out to their account manager to initiate the process. The account manager will work with the customer to collect the required information and initiate the process, including but not limited to:

- The DNS name of the private code host, e.g. `gitlab.internal.company.net` or private artifact registry, e.g. `artifactory.internal.company.net`.

Finally, Sourcegraph will provide the following:

- Instruction to run the agent along with credentials, and endpoint to allowlist egress traffic if needed.

### Create the connection

Customer can follow the provided instruction and install the tunnel agent in the private network. At a high level:

- Permit egress to the internet to a set of static IP addresses and corresponding ports to be provided by Sourcegraph.
- Permit egress to the private resources at the given port.
- Run the tunnel agent binary with provided config files and credentials.

### Create the code host connection

Once the connection to private code host is established, the customer can create the [code host connection](../../admin/external_service/index.md) on their Sourcegraph Cloud instance.

## FAQ

### How are connections encrypted? Can anyone else inspect the traffic?

Connections between the tunnel agent inside customer network and a tunnel server inside customer dedicated Sourcegraph GCP VPC use mTLS. Both agents, server and Sourcegraph clients have their own certificates and encrypt/decrypt traffic over TCP. mTLS enforce that both the client and the agent has to have a private key and present valid signed certificate from a trusted CA, which is not shared and this protects from [on-paths and spoofing attacks](https://www.cloudflare.com/en-gb/learning/access-management/what-is-mutual-tls/).

### How do you authenticate requests?

Both tunnel clients and agents are assigned an identity corresponding to a GCP Service Account, and they are provided credentials to prove such identity. For tunnel agents, a Service Account key is distributed to the customer. For tunnel clients, it will utilize Workload Identity to prove its identity. They use them to authenticate against tunnel server by sending signed JWT tokens and public key. JWT token contains information about GCP service account credential public key required to validate signature and confirm identity of requestor. The server will then sign the requestor public key and respond with a signed certificate containing GCP Service Account email as a Subject Alternative Name (SAN).

Finally, if the customer NAT Gateway/Exit Gateway has stable CIDRs, we can provision firewall rules to restrict access to the tunnel server from the provided IP ranges only for added layer of security.

### How do you enforce authorization to restrict what requests can reach the private code host?

The tunnel server is configured with ACLs. With mTLS every entity in the network has its own identity. Client identity is used as a source for accessing customer private code hosts, while agents identity is used for destination. Tunnel server ensures that only clients with proved identity can communicate with customer tunnel agents.

### Do you rotate the encryption keys?

Encryption keys are short-lived and both tunnel agents and clients have to refresh certificates every 24h. The customer may also manually rotate it by restarting the tunnel agent.

### How do you manage keys or certificates?

We utilize GCP Certificate Authority Service (CAS), a managed Public Key Infrastructure (PKI) service. It is responsible for the storage of all signing keys (e.g., root CAs, immediate CAs), and the signing of client certificates. Access to GCP CAS is governed by GCP IAM service and only necessary services or individuals will have access to the service with audit trails in GCP Logging.

The TLS private key on the tunnel agent or tunnel clients only exist in memory, and never shares with other party. Only the public key is sent to the tunnel server to issue a signed certificate to establish mTLS connection.

### How do you audit access?

Tunnel server will log all critical operations with sufficient metadata to identify the requester to GCP Logging with a default 30-day retention policy. We will also be monitoring unauthorized access events to watch out for potential attackers.

### Why TCP over gRPC?

The tunnel is built using TCP over gRPC. gRPC is a high-performant and battle-tested framework, e.g., built-in support for mTLS for a trusted secure connection. We believe TCP and HTTP/2 are widely supported in majority of environments. Moreover, the simplicity of having a single endpoint for connection between customer environment and their Cloud instance greatly simplifies the work required for customer IT admin. Compared to traditional VPN solutions, such as OpenVPN, IPSec, and SSH over bastion hosts, gRPC allows us to design our own protocol, and the programmable interface allows us to implement advanced features, e.g., fine-grained access control at a per connection level, audit logging with rich metadata.

### How many agents can a customer start?

To obtain high availability customer can start multiple tunnel agents. Each of the agents will use the same GCP Service Account credentials, authenticate with the tunnel server and establish connection to it. Tunnel client will randomly select an available agent to forward the traffic.

### How does the customer configure the network to make the agent work?

Customer tunnel agent has to authenticate and establish connection with the tunnel server. Sourcegraph will provide a single dedicated static IP from customer dedicated GCP VPC which is used to connect with the tunnel server. Customer has to configure network egress to allow TCP (HTTP/2) traffic access to this static IP.

### How can I restrict access to my private code host connection?

The customer has full control over the tunnel agent configuration and they can terminate the connection at any time.
What if the attacker gains access to the frontend?

In the event of an attacker gaining access to the sourcegraph containers, we consider this to be a security breach and we have Incident response processes in place that we will follow. However, we have many controls in place to prevent this from happening where Cloud infrastructure access always requires approval and the Security team is on-call for unexpected usage patterns. You may learn more from our [Security Portal].

Please reach out to us if you have any specific questions regarding our Cloud security posture, and we are happy to provide more detail to address your concerns.

### How to harden the tunnel agent deployment?

We recommend using an allowlist to limit the egress traffic of the agent to IP addresses provided by Sourcegraph and specific private resources you would like to permit access. This will prevent the agent to talk to arbitrary services, and reduce the blast radius in the event of a security event.

### How can I audit the data Sourcegraph has access to in my environment?

The tunnel is secured and authenticated by mTLS over gRPC, and everything is encrypted over transit. If customer is looking to perform audit, such as, TLS inspection, on the connection between the private resources and Sourcegraph Cloud. We recommend to only intercept and inspect traffic between the tunnel agent and private resources. The connection between the tunnel agent and Sourcegraph Cloud is using a custom protocol, and the decrypted payload has very little value.

### Can I use self-signed TLS certificate for my private resources?

Yes. Please work with your account team to add the certificate chain of your internal CA to [site configuration](https://docs.sourcegraph.com/admin/config/site_config#experimentalFeatures) at `experimentalFeatures.tls.external.certificates`

[security portal]: https://security.sourcegraph.com/
