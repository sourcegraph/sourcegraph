# Pull request reviews <span class="badge badge-note">SOC2/GN-98</span> <span class="badge badge-note">SOC2/GN-104</span>

Sourcegraph has a formal software development life cycle methodology and supporting procedures to help manage application and infrastructure changes.

All contributions to Sourcegraph must be reviewed and approved before being merged into the `main` branch. This includes code changes, documentation changes, and more.

GitHub repositories are configured to prevent merging without a review (including administrators), but in the eventuality of a pull request being merged without review, 
[`pr-auditor`](https://github.com/sourcegraph/sourcegraph/tree/main/dev/pr-auditor) will open an exception in the [audit trail](https://github.com/sourcegraph/sec-pr-audit-trail/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc) where the teammate who merged the pull request can document the exception.

Our goal is to have a pull request review process and culture that everyone would opt-in to even if reviews weren't required. 

In addition to being peer-reviewed, all contributions must pass our [continuous integration](./ci/index.md).

## Why do we require peer reviews?

Reviews are an investment. As an author, it takes extra time and effort to create good PRs, wait for someone to review your contribution, and address review comments. As a reviewer, it takes extra time to understand the motivation, design, and implementation of someone else's contribution so that you can provide valuable feedback. This investment is worth it because it provides benefits to every person on the team and it helps the team as a whole ship features to our customers that are _high quality_ and _maintainable_.

For authors:

- Requesting a review from one of your peers is motivation to ensure that the quality of your contribution is high.
- Writing a good PR description (and [commit messages](commit_messages.md)) develops your technical communication skills.
- Receiving review feedback can give you valuable insight into aspects of your change that you hadn't considered (architectural, performance, etc).
- Receiving feedback about how your code could be improved helps you learn how to write better code in the future.

For reviewers:

- Reviewing code from others increases your code literacy and empathy. You will learn new things from how your peers write code and you will be more aware of how to write code that is easier for your reviewers to understand.
- Reading PR descriptions increases your technical literacy and empathy. You will learn what makes a PR description effective and you will be more aware of how to write PR descriptions that are easier for your reviewers to understand.
- Being a reviewer gives you the ability to share knowledge you have that others do not. Every person, regardless of experience level, has both something to learn and something to contribute.

For the team:

- reviews increase the overall quality of our codebase by catching design and implementation defects before changes ship to customers.
- reviews increase the overall consistency of our codebase by spreading knowledge of best practices and eliminating anti-patterns.
- reviews distribute domain expertise, code knowledge, and ownership across the team so development can continue when people are unavailable (e.g. vacation).

Also see [When is an in-depth review not required?](#when-is-an-in-depth-review-not-required) for edge cases.

## Review cycles

Once a change is ready for review it may go through a few review cycles, in general it'll look something like this:

1. [Ready for review](#authoring-pull-requests)
2. [Reviewers leave feedback](#reviewing-pull-requests)
3. Author addresses feedback, ensuring they have marked each comment as resolved. This can be relaxed if a comment was just a suggestion or a small nit. The goal is to communicate to the reviewer what has changed since they last reviewed.
4. Author leaves a comment letting the reviewers know it is ready for another pass. Back to step 1.

Use your best judgement when deciding if another review cycle is needed. If your PR has been approved and the only changes you've made are ones that the approver would expect: accepting a suggestion around a name, or fleshing out some documentation, or something else that is low risk and you're comfortable with, you may choose to avoid another cycle and merge. However, you can always ask for more feedback if you're not totally comfortable.

### GitHub notifications

Ensure that you have you [GitHub notification settings](https://handbook.sourcegraph.com/engineering/github-notifications) configured correctly so that you are responsive to comments on PRs that you have authored and to review requests from teammates throughout review cycles.

## Authoring pull requests

### What makes an effective Pull Request (PR)?

An effective PR minimizes the amount of effort that is required for the reviewer to understand your change so that they can provide high quality feedback in a timely manner. [Further reading](https://www.atlassian.com/blog/git/written-unwritten-guide-pull-requests).

Do:

- Prefer small and incremental PRs. If a large PR is unavoidable, try to organize it into smaller commits (with [good commit messages](commit_messages.md)) that are reviewable independently (and indicate this to the reviewer in the description).
- Create a draft PR first and review your own diff as if you were reviewing someone else's change. This helps you empathize with your reviewer and can help catch silly mistakes before your reviewer sees them (e.g. forgetting to `git add` a file, forgetting to remove debugging code, etc.).
- Create meaningful PR title and description that communicates **what** the PR does and **why** (the **how** is the diff).
  - Include links to relevant issues (e.g. "closes #1234", "helps #5678").
  - Include a screenshot, GIF, or video if you are making user facing changes.
  - Discuss alternative implementations that you considered but didn't pursue (and why).
  - Describe any remaining open questions that you are seeking feedback on.
- Make it clear whose review(s) you are waiting for.
- Politely remind your reviewer that you are waiting for a review if they haven't responded after one business day.
- If you need to work on your PR after a first round of reviews, try not to force push commits that erase the latest reviewed commit or any commit prior to that. This makes it easy for reviewers to pick up where they left off in the previous round.

Don't:

- Submit a PR that would cause the main branch to [non-releaseable](https://handbook.sourcegraph.com/engineering/continuous_releasability).
- Submit PRs with multiple overlapping concerns. Every PR should have an obvious goals. PRs tackling multiple issues at once should be split into more highly-focused PRs when possible. For example, a necessary refactor for a feature addition should be split into a (prerequisite) refactor PR, followed by a (subsequent) feature addition PR. Highly cohesive PRs enable reviewers to effectively hold the state of the change in their head.

### Who should I get a review from?

You should get a review from the person who's approval will give you the most confidence that your change is high quality. If you are modifying existing code, you can use `git blame` to identify the previous author and the previous reviewer because they probably will have helpful context.

If your change touches multiple parts of our codebase (e.g. Go, TypeScript), then you will need to get approval from multiple peers (e.g. a Go reviewer and a TypeScript reviewer).

GitHub will automatically assign reviewers if there is a matching entry in the [CODEOWNERS](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/.github/CODEOWNERS) file, but that doesn't necessarily mean that you need to wait for an approval from everyone. For example, if you are making a change to the search backend then you only need approval from one person on that team, not all of them.

For small changes across few files assign to the team and request a review.

Use your best judgement and ask if you are uncertain.

In some cases, [reviewers may opt to give an approval without performing an in-depth review](#when-is-an-in-depth-review-not-required).

### When should I use a draft PR?

A draft PR signals that the change is not ready for reviewed. This is useful, for example, if you want to self-review your diff before sending review requests to others. If you are looking for feedback or discussion on your change, then you should mark the PR as ready for review and communicate your intentions in the PR description.

You may also set a ready-for-review PR *back* to a draft state to signal that work is being done that shouldn't be interesting (yet), and that potential reviewers should check back later.

## Reviewing pull requests

### What makes an effective review?

Please read:

- [On reviews](https://medium.com/@schrockn/on-code-reviews-b1c7c94d868c)
- [Code Health: Respectful Reviews == Useful Reviews](https://testing.googleblog.com/2019/11/code-health-respectful-reviews-useful.html)

The code author and reviewer have a _shared goal_ to bring the PR into a state that meets our quality standards and the needs that motivated the change.

Do:

- Take time to understand the context and goal of the PR. If it isn't clear, ask for clarification.
- Acknowledge what the author did well to balance the tone of the review.
- Make it clear which comments are blocking your explicit approval (e.g. use "nit:" prefix for minor comments) and approve if all of your comments are minor.
- If the author were to address all of your comments faithfully and you would be content, then you should also approve to avoid the author needing to wait for a subsequent review without reason (exception: you asked for fundamental or vast/large changes and believe those will need re-review by you).
- When you are making comments on a PR, use a tone that is kind, empathetic, collaborative, and humble. [Further reading](https://mtlynch.io/human-code-reviews-1/).

| 😕                                              | 🤗                                                                                                                              | Why?                                                                                                                                                                                                                                                                                               |
| ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| You misspelled "successfully"                   | sucessfully -> successfully                                                                                                     | Avoid using "you" because it can make the comment feel like a personal attack instead of just a minor correction.                                                                                                                                                                                  |
| Move the `Foo` class to a separate file.        | Can we move the `Foo` class to a separate file to avoid large files that are hard to scan?                                      | Feedback that is framed as a request encourages collaborative conversation. Including your reason helps the author understand the motivation behind your request.                                                                                                                                  |
| Can we simplify this with a list comprehension? | Consider simplifying with a list comprehension like this: $EXAMPLE or $LINK_TO_EXAMPLE                                          | Providing example code (using GitHub suggestions) or a link to an example helps the author quickly understand and apply your recommended change. This is particularly helpful when the author might not be familiar with concept that you are describing.                                          |
| Why didn't you \$X?                             | What do you think about doing $X? I think it would help with $Y.                                                                | A "What" question is better than a "Why" question because the latter sounds accusatory. Either the author considered $X and decided not to, or they didn't consider it, but in either case it is frustrating to the author if the reviewer doesn't explain why they think $X should be considered. |
| This code is confusing. Can you clarify it?     | It was hard for me to understand $X because $Y, but I haven't been able to think of a way to improve it. Do you have any ideas? | Don't declare your opinion as fact; instead, share your subjective experience and try to offer a suggestion for improvement. If you don't have a suggestion, acknowledge that to show that you empathize with the difficulty of improving this code.                                               |

Don't:

- Comment on the details of the PR if you have questions or concerns about the overall direction or design. The former will distract from the latter and might be irrelevant if the PR is reworked.
- Take longer than one business day to respond to a PR that is ready for your review (or re-review).
- Have protracted discussions in PR comments. If it can't be settled quickly in a few review round trips, try discussing in person or on a video call because these mediums are higher bandwidth and encourage empathy. Then summarize the results in the PR discussion after the fact.

### Security review

Special care should be taken when reviewing a diff that contains, or is adjacent to, comments that contain the following string: `🚨 SECURITY`. These comments indicate security-sensitive code, the correctness of which is necessary to ensure that no private data is accessed by unauthorized actors. The code owner of any modified security-sensitive code must approve the changeset before it is merged. Please refer to [Security patterns](security_patterns.md) for considerations when touching security-sensitive code.

### When is an in-depth review not required?

We do technically prevent PRs from being merged before being explicitly approved in order to meet security certification compliance requirements. However, there exist PRs that are appropriate to merge without waiting for a full review—in these cases, a reviewer may opt to give an approval without performing an in-depth review, and the author may opt to merge the PR without waiting for a reviewer to provide an in-depth review.

> NOTE: When in doubt, reviewers should prefer to provide an in-depth review, and authors should prefer to wait for an in-depth review.

Some valid examples include:

- Reverting a previous change to solve a production issue.
- Minor documentation changes.
- Auto-generated changes, such as when performing a version release, where a reviewer would not have anything of substance to review.

It should be obvious to any other teammate from the PR description and/or diff that it was appropriate for you to not wait for approval.

Here are some examples of reasons to skip review that are NOT acceptable:

- "I promised that I would ship this to $CUSTOMER by $DEADLINE"
  - The customer expects the feature to work and be maintained. review helps ensure both of these things by increasing the quality and distributing ownership.
- "This code is experimental"
  - Our goal is to have a review culture such that engineers who are working on "experimental" code still find reviews valuable and worth doing (for all the benefits mentioned in the rest of this document).
  - All code that is in `main` has the potential to impact customers (e.g. by causing a bug) and other developers at Sourcegraph (e.g. by making it harder to refactor code). As such, it is in our interest to ensure a certain quality level on all code whether or not it is considered "experimental".
  - Assume that we allowed "experimental" code to bypass review. How would we know when it is no longer experimental and how would it get reviewed? Either it wouldn't get reviewed, or an engineer would have to review all the code after the fact without a nice PR diff to look at or effective way to make comments. Neither of these outcomes would meet our need of reviewing all non-experimental code.
- "I don't have someone to review this code"
  - Ask for help to identify someone else on the team with whom you can share your knowledge, context, and ownership.
