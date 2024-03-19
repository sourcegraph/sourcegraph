<style>
.limg {
  list-style: none;
  margin: 3rem 0 !important;
  padding: 0 !important;
}
.limg li {
  margin-bottom: 1rem;
  padding: 0 !important;
}

.limg li:last {
  margin-bottom: 0;
}

.limg a {
    display: flex;
    flex-direction: column;
    transition-property: all;
   transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
     transition-duration: 350ms;
     border-radius: 0.75rem;
  padding-top: 1rem;
  padding-bottom: 1rem;

}

.limg a:hover {
  padding-left: 1rem;
  padding-right: 1rem;
  background: rgb(113 220 232 / 19%);
}

.limg p {
  margin: 0rem;
}
.limg a img {
  width: 1rem;
}

.limg h3 {
  display:flex;
  gap: 0.6rem;
  margin-top: 0;
  margin-bottom: .25rem

}

th:first-child,
td:first-child {
   min-width: 200px;
}

.markdown-body table thead tr{
  border-top:0;
}

.markdown-body table th, .markdown-body table td {
    text-align: left;
    vertical-align: baseline;
    padding: 0.5714286em;
}

.markdown-body table tr:nth-child(2n) {
  background: unset;
}

.markdown-body table th, .markdown-body table td {
    border: none;
}

.markdown-body .cards {
  display: flex;
  align-items: stretch;
}

.markdown-body .cards .card {
  flex: 1;
  margin: 0.5em;
  color: var(--text-color);
  border-radius: 4px;
  border: 1px solid var(--sidebar-nav-active-bg);
  padding: 1.5rem;
  padding-top: 1.25rem;
}

.markdown-body .cards .card:hover {
  color: var(--link-color);
}

.markdown-body .cards .card span {
  color: var(--link-color);
  font-weight: bold;
}
</style>

# SLAs and Premium Support

<p class="subtitle">This document explains the Sourcegraph's default contractual Service Level Agreements and Premium Support Offerings.</p>

## Service Level Agreements (SLAs)

Our service level agreements (SLAs) are designed for products that are generally available and exclude [beta and experimental features](../admin/beta_and_experimental_features.md). SLA response times indicate how quickly we aim to provide an initial response to your inquiries or concerns. Our team will resolve all issues as quickly as possible. However, it's important to understand that SLA times differ from guaranteed resolution times.

While we always strive to respond to your issues as quickly as possible, our SLAs are specifically applicable from Monday through Friday.

The following policy applies to both our cloud-based (managed instance) and on-premise/self-hosted Sourcegraph customers:

## For enterprise plans

| Severity level | Description | Response time | Support availability |
| -------------- | ----------- | ------------- | -------------------- |
| 0 | Emergency: Total loss of service or security-related issue (includes POCs) | Within two business hours of identifying the issue | 24x5 (Monday-Friday) |
| 1 | Severe impact: Service significantly limited for 60%+ of users; core features are unavailable or extremely slowed down with no acceptable workaround | Within four business hours of identifying the issue | 24x5 (Monday-Friday) |
| 2 | Medium impact: Core features are unavailable or somewhat slowed; workaround exists | Within eight business hours of identifying the issue | 24x5 (Monday-Friday) |
| 3 | Minimal impact: Questions or clarifications regarding features, documentation, or deployments | Within two business days of identifying the issue | 24x5 (Monday-Friday) |

### For enterprise Starter plans

| Severity level | Description | Response time | Support availability |
| -------------- | ----------- | ------------- | -------------------- |
| 0 | Emergency: Total loss of service or security-related issue (includes POCs) | Within four business hours of identifying the issue | 24x5 (Monday-Friday) |
| 1 | Severe impact: Service significantly limited for 60%+ of users; core features are unavailable or extremely slowed down with no acceptable workaround | Within six business hours of identifying the issue | 24x5 (Monday-Friday) |
| 2 | Medium impact: Core features are unavailable or somewhat slowed; workaround exists | Within 16 business hours of identifying the issue | 24x5 (Monday-Friday) |
| 3 | Minimal impact: Questions or clarifications regarding features, documentation, or deployments | Within three business days of identifying the issue | 24x5 (Monday-Friday) |

>NOTE: Premium support with enhanced SLAs can be added to your Enterprise plans as an add-on. Our business hours, defined as Sunday 2 PM PST to Friday 5 PM PST, align with our 24x5 support coverage.

## Sourcegraph cloud SLA (managed instance)

>NOTE: Effective: November 14, 2022 — Sourcegraph gives customers a 99.5% uptime commitment on the Business and Enterprise plans.

### Downtime

Downtime is the total number of minutes when the Sourcegraph Cloud instance was unavailable during a calendar month. Sourcegraph calculates unavailability using server monitoring software to assess server-side error rates, ping test outcomes, web server evaluations, TCP port examinations, and website tests.

It's important to note that, due to the single-tenant architecture of Sourcegraph Cloud, downtime is measured individually for each customer.

The following items are not considered as part of downtime:

- Slowness or other performance issues specific to individual Sourcegraph features
- Issues that are linked to external applications or third-party services, including authentication providers and code hosts
- Any products or features labeled as experimental or in beta
- External network problems beyond our reasonable control, including connectivity issues involving the client's Internet Service Provider (ISP), Cloudflare, or Google Cloud Platform
- Maintenance activities are conducted during Scheduled Downtime.

### Scheduled downtime

Scheduled downtime is occasionally required to maintain the proper functioning of your Sourcegraph Cloud instance. In case of such a scheduled downtime, we will provide you with a minimum of 48 hours' advance notice. Please note that the total scheduled downtime will be at most 10 hours in a quarter.

### Uptime commitment

Uptime is the percentage of total possible minutes Sourcegraph was available during a calendar month. Our commitment is to maintain at least 99.5% uptime. You can calculate your uptime by using the following formula:

```
[(total minutes in month - Downtime) / total minutes in month] > 99.5%
```

### Sev 0 - Emergency support scope

#### What constitutes a Sev 0 - Emergency?

A Severity 0 - Emergency is defined by the following criteria:

- The Sourcegraph instance is entirely unavailable or unresponsive for all users. This means the service is inaccessible, and users cannot use any features
- Sourcegraph's web user interface displays 4XX or 5XX HTTP error codes on every page. These errors indicate a fundamental problem with the service
- All users are unable to log in to the Sourcegraph instance. Authentication is not functioning, preventing user access
- A security-related incident poses a significant risk or exposure to the system or data. This may include vulnerabilities, breaches, or other security issues that demand immediate attention to protect the instance and its users

#### What is not a Sev 0 - Emergency?

A Severity 0 - Emergency typically does not include the following situations:

- If only one user is experiencing login issues while most users can still access the system
- Performance is slower than usual
- Any issues with repository synchronization

>NOTE: Custom support agreements apply to Enterprise Plus and Elite customers. Please consult your contract for details regarding custom service-level agreements.

## Sourcegraph Premium Support

### Benefits of Premium Support

Purchasing Premium Support provides several of the following services beyond those provided by Sourcegraph's enterprise or enterprise starter support models:
- Accelerated Service Level Agreements (SLAs) - Accelerated triaging and queue priority lead to reduced times to resolution.
- Elevated Support Expertise - Two named, dedicated senior support engineers possessing deep knowledge and understanding of your specific organization’s setup and tech stack with the ability to assist in the resolution of issues that arise.
- Heightened Support via Slack - Faster response times and closer collaboration through direct access to Sourcegraph experts with closer connection to end users and administrators.
- More Proactive Collaboration - Hands-on technical guidance to ensure successful migrations, upgrades, and maintenance; early access to new features; quarterly insights reports.

### Premium Support SLAs

Premium Support SLAs give customers access to our Support team 24x7 for Severity 0 and 1 issues that critically impact your instance and ability to use Sourcegraph.

First response times remain the same according to our SLAs. However, Emergency and Severe Impact issues are supported 24x7 with Premium Support (versus 24x5 without Premium Support).  

This service is provided for all GA products Sourcegraph offers, not for any Experimental or Beta features (see here for more details).

>NOTE: This package includes access to Slack Support & Slack Account Management

### Dedicated Support

Access to named senior support engineers who will have knowledge of your infrastructure and can help reduce the time needed to triage, diagnose, and resolve an issue in your instance.

>NOTE: This package includes access to Slack Support & Slack Account Management

### Slack Support & Slack Account Management

Slack Support provides access to creating tickets directly from Slack, allowing customers to resolve tickets directly from a familiar interface and allowing for greater collaboration between Support and customers.

Slack Account Management allows customers direct access to their Account team, including their Technical Advisor (if applicable). It can help resolve any non-technical issues or questions that come up.
