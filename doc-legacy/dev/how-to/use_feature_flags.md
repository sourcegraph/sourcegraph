# How to use feature flags

This document will take you through how to add, remove, and modify feature flags.

## When to use feature flags

Feature flags, as opposed to experimental features, are intended to be strictly short-lived.
They are designed to be useful for A/B testing, and the values of all active feature flags
are added to every event log for the purpose of analytics.

> Note: We allow feature flags to be overridden per-request by users. So do not use them to gate behaviour that needs to be controlled by an admin.

## How it works

Each feature flag is either a boolean feature flag, or a "rollout" flag.

- A **boolean flag** has a single value (`true` or `false`) for all users that haven't [overriden](#feature-flag-overrides) it.
- A **rollout flag** assigns a random (but stable) value to each user. Each rollout flag is created with a percentage of users that should be randomly assigned the value `true`.
  - The percentage is measured in increments of 0.01% (a "rollout basis point").
  - For example, to create a feature flag that applies to 50% of users, set the rollout basis points of the flag to 5000.

A user is identified either by their user ID (if logged in), or by an anonymous user ID in local storage.

The set of evaluated feature flags is appended to each event log so they can be queried against
for analytics.

## Example lifecycle of a feature flag

### Frontend A/B testing

The standard use of a feature flag for A/B testing a frontend feature will look like the following:

1. [Implement a feature behind a feature flag](#implement-a-feature-flag)
2. Deploy to sourcegraph.com
3. [Create the feature flag](#create-a-feature-flag)
4. [Measure the effect of the feature flag](#measure-the-effect-of-a-feature-flag)
   1. [Update the feature flag](#update-a-feature-flag) if needed
5. [Delete the feature flag](#delete-a-feature-flag)
6. [Remove code that references the feature flag](#remove-code-that-references-the-feature-flag)

## Implement a feature flag

### Frontend

In the frontend, evaluated feature flags map for the current user is available on 
the SourcegraphWebAppState. The map can be prop-drilled into the components that need access to the feature flags.

Ensure that a default value is set for feature flags so that:

1. code can be deployed before creating the feature flag
2. deleting the feature flag is safe before removing referenced code
3. self-hosted deployments continue to work as expected

You can add a new feature flag by adding the name as a new case to the `FeatureFlagName` type:

```typescript
export type FeatureFlagName = ... | 'my-feature-flag'
```

Feature flags are stored in a map with the type `Map<FeatureFlagName, boolean>`. Using `FeatureFlagName` as the map key ensures
that we can only access feature flags defined in the type. Deleting a flag name from `FeatureFlagName` type will also result in a compiler
error if the flag is still referenced somewhere.

Example usage:

```typescript
const [value, status, error] = useFeatureFlag('my-feature-flag')

const [value, status, error] = useFeatureFlag('unknown-feature-flag') // // compiler error
```

### Backend

In the backend, the [`featureflag` package](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/featureflag/middleware.go) provides
the implementation of feature flag functionality.

The entrypoint for most developers that wish to read feature flags should be [the `featureFlag.FromContext` function](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/featureflag/middleware.go?L78-83), which retrieves the current set of feature flags from a request's context object. [`featureFlag.GetBool / featureFlag.GetBoolOr`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@c2c03ab/-/blob/internal/featureflag/flagset.go?L5-15) can then be used to read flags from the [`FlagSet`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@c2c03ab/-/blob/internal/featureflag/flagset.go?L3) that [`featureFlag.FromContext`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/featureflag/middleware.go?L78-83) returns.

Example:

```go
import "github.com/sourcegraph/sourcegraph/internal/featureflag"

flags := featureFlag.FromContext(ctx)
value := flags.GetBoolOr("foo", false)

doSomething(value)
```

When writing code that uses feature flags, you may wish to avoid needing to pass a `context.Context` (for `featureFlag.FromContext()`) in every function that consumes it for a variety of reasons (avoiding mixing concerns, lack of type safety, etc.). See [search: add Features type #28969](https://github.com/sourcegraph/sourcegraph/pull/28969) for an example of a pattern in the search code base that successfully minimizes the need to pass around a full context object.

## Create a feature flag

Depending on how you implement a feature flag, you can disable a feature flag to turn off a feature.
To do so, go to `/site-admin/feature-flags`, click "Create feature flag", and create a flag corresponding to your feature flag name.

There are two types of feature flags—see [How it works](#how-it-works) for more details.

Creating a feature flag can also be done with a GraphQL query like the following from `/api/console`:

```graphql
mutation CreateFeatureFlag{
  createFeatureFlag(
    name: "myFeatureFlag",
    rolloutBasisPoints: 5000,
  ){
    __typename
  }
}
```

## Measure the effect of a feature flag

Feature flags are added as a column to all event logs, so in order to measure any 
effect, there needs to be a related event for it. For example, to compare the number of
`ShareButtonClicked` events between groups where `myFeatureFlag` is enabled and disabled,
you could use a query like the following:

```sql
SELECT 
  JSON_VALUE(feature_flags, '$.myFeatureFlag') AS my_flag, 
  count(*) 
FROM `telligentsourcegraph.dotcom_events.events` 
WHERE name = 'ShareButtonClicked' 
GROUP BY my_flag;
```

## Show usernames using a specific feature flag

If you ever need a list of users that have a feature flag enabled (for example `cody`), you could use a query like the following:

```graphql
query {
  featureFlag(name:"cody") {
    __typename
    ...  on FeatureFlagBoolean {
      overrides {
        namespace {
          ... on User {
            username
            displayName
          }
        }
        value
      }
    }
  }
}
```

## Update a feature flag

Depending on how you implement a feature flag, you can disable a feature flag to turn off a feature or update the rollout basis point value to roll out a feature to more or less users.
To do so, go to `/site-admin/feature-flags`, find your feature flag, and update the value
using the UI.

## Delete a feature flag

In most cases, after an A/B test is performed, a feature flag should be deleted.
Once a feature flag is deleted, it will no longer be added to events as metadata,
so removing the code path that uses it will not change any measurements.

To delete a feature flag, go to `/site-admin/feature-flags`, find your feature flag, and click the "Delete" button.

Deleting a feature flag can also be done with a GraphQL query like the following from `/api/console`:

```graphql
mutation DeleteFeatureFlag{
  deleteFeatureFlag(
    name: "myFeatureFlag",
  ){
    __typename
  }
}
```

### Remove code that references the feature flag

Once [a feature flag is deleted](#disable-or-delete-a-feature-flag), the code that references it can be safely deleted
without changing any of the measurements.

## Feature flag overrides

In addition to feature flags as described above, you can also create feature flag
overrides. This is useful if you'd like to test a feature flag locally by assigning
your user a specific value, or if you'd like to do an A/B test on members of the 
Sourcegraph org. 

Overrides can either apply to a single user or an entire org. If both are set, a user
override takes precedence over an org override.

If an override for a feature flag exists for a user (or the user's org), the value of 
the override will be used instead of the value that would have been randomly selected for a user.

### Creating an override

To create a feature flag override, you can use a graphql query like the following:

```graphql
mutation CreateFeatureFlagOverride{
  createFeatureFlagOverride(
    namespace: "Vx528v=", 
    flagName: "myFeatureFlag",
    value: false,
  ){
    __typename
  }
}
```

The `namespace` argument is the graphql ID of either a user or an organization.

### Override for a single request

We also allow overriding the value for a specific request. This can be done by setting the header `X-Sourcegraph-Override-Feature` or the URL query parameter `feat`. For example `?feat=myFeatureFlag1&feat=-myFeatureFlag2` will set `myFeatureFlag1` to `true` and `myFeatureFlag2` to `false`.

An example use of this is search has the feature flag `search-debug` which when enabled will enrich responses with debug information.

## Listing all feature flags

To view a list of all current feature flags on a Sourcegraph instance, go to `/site-admin/feature-flags`.

Listing feature flags can also be done with a GraphQL query like the following from `/api/console`:

```graphql
query FetchFeatureFlags {
  featureFlags {
    ... on FeatureFlagBoolean {
      name
      value
    }
    ... on FeatureFlagRollout {
      name
      rolloutBasisPoints
    }
  }
}
```

## Further reading

- Initial proposal: [RFC 286](https://docs.google.com/document/d/1aT8uI3mUXpm9IK9_WbXhFM5ahHj9KQeQ521hd9EE5U8/edit)
