# Autocomplete

<p class="subtitle">Learn how Cody helps you get contextually-aware autocompletions for your codebase.</p>

Cody provides intelligent **autocomplete** suggestions as you type using context from your code, such as your open files and file history. Cody autocompletes single lines or whole functions in any programming language, configuration file, or documentation. It’s powered by the latest instant LLM models for accuracy and performance.

Autocomplete supports any programming language because it uses LLMs trained on broad data. It works exceptionally well for Python, Go, JavaScript, and TypeScript.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/cody-in-action.mp4" type="video/mp4">
</video>

## Prerequisites

To view how Cody provides code completions , you'll need to have the following:

- A Free or Pro account through Sourcegraph.com or a Sourcegraph Enterprise account
- A supported editor extension (VS Code, JetBrains, and Neovim) installed

## Working with code autocomplete

The autocomplete feature is enabled by default on all IDE extensions, i.e., VS Code, JetBrains, and Neovim. Generally, there's a checkbox in the extension settings that confirms whether the autocomplete feature is enabled or not. In addition, some autocomplete settings are optionally and explicitly supported by some IDEs. For example, JetBrains IDEs have custom settings that allow you to customize colors and styles of the autocomplete suggestions.

You can start typing and Cody will automatically provide suggestions and context-aware completions based on your coding patterns and the code context. These autocomplete suggestions appear as grayed text. To accept the suggestion, press the `Enter` or `Tab` key.

## Configure autocomplete on an Enterprise Sourcegraph instance

By default, a fully configured Sourcegraph instance picks a default LLM to generate code autocomplete. Custom models can be used for Cody autocomplete based on your specific requirements. To do so:

- Go to the **Site admin** of your Sourcegraph instance
- Navigate to **Configuration > Site configuration**
- Here, edit the `completionModel` option inside the `completions`
- Click the **Save** button to save the changes

> NOTE: Cody autocomplete works only with Anthropic's Claude Instant model. Support for other models will be coming later.

> NOTE: Self-hosted customers must update to version 5.0.4 or more to use autocomplete.

Before configuring the autocomplete feature, it's recommended to read more about [Enabling Cody on Sourcegraph Enterprise](../overview/enable-cody-enterprise.md) guide.

Cody Autocomplete goes beyond basic suggestions. It understands your code context, offering tailored recommendations based on your current project, language, and coding patterns. Let's view a quick demo using the VS Code extension.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/contexual-autocpmplete.mp4" type="video/mp4">
</video>

Here, Cody provides suggestions based on your current project, language, and coding patterns. Initially, the `code.js` file is empty. Start writing a function for `bubbleSort`. As you type, Cody suggests the function name and the function parameters.

Cody automatically suggests the next few code snippets for every new line based on your current context, i.e., functions for `insertionSort` and `selectionSort`.

## Access autocomplete logs

VS Code logs can be accessed via the **Outputs** view. To access autocomplete logs, you need to enable Cody logs in verbose mode. To do so:

- Go to the Cody Extension Settings and enable: `Cody › Debug: Enable` and `Cody › Debug: Verbose`
- Restart or reload your VS Code editor
- You can now see the logs in the Outputs view
- Open the view via the menu bar: `View > Output`
- Select **Cody by Sourcegraph** from the dropdown list

![View Cody's autocomplete logs from the Output View in VS Code](https://storage.googleapis.com/sourcegraph-assets/Docs/autocomplete-logs.png)
