# Developing the EmbeddedWebApp

- To test your local changes on 3rd party sites, you will have to proxy your dev environment using [ngrok](https://ngrok.com/)
- Run ngrok using this command: `./ngrok http 3080 --host-header=rewrite`
- Copy the Forwarding https:// address and use it to replace the `externalURL` property in your `site-config.json`
  - If you copy the http:// address, then embedding will fail for non-Cloud local instances
- Start (or restart) your local dev environment
- Navigate to a 3rd party site (e.g., codepen.io), create a new page, and embed an iframe using the ngrok URL
  - Example iframe tag: `<iframe src="https://your-dev-env.ngrok.io/embed/notebooks/123" frameborder="0" sandbox="allow-scripts allow-same-origin allow-popups"></iframe>`
Hello World
