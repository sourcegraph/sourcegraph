# Sourcegraph Secret Formats

Sourcegraph uses a number of secret formats to store authentication tokens and keys. This page documents each secret type, and the regular expressions that can be used to match each format.

| Token Name                                   | Description                                                                      | Type                       | Regular Expression                                |
| -------------------------------------------- | -------------------------------------------------------------------------------- | -------------------------- | ------------------------------------------------- |
| Sourcegraph Access Token (v3)                | Token used to access the Sourcegraph GraphQL API                                 | User-generated             | `sgp_(?:[a-fA-F0-9]{16} \|local)_[a-fA-F0-9]{40}` |
| Sourcegraph Access Token (v2, deprecated)    | Token used to access the Sourcegraph GraphQL API                                 | User-generated             | `sgp_[a-fA-F0-9]{40}`                             |
| Sourcegraph Access Token (v1, deprecated)    | Token used to access the Sourcegraph GraphQL API                                 | User-generated             | `[a-fA-F0-9]{40}`                                 |
| Sourcegraph Dotcom User Gateway Access Token | Token used to grant sourcegraph.com users access to Cody                         | Backend (not user-visible) | `sgd_[a-fA-F0-9]{64}`                             |
| Sourcegraph License Key Token                | Token used for product subscriptions, derived from a Sourcegraph license key     | Backend (not user-visible) | `slk_[a-fA-F0-9]{64}`                             |
| Sourcegraph Product Subscription Token       | Token used for product subscriptions, not derived from a Sourcegraph license key | Backend (not user-visible) | `sgs_[a-fA-F0-9]{64}`                             |

For further information about Sourcegraph Access Tokens, see:
- [Creating an access token](../../cli/how-tos/creating_an_access_token.md)
- [Revoking an access token](../../cli/how-tos/revoking_an_access_token.md)

Sourcegraph is in the process of rolling out a new Sourcegraph Access Token format. When generating an access token you may receive a token in v2 or v3 format depending on your Sourcegraph instance's version. Newer instances are fully backwards-compatible with all older formats.
