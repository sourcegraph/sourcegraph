# Site administrator privileges

Site administrators have full administrative access to the Sourcegraph instance. In many cases, they also control the deployment environment. Special privileges are granted to site-admin users.

## Access to all repositories

Site administrators are able to access all repositories on the Sourcegraph instance and manage the settings of individual repositories.

## Access to all GraphQL APIs

Site administrators are able to access all GraphQL APIs of the Sourcegraph instance, including special GraphQL APIs that require a site-admin access token.

## Impersonate regular users

Site administrators are able to impersonate regular users in two ways:
1. View and update the settings of any user.
2. Initiate GraphQL API calls on behalf of any other user.

## Receive site alerts

Site administrators see update notifications and other site-level alerts (visible as a banner across the top of the screen) that may be invisible to non-admin users.
