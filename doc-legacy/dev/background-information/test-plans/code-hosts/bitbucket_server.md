# Bitbucket server

> NOTE: While this document is publicly accessible, it is meant as an internal resource. The test plans contain links to internal systems, documents and other information that will not be useful for people outside of Sourcegraph.

This test plan serves the purpose of manually testing if critical functionality of Sourcegraph works with bitbucket server.

## Prerequisites

1. Sourcegraph instance running
1. Site admin account created and functional
1. No prior bitbucket server code host connection setup
    a. If you already have one, delete it before running this test plan
    b. Deleting is better than modifying, because it provides consistent path for everyone who follows the test plan
1. No user with username `engineers` on the instance
    a. This is the user we will use from the [bitbucket.sgdev.org](https://bitbucket.sgdev.org) instance, the username must match a sourcegraph user
1. Builtin auth provider is enabled in site configuration:
    ```
    "auth.providers": [
        {
          "type": "builtin",
        },
        // there might be more auth providers, not important for this test
    ]
    ```

# T1: Repository syncing works
1. Go to `/site-admin/external-services`
1. Click on *Add connection* button
1. Pick *Bitbucket server* from the list
1. Add the following JSON config:
    ```
    {
      "url": "https://bitbucket.sgdev.org",
      "token": "REDACTED",
      "username": "engineers",
      "repos": [
        "SGDEMO/sourcegraph",
        "SGDEMO/mux",
        "SGDEMO/jenkins",
      ],
    }
    ```
1. For the token, copy the value of Token field from [the shared 1Password vault](https://start.1password.com/open/i?a=HEDEDSLHPBFGRBTKAKJWE23XX4&v=dnrhbauihkhjs5ag6vszsme45a&i=2cbhkd47kzhfjfj47ky3v3kw4a&h=team-sourcegraph.1password.com)
1. Scroll down and click *Add connection* button
1. Click *Trigger manual sync* button
1. A new sync job should appear in the *Recent sync jobs* list
1. Wait for the sync to finish. Make sure there are no errors on the sync job displayed in the *Recent sync jobs* list
1. Go to `/site-admin/repositories`
1. In the Code host picker pick the bitbucket code host connection you added
1. Make sure you see 3 repositories in the list:
    - `SGDEMO/sourcegraph`
    - `SGDEMO/mux`
    - `SGDEMO/jenkins`
1. Go to `/search?q=context:global+repo:SGDEMO&patternType=standard&sm=1&groupBy=repo`
1. Make sure that you see the 3 repositories above in the search result list.

# T2: Permission syncing works

## T2.1: Regular users can see repos with no permission enforcement

1. Go to `/site-admin/users`
1. Click on *Create user*
1. For username, type in `engineers`, for email type in your own email with some +suffix, e.g. `milan.freml+bitbucket_server_engineers_user@sourcegraph.com`
1. Click on *Create account and generate password reset link* button at the bottom of the form
1. You should see a message *Account created for engineers*.
1. Click on *Copy* button to copy the reset password URL.
1. Log out of the sourcegraph instance
1. Paste the reset password URL you copied previously into the address field of the browser
1. Follow the instructions to create the user account. Use a reasonable password and store the password somewhere, you will need it later on.
1. Go to `/search?q=context:global+repo:SGDEMO&patternType=standard&sm=1&groupBy=repo`
1. Make sure that you see the 3 repositories in the search result list. 

> NOTE: The code host connection does not enforce permissions, so every Sourcegraph user should be able to see the repositories and code within.

## T2.2: Permission enforcement works correctly

1. Logout and log back in as a site admin
1. Go to `/site-admin/external-services` and click on *Edit* button for the bitbucket connection you created in T1
1. Add the following JSON into the existing config:
    ```
    "authorization": {
        "identityProvider": {
          "type": "username"
        },
        "oauth": {
          "consumerKey": "the_key",
          "signingKey": "the_key"
        }
      },
    ```
1. For `consumerKey` and `signingKey` fields copy the values from the [shared 1Password vault](https://start.1password.com/open/i?a=HEDEDSLHPBFGRBTKAKJWE23XX4&v=dnrhbauihkhjs5ag6vszsme45a&i=2cbhkd47kzhfjfj47ky3v3kw4a&h=team-sourcegraph.1password.com)
1. Go to `/bitbucket.sgdev.org/SGDEMO/sourcegraph/-/settings/permissions` 
1. A permission sync job should appear in the *Permission sync jobs* list
1. Wait for the permission sync to finish, this can take several seconds
1. Check the *Total* column in the list, it should be non-zero as the `engineers` user should have access to the repository
1. Scroll down the page and look at the *Users* list (List of users who have access to the repository).
1. Make sure that the `engineers` user appears in the list with *Permissions Sync* as a reason.
1. Logout and log back in as `engineers` user.
1. Go to `/users/engineers/settings/permissions`
1. Check that you can see 3 repositories in the *Accessible Repositories* list with *Permissions Sync* as a reason:
    - `SGDEMO/sourcegraph`
    - `SGDEMO/mux`
    - `SGDEMO/jenkins`
1. Go to `/search?q=context:global+repo:SGDEMO&patternType=standard&sm=1&groupBy=repo`
1. Make sure that you see the 3 repositories above in the search result list. 

> NOTE: The code host connection does enforce permissions and the user should have all the permissions to the 3 bitbucket repositories

## T2.3: User with no bitbucket account does not see the repositories

1. Logout and log back in as a site-admin
1. Follow steps 1-9 from [T2.1 test](#t21-regular-users-can-see-repos-with-no-permission-enforcement) to create a different user that is not a site admin
1. Go to `/users/YOUR_USERNAME/settings/permissions`
1. Check that the bitbucket repositories are not shown in the *Accessible Repositories* list
1. Go to `/search?q=context:global+repo:SGDEMO&patternType=standard&sm=1&groupBy=repo`
1. Verify that you see no results.

# Test runs

Test runs of this test plan are collected in the [following spreadsheet](https://docs.google.com/spreadsheets/d/1SsyZf6L-Os9ymuUuxvUeYrQzz-Ma0PoSraiKY4HfZG0).
