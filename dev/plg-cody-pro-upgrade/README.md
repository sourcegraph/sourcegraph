# PLG Cody Pro Upgrade script

We use this script to give Cody Pro to our teammates.

## Usage

Run the script using

```bash
go run add_member.go [NEW_MEMBER_ACCOUNT_ID] [SAMS_SESSION_COOKIE]
```
replacing the placeholders with actual values.

## Adding teammates to Cody Pro

Here is the complete process:

1. Use [Redash](https://redash.sgdev.org/queries/614/source) to get the SAMS ID.
    - If a user doesn't have a `sams_account_id`, it means they don't have a SAMS account because the user hasn't logged into dotcom since the SAMS rollout on 2024-02-08.
2. Add the SAMS account to the Sourcegraph team
    - Someone with `cody-admin` SSC superpowers needs to run this.
    - Go to https://accounts.sourcegraph.com/ and in Dev Tools → Application → Cookies → https://accounts.sourcegraph.com/, copy the value for the `accounts_session_v2` cookie.
    - Run `sourcegraph/sourcegraph/dev/plg-cody-pro-upgrade/main.go {SAMS_ID} {SESSION_COOKIE}`.

## Troubleshooting

- "401 Unauthorized" means that your session ID is wrong our you don't have the SSC superpowers needed.
- A pair of "201 Created" and "200 OK" means the SSC account was created, and team membership was set.
- A pair of "400 Bad Request" and "200 OK" means the SSC account already existed, the team membership was set.
- A pair of "400 Bad Request" and "400 Bad Request" means that it was already all set.
