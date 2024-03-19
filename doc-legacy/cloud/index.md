# Sourcegraph Cloud

Sourcegraph Cloud is a single-tenant cloud solution. Cloud instances are private, dedicated instances provisioned and managed by Sourcegraph. Sourcegraph Cloud was formerly known as managed instances.

Sourcegraph provisions each instance in an isolated and secure cloud environment. Access is restricted to only your organization through your SSO provider of choice. Enterprise VPN is available upon request.

## Start a Sourcegraph Cloud trial

<div>
  <a class="cloud-cta" href="https://sourcegraph.com/get-started?t=enterprise" target="_blank" rel="noopener noreferrer">
    <div class="cloud-cta-copy">
      <h2>Get Sourcegraph on your code.</h2>
      <h3>A single-tenant instance managed by Sourcegraph.</h3>
      <p>Sign up for a 15 day trial for your team.</p>
    </div>
    <div class="cloud-cta-btn-container">
      <div class="visual-btn">Get free trial now</div>
    </div>
  </a>
</div>

Use the button above to sign up for a free 15-day trial of Sourcegraph Cloud. Please [contact us](https://sourcegraph.com/contact/sales) if you have specific VPN requirements or you require a large deployment with >500 users, >1,000 repos, or monorepos >5 GB.

### Trial limitations

We currently have a limited capacity of single-tenant cloud instances and are prioritizing organizations with more than 100 developers. When you request a trial, you will receive an email indicating the status of your request.

If your organization has fewer than 100 developers, we recommend trying [Sourcegraph self-hosted](https://docs.sourcegraph.com/#self-hosted).

If you're eligible for a cloud instance, you will receive a link to the instance URL once it's provisioned. This normally takes less than one hour during business hours. From there, we recommend using the [onboarding checklist](../getting-started/cloud-instance.md) to set up your instance quickly.

Trials last 15 days. When the end of the trial approaches, Sourcegraph's Customer Support team will check in with you to either help you set up a Cloud subscription or end your trial.

## Cloud subscription

As part of this service you will receive a number of benefits from our team, including:

### Initial setup, configuration, and cost estimation

- Advising if managed instances are right for your organization.
- Initial resource estimations based on your organization & code size.
- Putting forward a transparent deployment & cost estimate plan.
- Your own `example.sourcegraphcloud.com` domain with fully managed [DNS & HTTPS](../admin/http_https_configuration.md). Optionally, you can [bring your own domain](#custom-domain).
- Hardware provisioning, software installation, and kernel configuration done for you.
- Direct assistance in:
  - [Adding repositories from all of your code hosts to Sourcegraph](../admin/external_service/index.md)
  - [Integrating your single sign-on provider with Sourcegraph](../admin/auth/index.md)
  - [Configuring Sourcegraph](../admin/config/index.md)

### Access to all Sourcegraph features

All Sourcegraph features are avilable on Sourcegraph Cloud instances out-of-the-box, including but not limited to:

- [Server-side Batch Changes](../batch_changes/explanations/server_side.md)
- [Precise code navigation powered by auto-indexing](../code_navigation/explanations/auto_indexing.md)
- [Code Monitoring](../code_monitoring/index.md) (including [email delivery](#managed-smtp) of notifications)

### Access restrictions

- Granting your team application-level admin access to the instance.
- Configuring any IP-restrictions (e.g. VPN) and/or SSO restrictions to the instance.

### Monthly upgrades and maintenance

- Automatic monthly [upgrades](../admin/updates/index.md) and maintenance.
- Regular reassessment of resource utilization based on your organization's unique usage to determine if costs can be reduced without impact to service. Additionally, you will automatically benefit from any committed use cloud provider discounts we receive.

### Custom domains

Sourcegraph Cloud provides all customer instances a `customer.sourcegraphcloud.com` domain. This domain is fully managed by Sourcegraph, including DNS and HTTPS.
However, to provide better branding and a more seamless experience for your users, you may bring your own company domain, for example `sourcegraph.company.io`.

In order to use your own domain, you need to perform an one-time setup by adding DNS records at your authoritative DNS. These DNS records are neccessary to ensure that your users can access your Sourcegraph instance via the custom domain, and also to ensure we can provide managed TLS certificates for your instance. See a [list of DNS records to be created by your organization](#dns-records-to-be-created-by-your-organization) below as an example. Additionally, your custom domain's [CAA records](https://blog.cloudflare.com/caa-of-the-wild/) should permit our upstream certificate authorities to issue certificates for your domain, follow the [instructions](#verify-caa-records) below to verify your CAA records.

Please reach out to your Sourcegraph account team to request a custom domain to be configured for your Sourcegraph Cloud instance.

#### DNS records to be created by your organization

Below is a list of the DNS records that are required to be created by your organization. This is for illustrative purposes only, and the actual records will be provided by your Sourcegraph account team. We use `src.acme.com` as an example custom domain.

- `CNAME` record for `src.acme.com` pointing to `acme.sourcegraphcloud.com`
- `TXT` record for `_cf-custom-hostname.src.acme.com` with value `$token`
- `CNAME` record for `_acme-challenge.src.acme.com` pointing to `src.acme.com.$token.dcv.cloudflare.com`

#### Verify CAA records

To verify that your CAA records are set correctly, you can use the following command:

```sh
dig acme.com caa +short
```

If the output is empty, you don't have to do anything. If the output is not empty, and it does not contain `letsencrypt.org` and `pki.goog`, you need to add them to your CAA records to the apex domain.

#### Limitations

- You can only have a single custom domain per Sourcegraph Cloud instance.
- You can only use the custom domain to access your Sourcegraph Cloud instance.

### Multiple region availability

Sourcegraph Cloud instances are deployed in one of Google Cloud Platform data center locations:

- North America (USA) - `us-central1`
- Europe (UK or Germany) - `europe-west2` or `europe-west3`
- Asia (Japan) - `asia-northeast1`
- Australia - `australia-southeast1`

If you have specific requirements for the region, please reach out to your Sourcegraph account team.

More details about the locations and data storage can be found in [our handbook](https://handbook.sourcegraph.com/departments/cloud/technical-docs/multi-region/)

### Private Connectivity

Sourcegraph Cloud can connect to resources that are publically accessible or protected by IP-based firewall rules out-of-the-box. Sourcegraph can provide static IP addresses for customers to add to their firewall allowlist. Please let your account team know.

Private Connectivity enables customers to privately connect Private Resources to the Sourcegraph Cloud instance. Private Resources refer to services that are not publicly accessible, such as self-hosted GitHub Enterprise servers, self-hosted GitLab instances, self-hosted Nexus instance, or Jira Data Center deployed in a private network that are only accessible through VPN. Learn more about Private Connectvity support below:

- [Private Resources on AWS via AWS Private Link](./private_connectivity_aws.md)
- [Private Resources on GCP via GCP Private Service Connect](./private_connectivity_gcp.md)
- [Private Resources on on-prem data center via Sourcegraph Connect agent](./private_connectivity_sourcegraph_connect.md)
- Private Resources on Azure are not supported yet, please reach out to your account manager if you are interested in this feature.

For unsupported private connectivity methods, Sourcegraph offers connectivity via customer-managed alternate public load balancers:

- [Private Resources exposed via alternate public load balancers](./private_connectivity_public_lb.md)

### Health monitoring, support, and SLAs

- Instance performance and health [monitored](../admin/observability/index.md) by our team's on-call engineers.
- [Support and SLAs](https://handbook.sourcegraph.com/support#for-customers-with-managed-instances).

### Backup and restore

<span class="badge badge-note">SOC2/CI-79</span>

Backup and restore capability is provided via automated snapshots.

- Frequency: Snapshots are produced daily.
- Retention period: Snapshots are kept for 90 days.

### Training, feedback, and engagement

As with any Sourcegraph enterprise customer, you will also receive support from us with:

- [Installing code host and code review integrations](../integration/index.md)
- Monitoring and aggregating user feedback
- Understanding usage statistics of your deployment
- Internal rollout programs including:
  - Holding company-wide or team-by-team training sessions ([contact us](https://sourcegraph.com/contact/sales) for details)
  - Helping the maintainers of your internal engineer onboarding add a session on Sourcegraph
  - Holding ongoing brown bag lunches to introduce new feature releases
  - Advice and templates on how to introduce Sourcegraph to your engineering organization

### Managed SMTP

All Sourcegraph Cloud instances are provisioned with a Sourcegraph-managed SMTP server through a [third-party provider](https://sourcegraph.com/terms/subprocessors) for transactional email delivery. Email capabilities power features like:

- [Code Monitoring](../code_monitoring/index.md) notifications
- Inviting other users to a Sourcegraph instance, or to an organization/team on a Sourcegraph instance
- Important updates to user accounts (for example, creation of API keys)
- For [`builtin` authentication](../admin/auth/index.md#builtin-password-authentication), password resets and email verification

By default, emails will be sent from an `@cloud.sourcegraph.com` email address. To test email delivery, refer to [sending a test email](../admin/config/email.md#sending-a-test-email).

To opt out of managed SMTP, please let your Sourcegraph Account team know when requesting a trial. You can also opt out by [overriding the managed `email.address` and `email.smtp` configuration with your own in site configuration](../admin/config/email.md). If you have specific requests for managed SMTP, please [reach out regarding special requirements](#accommodating-special-requirements).

To learn more about how the Sourcegraph team operates managed SMTP internally, refer to [our handbook](https://handbook.sourcegraph.com/departments/cloud/technical-docs/managed-smtp/).

### Cody

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> This feature is beta and might change in the future. We've released it to provide a preview of functionality we're working on.
</p>
</aside>

Cody is an AI coding assistant that lives in your editor that can find, explain, and write code. Cody uses a combination of Large Language Models (LLMs), Sourcegraph search, and Sourcegraph code intelligence to provide answers that eliminate toil and keep human programmers in flow. You can think of Cody as your programmer buddy who has read through all the code in open source, all the questions on StackOverflow, and all your organization's private code, and is always there to answer questions you might have or suggest ways of doing something based on prior knowledge. Learn more from [Cody documentation](../cody/overview/index.md) about how Cody can help you.

On Cloud, Cody can be enabled by contacting your Sourcegraph account team. Once Cody has been enabled by us, you can follow the instruction below to try it out.

#### Step 1: Configure the VS Code extension

Now that Cody is turned on on your Sourcegraph Cloud instance, any user can configure and use the Cody VS Code extension. This does not require admin privilege.

1. If you currently have a previous version of Cody installed, uninstall it and reload VS Code before proceeding to the next steps.
1. Search for “Sourcegraph Cody” in your VS Code extension marketplace, and install it.

<img width="500" alt="image" src="https://user-images.githubusercontent.com/25070988/227508342-cc6f29c0-ed91-4381-b651-16870c7c676b.png">

3. Reload VS Code, and open the Cody extension. Review and accept the terms.

4. Now you'll need to point the Cody extension to your Sourcegraph instance. On your instance, go to `settings` / `access token` (`https://<your-instance>.sourcegraphcloud.com/users/<your-username>/settings/tokens`). Generate an access token, copy it, and set it in the Cody extension.

<img width="1369" alt="image" src="https://user-images.githubusercontent.com/25070988/227510686-4afcb1f9-a3a5-495f-b1bf-6d661ba53cce.png">

5. In the Cody VS Code extension, set your instance URL and the access token

<img width="553" alt="image" src="https://user-images.githubusercontent.com/25070988/227510233-5ce37649-6ae3-4470-91d0-71ed6c68b7ef.png">

You're all set!

#### Step 2: Try Cody!

A few things you can ask Cody:

- "What are popular go libraries for building CLIs?"
- Open your workspace, and ask "Do we have a React date picker component in this repository?"
- Right click on a function, and ask Cody to explain it

## Requirements

### Business

- A dedicated project manager who serves as the point of contact for the rollout process.
- A mutual non-disclosure agreement and any additional approvals or special status required to allow Sourcegraph to manage infrastructure access tokens (listed below).
- Acceptance of our [Terms of Service for private instances](https://sourcegraph.com/terms-private) or an enterprise contract.

### Technical

- A dedicated technical point of contact for the installation process.
- [Tokens with read access to your code hosts](../admin/external_service/index.md) (we will direct you on how to enter them).
- [Keys, access tokens, or any other setup required to integrate your SSO (single sign-on) provider with Sourcegraph](../admin/auth/index.md), as well as support from a member of your team with administrator access to your SSO provider to help set up and test the integration.

### Limitation

> NOTE: We may be able to [support special requests](#accommodating-special-requirements), please reach out to your account team.

- The Sourcegraph instance can only be accessible via a public IP. Running it in a private network and pairing it with your private network via site-to-site VPN or VPC Peering is not yet supported.
- Instances currently run only on Google Cloud Platform in the [chosen regions](#multiple-region-availability). Other regions and cloud providers (such as AWS or Azure) are not yet supported.
- Some [configuration options](../admin/config/index.md) are managed by Sourcegrpah and cannot be override by customers, e.g. feature flags, experimental features.

## Security

Your managed instance will be accessible over HTTPS/TLS, provide storage volumes that are encrypted at rest, and have access restricted to only your team through your enterprise VPN and/or internal [SSO (single sign-on provider)](../admin/auth/index.md) of choice.

For all managed instances, we will provide security capabilities from Cloudflare such as WAF and rate-limiting to protect your instance from malicious traffic. By default, we permit 1000 requests per minute based on IP address. If you need a higher quota, please reach out to your account manager.

Your instance will be hosted in isolated Google Cloud infrastructure. See our [employee handbook](https://handbook.sourcegraph.com/departments/cloud/technical-docs/) to learn more about the cloud architecture we use. Both your team and limited Sourcegraph personnel will have application-level administrator access to the instance.

Only essential Sourcegraph personnel will have access to the instance, server, code, and any other sensitive materials, such as tokens or keys. The employees or contractors with access are bound by the same terms as Sourcegraph itself. Learn more in our [security policies for Sourcegraph Cloud](https://sourcegraph.com/security) or [contact us](https://sourcegraph.com/contact/sales) with any questions or concerns. You may also request a copy of our SOC 2 Report on our [security portal](https://security.sourcegraph.com).

### Sourcegraph management access

[Sourcegraph management access](https://handbook.sourcegraph.com/departments/cloud/technical-docs/oidc_site_admin/) is the ability for Sourcergaph employees to grant time-bound and audit-trailed UI access to Cloud instances in the events of instance maintenance, issue troubleshooting, and customer assistance. Customer consent is guaranteed prior to human accesses.

All Sourcegraph Cloud instances have Sourcegraph management access enabled by default, and customers may request to disable by contacting your Sourcegraph contact.

### Audit Logs

Our Cloud instances provide [audit logs](../admin/audit_log.md#cloud) to help you monitor and investigate actions taken by users and the system. These logs are available to download by request  and are also sent to a [centralized logging service](https://sourcegraph.com/security#logging) for 30 day retention (configurable for greater periods by request).

## Accommodating special requirements

We may be able to support special requests (network access policies, infrastructure requirements, custom version control systems, etc.) with additional time, support, and fees. [Contact us](https://sourcegraph.com/contact/sales) to discuss any special requirements you may have.

## FAQ

### Why Sourcegraph Cloud?

Sourcegraph Cloud is fully managed by a dedicated team of Sourcegraph experts who maintain 24 x 7 x 365 incident response and perform routine updates with [uptime SLA](https://docs.sourcegraph.com/sla#sourcegraph-cloud-sla-managed-instance). Sourcegraph Cloud is SOC 2 Type II audited.

Sourcegraph Cloud instances are single-tenant, limiting exposure to outages and security risks to individual instances. Each customer instance is isolated in a dedicated and fully segregated GCP project.

[Diagram 1](https://link.excalidraw.com/readonly/YGpFIX7DcEBezf5iGXTP)

<iframe src="https://link.excalidraw.com/readonly/YGpFIX7DcEBezf5iGXTP" width="100%" height="100%" style="border: none;"></iframe>

[Diagram 2](https://link.excalidraw.com/readonly/Dbs30t8ahBZadGHZACoc)

<iframe src="https://link.excalidraw.com/readonly/Dbs30t8ahBZadGHZACoc" width="100%" height="100%" style="border: none;"></iframe>

### Is data safe with Sourcegraph Cloud?

Sourcegraph Cloud utilizes a single-tenant architecture. Each customer's data is isolated and stored in a dedicated GCP project. Data is [encrypted in transit](https://cloud.google.com/docs/security/encryption-in-transit) and [at rest](https://cloud.google.com/docs/security/encryption/default-encryption) and is backed up daily. The data encryption keys are unique to each customer and are fully managed by GCP. Such data includes but is not limited to, customer source code, repository metadata, code host connection configuration, and user profile. Sourcegraph Cloud also has [4 supported regions](https://docs.sourcegraph.com/cloud#multiple-region-availability) on GCP to meet data sovereignty requirements.

Sourcegraph continuously monitors Cloud instances for security vulnerability using manual reviews and automated tools. Third-party auditors regularly perform testing to ensure maximum protection against vulnerabilities and are automatically upgraded to fix any vulnerability in third-party dependencies. In addition, GCP’s managed offering regularly patches any vulnerability in the underlying infrastructure. Any infrastructure changes must pass security checks, which are tested against industry-standard controls.

Access to Cloud instances are not permitted by default to all Sourcegraph teammates and requires an audit-logged approval to escalate permissions.

### What data does Sourcegraph have access to?

For Sourcegraph Cloud to function properly, we will have possession of some customer data. This is a non-exhaustive list and may not be up-to-date. Please refer to our [Security Portal](https://security.sourcegraph.com/) for the latest information.

- source code
- user metadata, such as email address, username, and name
- code host connection credentials
- repository metadata, such as name, labels

### Do Sourcegraph employees have access to my source code?

Customer source code is stored in an isolated GCP project dedicated to the customer, and the data is [encrypted at rest](https://cloud.google.com/docs/security/encryption/default-encryption). Sourcegraph employees do not have access to customer data, and it always requires an audit-logged approval to obtain permissions. Learn more from our compliance report from [Security Portal](https://security.sourcegraph.com/).

### How to protect access to Sourcegraph Cloud instances?

Sourcegraph supports most well-known [identity providers](../admin/auth/index.md) and integrates with customers’ existing Single Sign-On (SSO) solutions. Experimental support for [SCIM](../admin/scim.md) is also available for Azure AD and Okta.

Optionally, access to Cloud instances can be restricted to a list of IP addresses and ranges customers provide.

### How do you detect malicious activity in Sourcegraph Cloud instances?

All Sourcegraph employee access to the instance and underlying infrastructure is audit-logged and continuously analyzed for unexpected behavior. All logs from Sourcegraph Cloud instances are ingested in a SIEM and alerts managed by our Security team.

Sourcegraph Cloud instances have intrusion detection capabilities through [Falco](https://falco.org/), a runtime security tool, to monitor behavior within the server.

### What are uptime guarantees?

Sourcegraph Cloud offers a 99.5% uptime guarantee. Learn more from our [SLA](../sla/index.md#sourcegraph-cloud-sla-managed-instance).

### What is the latency of Sourcegraph Cloud?

Sourcegraph measures latency internally for each service, but because of the variability in response times incurred by a search product, these metrics are not meaningful when aggregated into a single "network latency" figure. Because Sourcegraph’s services operate transactionally across several platforms, latency has proven to be too broad a number that averages out many disconnected performance factors to be useful or meaningful for our customers.

The below are for example purposes only:

In the case of things like Code Search, latency is directly correlated with user input / shape of the query, ex. on our public [sourcegraph.com](https://sourcegraph.com/search) instance:

- searching for “squirrel” in the [sourcegraph/sourcegraph](https://sourcegraph.com/github.com/sourcegraph/sourcegraph) repo takes 30ms and returns 163 results
- searching for “squirrel” in all OSS repos, but only requesting 1000 matches takes 540ms
- searching for all matches of “squirrel” in all OSS repos returns 1.7million results in 30000ms
- In the case of other features, latency of Sourcegraph directly depends on latency / uptime / rate-limits of customer managed systems, ex. for:
- repo syncing (depends on code-host system)
- user account syncing (depends on Identity Provider)
- permission syncing (depends on Identity Provider / Code Host / customer calling our Explicit Permissions API)

### How to connect Sourcegraph Cloud instances to on-prem resources?

Sourcegraph Cloud can securely connect to customer-owned resources on cloud provider platforms or on-prem data centers, such as GitHub Enterprise server in a private network on GCP and Artifact Registry on AWS. Learn more from [Private Connectivity](#private-connectivity).

### How often do Sourcegraph Cloud instances receive updates?

The exact cadence depends on the product release schedule. Sourcegraph Cloud instances receive timely upgrades whenever a new release is available, and there is a need for emergency security patches. Learn more about our [release schedule](https://handbook.sourcegraph.com/departments/cloud/technical-docs/#release-process). The schedule is for information only, and it is subject to change anytime.

### What audit logs are available?

Sourcegraph Cloud instances retain critical audit logs for 30 days by default. Logs are available upon request. For enterprise customers, we can also accommodate for extended retention periods.
