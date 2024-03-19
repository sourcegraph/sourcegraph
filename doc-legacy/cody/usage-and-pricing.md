# Cody Pricing

<p class="subtitle">Learn about the different plans available for Cody.</p>

Cody provides three subscription plans: **Free**, **Pro**, and **Enterprise**. Each plan is aimed to cater to a diverse range of users, from individual projects to large-scale enterprises. Cody Free includes basic features, while the Pro and Enterprise plans offer additional advanced features and resources to meet varying user requirements.

## Free

The free plan is designed for individuals to get started with Cody. It comes with a set of features to enhance your coding experience. It includes **500 autocompletion suggestions** per month, covering both whole and multi-line suggestions. You also get **20 chats/commands** per month with access to creating Custom Commands.

The free plan provides access to local context with keyword search and embeddings on open-source code via Sourcegraph.com. You get **200 MB** of embeddings over your lifetime and can manage these from the user settings in VS Code, with JetBrains IDE support coming soon. If you embed 150 MB of code, you can only embed another 50 MB of code. If you delete that 150 MB of code embedding, the original 200 MB of code embeddings will be available again.

The free plan ensures local context utilization and allows you to seamlessly integrate Cody into your preferred client, whether it's VSCode, JetBrains, or Neovim. Finally, you'll have default support with Anthropic and StarCoder as the officially supported LLMs.

### Billing cylce

There is no billing cycle for Cody Free, as it is free to use. If you complete your monthly autocompletions or chat/commands limit, you'll be prompted to upgrade to the Pro plan. Otherwise, you'll have to wait approximately 30 days for the limits to reset.

The reset date is based on your sign-up date. For example, if you sign up on December 15th, your limit will reset on January 15th.

### Upgrading from Free to Pro

Until February 2024, you can upgrade to Cody Pro for free. If you want to continue using Cody Pro after February 2024, you get that at a **monthly price of $9 per user per month**.

>NOTE: We'll be sharing payment details for the Pro plan in the coming weeks.

## Pro

Cody Pro, designed for individuals or small teams at **$9 per user per month**, offers an enhanced coding experience beyond the free plan. It provides unlimited autocompletion suggestions, allowing users to streamline their coding process without restrictions. Chat and commands executions are also unlimited, allowing users to create custom workflows.

The plan includes the ability to create an unlimited number of Custom Commands for personalized workflows. In addition to using the local context of your code to improve responses, you can embed up to **1 GB** of your private code for even better Cody responses that reflect a deep understanding of your code in VS Code, with JetBrains IDE support coming soon.

Support for Cody Pro is available through Discord, ensuring prompt assistance and guidance. Finally, you'll have default support with Anthropic and StarCoder as the officially supported LLMs. Moreover, Pro accounts using VS Code IDE will get an LLM selection for chat only. These LLMs are Claude Instant 1.2, Claude 2, ChatGPT 3.5 Turbo, ChatGPT 4 Turbo Preview, and Mixtral.

> NOTE: There will be high daily limits to catch bad actors and prevent abuse, but under most normal usage, Pro users won't experience these limits.

### Downgrading from Pro to Free

To revert back to Cody Free from Pro:

- Go to your Sourcegraph dashboard **Cody > Manage**
- Next, **Manage subscription** that takes you to **Cody > Subscription**
- Clicks **Cancel** on the Pro tier to cancel your Pro subscription
- This automatically downgrades you to Cody Free

### Upgrading from Pro to Enterprise

To upgrade from Cody Pro to Cody Enterprise, you should [Contact Sales](https://sourcegraph.com/contact/request-info) and connect with one of our account teams. They will help you set up your account and start with Cody Enterprise.

## Enterprise

Cody Enterprise is designed for enterprises prioritizing security and administrative controls. We offer either seat-based or token based pricing models, depending on what makes most sense for your organization. You get additional capabilities like BYOLLM (Bring Your Own LLM), supporting Single-Tenant and Self Hosted setups for flexible coding environments.

Security features include SAML/SSO for enhanced authentication and guardrails to enforce coding standards. Cody Enterprise supports advanced Code Graph context and multi-code host context for a deeper understanding of codebases, especially in complex projects. With 24/5 enhanced support, Cody Enterprise ensures timely assistance.

## Plans Comparison

The following table shows a high-level comparison of the three plans available on Cody.

| **Features**                              | **Free**                            | **Pro**                              | **Enterprise**                                 |
|---------------------------------------|---------------------------------|----------------------------------|--------------------------------------------|
| **Autocompletion suggestions**                   | 500 per month (whole + multi-line)| Unlimited                       | Unlimited                                  |
| **Chat/Command Executions**           | 20 per month                     | Unlimited                       | Unlimited                                  |
| **Custom Commands**              | Supported                             | Supported                       | Supported                                  |
| **Embeddings (public code)**              | Supported                             | Supported                       | Supported                                  |
| **Embeddings (private code)**              | 200 MB                             | 1 GB                       | Greater than 1 GB and scalable                                  |
| **Keyword Context (local code)**                     | Supported                             | Supported                        | Supported                                  |
| **Developer Limitations**             | 1 developer                     | Up to 50 devs                    | Scalable, consumption-based pricing      |
| **LLM Support**                       | Anthropic + Starcoder for Chat, Commands, and Autocomplete | Choice of LLMs for Chat (VS Code only), Anthropic + Starcoder for Commands and Autocomplete  | Bring Your Own LLM Key (experimental)                |
| **Code Editor Support**                       | VS Code, JetBrains IDEs, and Neovim | VS Code, JetBrains IDEs, and Neovim  | VS Code, JetBrains IDEs, and Neovim                |
| **Single-Tenant and Self Hosted**     | N/A                             | N/A                              | Yes                                        |
| **SAML/SSO**                          | N/A                             | N/A                              | Yes                                        |
| **Guardrails**                        | N/A                             | N/A                              | Yes                                        |
| **Advanced Code Graph Context**                 | N/A                             | N/A                              | Included                                  |
| **Multi-Code Host Context**           | N/A                             | N/A                              | Included                                  |
| **Discord Support**                   | No                              | Yes                              | Yes                                        |
| **24/5 Enhanced Support**             | N/A                             | N/A                              | Yes                                        |
