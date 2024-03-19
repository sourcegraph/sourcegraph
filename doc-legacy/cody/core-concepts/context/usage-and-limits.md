# Usage and Limits

<p class="subtitle">Learn about all the usage policies and limits that apply to context.</p>

Cody's usage and limits policies help optimize its performance and ensure cost-effectiveness. This section provides insights into the context window size, token limits for chat, commands, and completions, and administrators' control over these settings.

## Cody context window size

Cody's context window represents the amount of contextual information provided to assist in generating code or responses. Currently, the context window size for both chat and commands is set to **7000 tokens per interaction**.

While Cody aims to provide maximum context for each prompt, there are limits to ensure efficiency. Cody shares all relevant context for chat but limits it to **12 code results** and **3 text results** to maintain performance. Each result comprises approximately **250 tokens**.

## Manage context window size

Site administrators can update the maximum context window size to meet their specific requirements. While using fewer tokens is a cost-saving solution, it can also cause errors. For example, using the `/edit` command with few tokens might get you errors like `You've selected too much code`.

Using more tokens usually produces high-quality responses. It's recommended not to modify the token limit. However, if needed, you can set the value to a limit that does not comprise quality and generates errors.
