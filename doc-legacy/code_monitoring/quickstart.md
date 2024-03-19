# Quickstart for code monitoring

## Introduction

In this tutorial, we will create a new code monitor that monitors new appearances of the word "TODO" in your codebase.

## Creating a code monitor

Prerequisite: Ensure [email notifications](https://docs.sourcegraph.com/admin/observability/alerting#email) are configured in site configuration.  

1. On your Sourcegraph instance, click the **Code monitoring** menu item at the top right of your page. Alternatively, go to `https://sourcegraph.example.com/code-monitoring` (where sourcegraph.example.com represents your unique Sourcegraph url).
1. Click the **Create new code monitor** button at the top right of the page.
1. Fill out the **Name** input with: "TODOs".
1. Under the **Trigger** section, click **When there are new search results**. 
1. In the **Search query** input, enter the following search query:  
`TODO type:diff patternType:literal`.  
(Note that `type:` and `patternType:` are required as part of the search query for code monitoring.)
1. You can click **Preview results** to see all previous additions or removals of TODO to your codebase.
1. Back in the code monitor form, click **Continue**.
1. Click **Send email notifications** under the **Actions** section.
1. Click **Done**.
1. Click **Create code monitor**.

You should now be on `https://sourcegraph.example.com/code-monitoring`, and be able to see the TODO code monitor on the page.

## Sending a test email notification

If you want to preview the email notification alerting you of a new result with TODO, follow these steps:

1. In the **Send email notifications** action, click "Send test email".
1. Within a few minutes, you should receive a test email from Sourcegraph with a preview of the email notification.

If you want to test receiving an email with a real new result, follow these steps:

1. In any repository that's on your Sourcegraph instance (for purposes of this tutorial, we recommend a dummy or test repo that's not used), add `TODO` to any file.
1. Commit the change, and push it to your code host. 
1. Within a few minutes, you should see an email from Sourcegraph with a link to the new result you just pushed.
