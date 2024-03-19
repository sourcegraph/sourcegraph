# Setting up Webhook notifications

<aside class="note">
<p>
<span class="badge badge-beta">Beta</span> This feature is currently in beta and may change in the future.
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

Webhook notifications provide a way to execute custom responses to a code monitor notification.
They are implemented as a POST request to a URL of your choice. The body of the request is defined
by Sourcegraph, and contains all the information available about the cause of the notification.

## Prerequisites

- You must not have have the setting `experimentalFeatures.codeMonitoringWebHooks` disabled in your user, org, or global settings.
- You must have a service running that can accept the POST request triggered by the webhook notification

## Creating a webhook receiver

A webhook receiver is a service that can accept an HTTP POST request with the contents of the webhook notification.
The receiver must be reachable from the Sourcegraph cluster using the URL that is configured below.

The HTTP POST request sent to the receiver will have a JSON-encoded body with the following fields:

- `monitorDescription`: The description of the monitor as configured in the UI
- `monitorURL`: A link to the monitor configuration page
- `query`: The query that generated `results`
- `results`: The list of results that triggered this notification. Contains the following sub-fields
  - `repository`: The name of the repository the commit belongs to
  - `commit`: The commit hash for the matched commit.
  - `diff`: The matching diff in unified diff format. Only set if the result is a diff match.
  - `matchedDiffRanges`: The character ranges of `diff` that matched `query`. Only set if the result is a diff match.
  - `message`: The matching commit message. Only set if the result is a commit match.
  - `matchedMessageRanges`: The character ranges of `message` that matched `query`. Only set if the result is a commit match.

Example payload:
```json
{
  "monitorDescription": "My test monitor",
  "monitorURL": "https://sourcegraph.com/code-monitoring/Q29kZU1vbml0b3I6NDI=?utm_source=",
  "query": "repo:camdentest -file:id_rsa.pub BEGIN",
  "results": [
    {
      "repository": "github.com/test/test",
      "commit": "7815187511872asbasdfgasd",
      "diff": "file1.go file2.go\n@ -97,5 +97,5 @ func Test() {\n leading context\n+matched added\n-matched removed\n trailing context\n",
      "matchedDiffRanges": [
        [ 66, 73 ],
        [ 91, 98 ]
      ]
    },
    {
      "repository": "github.com/test/test",
      "commit": "7815187511872asbasdfgasd",
      "message": "summary line\n\nsample\ncommit\nmessage\n",
      "matchedMessageRanges": [
        [ 15, 19 ]
      ]
    }
  ]
}
```

## Configuring a code monitor to send Webhook notifications

1. In Sourcegraph, click on the "Code Monitoring" nav item at the top of the page.
1. Create a new code monitor or edit an existing monitor by clicking on the "Edit" button next to it.
1. Go through the standard configuration steps for a code monitor and select action "Call a webhook".
1. Paste your webhook URL into the "Webhook URL" field.
1. Click on the "Continue" button, and then the "Save" button.
