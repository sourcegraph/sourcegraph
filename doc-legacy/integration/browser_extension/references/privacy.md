# Privacy

Sourcegraph integrations will only connect to Sourcegraph.com as required to provide code navigation or other functionality on public code. As a result, no private code, private repository names, usernames, or any other specific data is sent to Sourcegraph.com.

If connected to a **private, self-hosted Sourcegraph instance**, Sourcegraph integrations never send any logs, pings, usage statistics, or telemetry to Sourcegraph.com. They will send notifications of usage to that private Sourcegraph instance only. This allows the site admins to see usage statistics.

If connected to the **public Sourcegraph.com instance**, Sourcegraph integrations will send notifications of usage on public repositories to Sourcegraph.com.  

The browser extension also does not store sensitive data locally. The information stored is restricted to:

- AnonymousUid
- Feature flags
- Client settings
  - Enable/disable status
  - Sourcegraph URL
