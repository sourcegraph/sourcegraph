# Setting up Slack notifications

<aside class="note">
<p>
<span class="badge badge-beta">Beta</span> This feature is currently in beta and may change in the future.
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

Slack notifications are supported via webhooks. Webhooks are special URLs that Sourcegraph's Code Monitoring 
can call in order to send a message to a Slack channel when there are new search results for a query. 
In order to use Slack notifications, you must first create a Slack application in your organization's Slack workspace, 
add a webhook for a Slack channel within that application, and then configure a code monitor in Sourcegraph to
use that webhook's URL.

## Prerequisites

- You must not have have the setting `experimentalFeatures.codeMonitoringWebHooks` disabled in your user, org, or global settings.
- You must have permission to create apps inside of your organization's Slack workspace

## Creating a Slack webhook

1. Navigate to https://api.slack.com/apps and sign in to your Slack account if necessary.
1. Click on the "Create an app" button.
1. Create your app "From scratch".
1. Give your app a name and select the workplace you want notifications sent to.
 <video src="https://storage.googleapis.com/sourcegraph-assets/search/code-monitoring/slack-tutorial/1-create-app.mp4" controls />
1. Once your app is created, click on the "Incoming Webhooks" in the sidebar, under "Features".
1. Click the toggle button to activate incoming webhooks.
1. Scroll to the bottom of the page and click on "Add New Webhook to Workspace".
1. Select the channel you want notifications sent to, then click on the "Allow" button.
1. Your webhook URL is now created! Click the copy button to copy it to your clipboard.
 <video src="https://storage.googleapis.com/sourcegraph-assets/search/code-monitoring/slack-tutorial/2-create-webhook.mp4" controls />

## Configuring a code monitor to send Slack notifications

1. In Sourcegraph, click on the "Code Monitoring" nav item at the top of the page.
1. Create a new code monitor or edit an existing monitor by clicking on the "Edit" button next to it.
1. Go through the standard configuration steps for a code monitor and select action "Send Slack message to channel".
1. Paste your webhook URL into the "Webhook URL" field.
1. Click on the "Continue" button, and then the "Save" button.
 <video src="https://storage.googleapis.com/sourcegraph-assets/search/code-monitoring/slack-tutorial/3-create-monitor.mp4" controls>
