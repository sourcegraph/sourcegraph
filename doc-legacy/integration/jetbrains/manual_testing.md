# JetBrains plugin manual test cases

## Overview

All these test scripts are written in a way so that humans can perform it, but also an E2E testing tool can run it eventually.

Each of these test cases should be run either right after starting the IDE and opening a project, _or_ after closing and reopening a project, to make sure we have a clean state.

### Test case format anatomy

* The numbered items are the steps to perform in order.
* The indented bullets are assertions.

### Performing a test manually

If a red error notification appears at the bottom right of the IDE, that means we have a problem.

Same if any of the assertions fail.

If there is a problem, we should either investigate right away (typically, if a developer does the manual testing), or create an issue with the error message / specifying which test case failed, and any additional helpful context.

### Operating systems

We test on macOS by default, but we should also test on Windows and Linux more often.
We have three test instances to test the plugin on a Linux server, an Ubuntu Desktop machine, and on Windows Server. They are all on GCP, [here](https://console.cloud.google.com/compute/instances?cloudshell=true&project=david-veszelovszki-temp-env).
- For the Windows instance, follow [this guide](https://www.snel.com/support/how-to-connect-to-your-server-via-rdp-on-macos/) to connect, then use the login/password you find in 1Password by the name "GCP: JetBrains plugin testing Windows". 

## Test cases

### Website-related features

#### Preparation

**Perforce** testing is slightly tricky because we don’t currently have a lot of Perforce code hosts added to any of our Sourcegraph instances. Try this:

* Use [https://cse-k8s.sgdev.org/](https://cse-k8s.sgdev.org/) as your Sourcegraph URL
* Generate an access token for yourself for CSE-K8S (admin user in 1Password), set that
* Set up a Perforce client
* Set `perforce-tests.sgdev.org:1666` for the port. User is “admin”, password is in 1Password.
* Make sure your connection work, and _actually connect_ in P4V (connection didn’t work without this step  in IntelliJ for me)
* Create a workspace for yourself (delete an old one if you bump into the 20-workspace limit), and include the `test-large-depot` depot
* In the JetBrains plugin settings, set the replacement string to `perforce-tests.sgdev.org:,perforce.beatrix.com:app/,.perforce,/patch/core.perforce`.
* At this point, your files should open properly in your browser.

#### Search Selection and Open/Copy features

1. Open a project with a file that’s under Git version control
2. Right click on editor | Sourcegraph | Search Selection on Sourcegraph Web
    * Make sure it searches the current selection in all repos
3. Right click on editor | Sourcegraph | Search Selection in Repository on Sourcegraph Web
    * Make sure it searches the current selection in the current repo
4. Right click on editor | Sourcegraph | Open Selection in Sourcegraph Web
    * Make sure it opens the right file on the web
5. Copy Sourcegraph File Link
    * Make sure it copies the right URL (should be good if the previous point worked because they share the logic.)
6. Repeat it with a Perforce repo.

#### “Open Revision Diff in Sourcegraph Web”

1. Open a project that has files under Git version control
2. Open Version Control (⌘9) | Log
3. Right click on a commit and choose Open Revision Diff in Sourcegraph Web
4. Make sure the right page opens on the Sourcegraph instance set up in the plugin settings
5. (Would be cool to also test this each time with a project that has files from multiple repositories.)
6. Repeat the process with a project that has files under Perforce version control.

### Find with Sourcegraph

#### Popup opens, loads, and closes

Why: To make sure the popup is discoverable and generally works

1. Press the shortcut for the popup
    * The popup should become active
    * The popup should be in the “loading” state
2. Wait until the “loading” state is gone
    * It should not take more than ~five seconds
3. Close the popup with ESC
    * The popup should become hidden
4. Open “Find Action” (⌘⇧A) and search for “find with sourcegraph” and select the first result
    * The popup should become active
    * The popup should not be in the “loading” state
5. Click the header of the IDE main window
    * The popup should become hidden
6. In the main menu, choose Edit | Find | Find on Sourcegraph…
    * The popup should become active

#### Browser doesn’t disappear after a few opens

Why: After opening and closing the "Find on Sourcegraph" popup four-five times on Mac, the browser disappears if _circumstances are not right_. This bug should be fixed, but it may reappear if we mess something up.

1. Open popup with the shortcut
    * The popup should become active
2. Wait until the “loading” state is gone
3. Close the popup with ESC
4. Open popup with the shortcut
    * The popup should become active, and the browser should be visible
5. Repeat 3–4 at least five times.

#### Search results are displayed and navigation works

Why: To make sure that the most basic functionality of the popup works

1. Open the popup with the shortcut
2. Wait for the browser content to load
3. Type “repo:^github\.com/sourcegraph/sourcegraph file:index.ts”
    * It should type into the input box
    * The content should be exactly the same as what was typed
4. Press Enter
5. Wait for the search results and preview to load
    * The result list and the preview box should populate
6. Press `↓` key
7. Wait for the search results and preview to load
    * The preview should be different from the previous one
8. Press `⌥Enter`
    * The popup should close
    * A new editor tab should open
    * The content of the new tab should be the same as the last preview
9. Open the popup with the shortcut
10. Press ⌘A, type “repo:^github\.com/sourcegraph/sourcegraph” and press Enter
11. Wait for the search results to load
    * The result list should update
    * The preview state should be “No preview available”
12. Press ⌥Enter
    * The popup should close
    * A browser should open (if automated: an “open browser” trigger should be sent)

#### Server connection success/failure is recognized

This test needs a valid access token that can be generated at https://sourcegraph.com/users/{username}/settings/tokens

* Open Settings with `⌘`,
* Go to Tools | Sourcegraph
* Set the URL to “[https://sourcegraph.com](https://sourcegraph.com)”
* Set the access token to a valid one
    * Press Enter to save settings
* Open the popup with the shortcut
* Wait for the browser content to load.
    * The search box should be visible
* Close the popup with ESC
* Open Settings with ⌘,
* Type “TEST” after the end of the valid access token.
* Press Enter to save settings
* Open the popup with the shortcut
    * The search box should not be visible, an error message should appear
    * The preview should be saying “No preview available”
* Close the popup with ESC
* Open Settings with ⌘,
* Remove “TEST” to again have the valid access token.
* Press Enter to save settings
* Open the popup with the shortcut
    * The search box should be visible again.
