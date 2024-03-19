# How to conduct an accessibility audit

## Prerequisites

Before starting your audit, you should ensure the following statements are true:

- A relevant GitHub issue has been created for this audit, and it is attached to the [WCAG 2.1 auditing tracking issue](https://github.com/sourcegraph/sourcegraph/issues/31475).
- The user journey you are about to audit feels appropriately scoped.
  - Example: Instead of auditing the entirety of code insights, the audit should be focused on a specific part, such as creating a new code insight.
- You have read and understood [How to use a screen reader](how-to-screen-reader.md).

<i>If you run into any problems, please contact the Frontend Platform team.</i>

## Auditing a user journey

1. Navigate through the journey using **only** the keyboard.
    - Ensure that you are able to access and trigger all important **actions**.
    - Ensure that the current focus position is always **clear** and **visible**.
    - **Note:** Use [this cheatsheet](https://webaim.org/techniques/keyboard/#testing) to help you navigate with a keyboard—it isn't always obvious!
2. Enable a screen reader. Navigate through the user journey **without** looking at your screen.
    - Would a user be able to understand the **content** of the journey?
    - Would a user be able to correctly and predictably perform each important **action**?
    - **Note:** Use the cheatsheet in [How to use a screen reader](how-to-screen-reader.md) to help you navigate.
3. Navigate through the user journey using a viewport that has a **width of 320px**.
    - Would a user be able to sufficiently read all required content in the journey?
    - Would a user be able to correctly and predictably perform each important **action**?
      - Keep in mind that it is typically harder to select small buttons and icons using a touch device.
    - **Note:** You don't need to use a physical mobile device to test this. Most browsers support emulating a mobile viewport—just ensure it is set to **320px**.
      - [Chrome documentation](https://developer.chrome.com/docs/devtools/device-mode/#viewport)
      - [Firefox documentation](https://developer.mozilla.org/en-US/docs/Tools/Responsive_Design_Mode)
    - **Note:** This does not imply full mobile device support (i.e. Touch navigation). [Sourcegraph does not target mobile devices](https://handbook.sourcegraph.com/departments/engineering/#launch).
    - This is to support proper reflow when the browser is zoomed in. See the [WCAG 1.4.10 Reflow criterion](https://www.w3.org/WAI/WCAG21/Understanding/reflow.html) for more information.
4. Work through relevant sections from the [detailed checklist](detailed-checklist.md) and ensure that there are no issues.

## Raising a bug

If the bug is very small, and you are confident you can quickly fix it—then go ahead and make a PR! If you aren't able to immediately address the bug, then you should create a new GitHub issue using the following steps:

1. Open [this GitHub Issue template](https://github.com/sourcegraph/sourcegraph/issues/new?assignees=&labels=accessibility%2Cwcag%2F2.1%2Cwcag%2F2.1%2Ffixing%2Cestimate%2F3d&template=accessibility_issue.yaml&title=%5BAccessibility%5D%3A+)
    - Provide as much detail as possible about the bug you found, and the behavior you expected to happen.
    - Make sure that anyone reading the issue would be able to reproduce the behavior. Screenshots, URLs or videos are helpful!
2. The issue will be automatically added to the [WCAG 2.1 Tracking Issue](https://github.com/sourcegraph/sourcegraph/issues/31476) and the [Accessibility project](https://github.com/orgs/sourcegraph/projects/238) on GitHub.
3. If your team has capacity to address the issue, then please assign yourself to it.
    - The Frontend Platform team will triage any unassigned issues and either fix them or assign them to one of our external contractors.

**Note:** If you created an issue without using the issue template above, please add the following GitHub labels to your issue to ensure we can still track it: `accessibility`, `wcag/2.1`, `wcag/2.1/fixing`.

<i>If you run into any problems, please contact the Frontend Platform team.</i>
