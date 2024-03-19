# Run a local Sourcegraph instance behind ngrok

Sometimes it's useful to have the Sourcegraph instance you're running on your
local machine to be reachable over the internet. If you're testing webhooks, for
example, where a code host needs to be able to send requests to your instances.

One way to do that is to use [ngrok](https://ngrok.io/), a reverse proxy that
allows you to expose your instance to the internet.

1. Install `ngrok`: `brew install ngrok`
1. Authenticate the `ngrok` if this is your first time running it (the token can be obtained from [Ngrok dashboard](https://dashboard.ngrok.com/get-started/setup)): `ngrok config add-authtoken <Your token>`
1. Start your Sourcegraph instance: `sg start`
1. Start `ngrok` and point it at Sourcegraph: `ngrok http --host-header=rewrite 3080`
1. Copy the `Forwarding` URL `ngrok` displays. e.g.: `https://630b-87-170-68-206.eu.ngrok.io`
1. Edit your site-config (i.e. `../dev-private/enterprise/dev/site-config.json`) and update the `"externalURL"` to point to your ngrok: `"externalURL": "https://630b-87-170-68-206.eu.ngrok.io"`
1. Open the ngrok URL in browser to make sure you see your instance
1. Done
