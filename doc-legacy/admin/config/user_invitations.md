# Sourcegraph user invitations

Starting in Sourcegraph v3.38, the homepage offers the ability for users to invite collaborators:

![image](https://user-images.githubusercontent.com/3173176/157572519-a9876e04-53bd-4134-acbd-19926ffbf616.png)

## How it works

The collaborators you see are determined based on your repository's Git commit history. We sample a few random repositories and use some heuristics to suggest collaborators you may want to invite to Sourcegraph.

When a user is invited, **no additional permissions are granted**: they merely receive an email informing them that the Sourcegraph instance exists. When the invited user visits Sourcegraph, they will have to sign in using the configured authentication providers.

If `"allowSignup": false,` is configured in any of your authentication providers, user invitations are disabled entirely.

## Disabling

You can disable user invitations for all users by setting the feature flag to _false_ in your **global user settings** at e.g. `https://sourcegraph.example.com/site-admin/global-settings` with the following:

```json
{
  "experimentalFeatures": {
    "homepageUserInvitation": false,
  }
}
```

Teams on Sourcegraph.com can disable this via **Your organizations** > **Settings** using the same configuration.

Individuals can disable this in their user settings at `https://sourcegraph.example.com/user/settings` using the same configuration.

If you have any feedback on how we can improve this feature, please [let us know](mailto:feedback@sourcegraph.com)!
