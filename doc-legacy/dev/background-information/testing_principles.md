# Testing principles and guidelines

<span class="badge badge-note">SOC2/GN-105</span>

This page documents how we test code at Sourcegraph.

## Policy

We rely on automated testing to ensure the quality of our product. Our goal is to ensure that our product and code work, and that all reasonable effort has been taken to reduce the risk of a security-related incident associated to Sourcegraph.

Any addition or change to our codebase should be covered by an appropriate amount of [automated tests](#types-of-tests) or [other testing strategies](#other-testing-strategies) to ensure that:

1. Our product and code works as intended when we ship it to customers.
1. Our product and code doesn't accidentally break as we make changes over time.

Engineers should budget an appropriate amount of time for ensuring [test plans](#test-plans) are made for all changes when making iteration plans.

### Test plans

**All pull requests must provide test plans** that indicate what has been done to test the changes being introduced. This can be done with a "Test plan" section within a pull request's description.

These plans are here to demonstrate we're complying with industry standards which are critical for our customers. They may be read by auditors, customers, who are seeking to understand if with we uphold these standards. 
Therefore, it's perfectly fine to be succint as long as the substance is here. And because the audience for these test plans are engineers, they will understand the context. Changing a README for example can simply covered by stating you rendered it locally and it was fine.

Some pull requests may not require a rigorous test plan in certain situations, see [Exceptions](#exceptions).

## Types of tests

In order to ensure we are true to our [testing policy](#policy), we have various implementations of [automated testing](#automated-tests) for our code base. In addition, we have [other ways to ensure changes are appropriately tested](#other-testing-strategies).

### Automated tests

A good automated test suite increases the velocity of our team because it allows engineers to confidently edit and refactor code, especially code authored by someone else. This includes, but is not limited to:

- Image vulnerability scanning
- Infrastructure as code static analyses
- [Unit](#unit-tests), [integration](#integration-tests), [end-to-end](#end-to-end-tests-e2e), and [visual](#visual-testing) tests

The testing pyramid is a helpful way to determine the most appropriate type of test when deciding how to test a change:

![Testing pyramid](testing-pyramid.svg)

The closer a test is to the top, the larger the scope of that test is. It means that failures will be harder to link to the actual cause. Tests at the top are notoriously slower than at the bottom.

It's important to take these trade-offs into account when deciding at which level to implement a test. Please refer to each testing level below for more details.

#### Unit tests

Unit tests test individual functions in our codebase and are the most desirable kind of test to write.

Benefits:

- They are usually very fast to execute because slow operations can be mocked.
- They are the easiest tests to write, debug, and maintain because the code under test is small.
- They only need to run on changes that touch code which could make the test fail, which makes CI faster and minimizes the impact of any [flakiness](#flaky-tests).

Tradeoffs:

- They don't verify our systems are wired up correctly end-to-end.

#### Integration tests

Integration tests test the behavior of a subset of our entire system to ensure that subset of our system is wired up correctly.

Benefits:

- To the extent that fewer systems are under test compared to e2e tests, they are faster to run, easier to debug, have clearer ownership, and less vulnerable to [flakiness](#flaky-tests).
- They only need to run on changes that touch code which could make the test fail, which makes CI faster and minimizes the impact of any [flakiness](#flaky-tests).
- For UI behavior, they run in an actual browser—rather than a JSDOM environment.

Tradeoffs:

- They don't verify our systems are wired up correctly end-to-end.
- They are not as easy to write as unit tests.

Examples:

- Tests that call our search API to test the behavior of our entire search system.
- Tests that validate UI styles, through [visual testing](#visual-testing).
- Tests that validate UI behavior in the browser while mocking out all network requests so no backend is required.
  - Note: We still typically prefer unit tests here, only fall back to integration tests if you need to test some very specific behavior that cannot be covered in a unit test.

#### End-to-end tests (e2e)

E2e tests test our entire product from the perspective of a user. We try to use them sparingly. Instead, we prefer to get as much confidence as possible from our [unit tests](#unit-tests) and [integration tests](#integration-tests).

> WARNING: You should generally avoid writing e2e tests. If required, they should be as simple as possible and only aim for basic tests on core behavior (e.g. check homepage loads, check sign-in works).

Benefits:

- They verify our systems are wired up correctly end-to-end.

Tradeoffs:

- They are typically the slowest tests to execute because we have to build and run our entire product.
- They are the hardest tests to debug because failures can be caused by a defect anywhere in our system. This can also make ownership of failures unclear.
- They are the most vulnerable to [flakiness](#flaky-tests) because there are a lot of moving parts.

Examples:

- Run our Sourcegraph Docker image and verify that site admins can complete the registration flow.
- Run our Sourcegraph Docker image and verify that users can sign in and perform a search.

#### Visual testing

Visual testing is useful to catch visual regressions and verify designs for new features. [More info about visual testing philosophy](testing_web_code.md#visual-regressions)

We use [Chromatic Storybook](https://www.chromatic.com/) to detect visual changes in specific React components. Post a message in #dev-chat that you need access to Chromatic, and someone will add you to our organization (you will also receive an invitation via e-mail). You should sign into Chromatic with your GitHub account. If a PR you author has visual changes, a UI Review in Chromatic will be generated. It is recommended that a designer approves the UI review.

We use [Percy](https://percy.io/) to detect visual changes in Sourcegraph features during browser-based tests (client integration tests and end-to-end tests). You may need permissions to update screenshots if your feature introduces visual changes. Post a message in #dev-chat that you need access to Percy, and someone will add you to our organization (you will also receive an invitation via e-mail). Once you've been invited to the Sourcegraph organization and created a Percy account, you should then link it to your GitHub account.

### Other testing strategies

- Targeted [code reviews](pull_request_reviews.md) can help ensure changes are appropriately tested.
  - If a change contains changes pertaining to the processing or storing of credentials or tokens, authorization, and authentication methods, the `security` label should be added and a review should be requested from members of the [Sourcegraph Security team](https://handbook.sourcegraph.com/departments/product-engineering/engineering/cloud/security)
  - If a change requires changes to self-managed deployment method, get a review from the [Delivery team](https://handbook.sourcegraph.com/departments/engineering/teams/delivery/).
  - If a change requires changes to Cloud (managed instances), get a review from the [Cloud team](https://handbook.sourcegraph.com/departments/cloud/).
  - Performance-sensitive changes should undergo reviews from other engineers to assess potential performance implications.
- Deployment considerations can help test things live, detect when things have gone wrong, and limit the scope of risks.
  - For high-risk changes, consider using [feature flags](../how-to/use_feature_flags.md), such as by rolling a change out to just Sourcegraph teammates and/or to a subset of production customers before rolling it out to all customers on Sourcegraph Cloud managed instances or a full release.
  - Introduce adequate [observability measures](observability/index.md) so that issues can easily be detected and monitored.
- Documentation can help ensure that changes are easy to understand if anything goes wrong, and should be added to [sources of truth](https://handbook.sourcegraph.com/company-info-and-process/communication#sources-of-truth).
  - If the documentation will be published to docs.sourcegraph.com, it can be tested by running `sg run docsite` and navigating to the corrected page.
- Some changes are easy to test manually—test plans can include what was done to perform this manual testing.

## Exceptions

If for a situational reason, a pull request needs to be exempted from the testing guidelines, skipping reviews or not providing a [test plan](#test-plans) will trigger an automated process that create and link an issue requesting that the author document a reason for the exception within [sourcegraph/sec-pr-audit-trail](https://github.com/sourcegraph/sec-pr-audit-trail).

### Why does it matter?

In order to comply with industry standards that we share with our customers, we have to demonstrate that even when we're deviating from the standard process we're taking the necessary actions. This exception process is what gives us the flexibility to deal with edge cases when the normal process would slow us down to land some changes that we need to be merged right now. This automated exception mechanism gives us flexibility rather than forcing us to blindly comply with the process even if the situation clearly requires to go around it. 

Remember that auditors may look at these exception explanations, and might eventually ask you about what happened exactly six month from now. It's a small price to pay for a bit of flexibility. The created issues contains example on how to explain the most common scenarios to help you write a good explanation.

### Fixed exceptions

The list below designates source code exempt from the testing guidelines because they do not directly impact the behaviour of the application in any way.

- [sourcegraph/sourcegraph](https://github.com/sourcegraph/sourcegraph)
  - `dev/*`: internal tools, scripts for the local environment and continuous integration.
  - Dev environment configuration (e.g. `.editorconfig`, `shell.nix`, etc.)

To indicate exceptions like these, simply write `n/a` within your pull request's [test plan](#test-plans).

### Pull request review exceptions

Certain workflows leverage PRs that deploy already-tested changes or boilerplate work.
For these PRs a review may not be required. This can be indicated by creating a section within your test plan indicating `No review required:`, like so:

```md
## Test plan

No review required: deploys tested changes.
```

Or, you may also attach `automerge` or `no-review-required` label to the PR to indicate the status of the PR and include a normal test plan without the `No review required:` prefix.


## Test health

### Failures on the `main` branch

**A red `main` build is not okay and must be fixed.** Consecutive failed builds on the `main` branch means that [the releasability contract is broken](https://handbook.sourcegraph.com/engineering/continuous_releasability#continuous-releasability-contract), and that we cannot confidently ship that revision to our customers nor have it deployed in the Cloud environment.

### Flaky tests

**We do not tolerate flaky tests of any kind.** Any engineer that sees a flaky test in [continuous integration](./ci/index.md) should immediately [disable the flaky test](ci/index.md#flaky-tests).

Why are flaky tests undesirable? Because these tests stop being an informative signal that the engineering team can rely on, and if we keep them around then we eventually train ourselves to ignore them and become blind to their results. This can hide real problems under the cover of flakiness.

When fixing a flaky test, make sure to re-run the test in a loop to assess whether the fix actually worked. ([Go example](languages/testing_go_code.md#verifying-fixes-to-flaky-tests))

Other kinds of flakes include [flaky steps](ci/index.md#flaky-steps) and [flaky infrastructure](ci/index.md#laky-infrastructure)

#### Analytics about flakes 

Our pipeline exports test results to Buildkite Test Analytics. Rather than doing it at the individual test level, we record them at the test suite level (Bazel test targets). Therefore, any kind of test suite running with Bazel is recorded. 
You can find the statistics for all targets in the [`sourcegraph-bazel`](https://buildkite.com/organizations/sourcegraph/analytics/suites/sourcegraph-bazel?branch=main) dashboard. 

These numbers are extremely useful to prioritize work toward making a test suite more stable: you can instantly see if a test suite is failing often and costs time to every engineer, or if that's a rare occurence.

## Ownership

- [DevX Team](https://handbook.sourcegraph.com/engineering/enablement/dev-experience) owns build and test infrastructure (also see [build pipeline support](https://handbook.sourcegraph.com/departments/product-engineering/engineering/enablement/dev-experience#build-pipeline-support)).
- [Frontend Platform Team](https://handbook.sourcegraph.com/engineering/enablement/frontend-platform) owns any tests that are driven through the browser.
- All other tests and tools used in tests are owned by the teams responsible for the domain being tested.

## Reference

- [Continuous integration](ci/index.md)
- [How to write and run tests](../how-to/testing.md)
- [Testing Go code](languages/testing_go_code.md)
- [Testing web code](testing_web_code.md)
- [Pull request reviews](pull_request_reviews.md)
