# Welcome to Sourcegraph
This is a compilation of the most important information you will need to get started.


## Sync Code Hosts

Sourcegraph is nothing without your code, so connecting your code hosts is crucial. 

How to connect to…

- [GitHub](https://docs.sourcegraph.com/admin/external_service/github)
- [GitLab](https://docs.sourcegraph.com/admin/external_service/gitlab)
- [Bitbucket Cloud](https://docs.sourcegraph.com/admin/external_service/bitbucket_cloud)
- [Bitbucket Server / Bitbucket Data Center](https://docs.sourcegraph.com/admin/external_service/bitbucket_server)
- [Other Git code hosts (using a Git URL)](https://docs.sourcegraph.com/admin/external_service/other)
- [Non git hosts](https://docs.sourcegraph.com/admin/external_service)


## Inviting users and SSO

A team is essential when using Sourcegraph. Be sure to spread the word and get everybody in.


#### Setting up SSO

Sourcegraph supports out-of-the-box support for different auth providers. Our documentation provides guidance on how to setup these.

- [User authentication (SSO) - Sourcegraph docs](https://docs.sourcegraph.com/admin/auth)

You may not have the right privileges or role in your team to set this up. In that case, you could invite someone with the right privileges and make them a site-admin.


#### Inviting Single Users

Getting users into Sourcegraph is easy, you just need to navigate to:

**Site Admin** → **Users and auth** → **Users** → **+ Create user**



## Setup Email Server

By default the Sourcegraph instance cannot send emails. So features like password resets, email invites and email code monitors will not work.

- [Configure email sending / SMTP server](https://docs.sourcegraph.com/admin/config/email)


## Learn Sourcegraph

Extra resources that we think are helpful:

- [Sourcegraph 101](https://docs.sourcegraph.com/getting-started)
- [Search Examples](https://docs.sourcegraph.com/code_search/tutorials/examples)


## Community and Support

[Join our discord server](https://srcgr.ph/discord-cloud-onboarding). If you need support, reach to via support@sourcegraph.com.
