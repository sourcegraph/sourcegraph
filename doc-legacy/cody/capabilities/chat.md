# Chat

<p class="subtitle">Use Cody's chat to get contextually-aware answers to your questions.</p>

Cody **chat** allows you to ask coding-related questions about your entire codebase or a specific code snippet. You can do it from the **Chat** panel of the supported editor extensions (VS Code, JetBrains, and Neovim) or in the web app.

Key functionalities in the VS Code extension include support for multiple simultaneous chats, enhanced chat context configurability through `@` commands, detailed visibility into the code that Cody read before providing a response, and more.

You can learn more about the IDE support for these functionalities in the [feature parity reference](./../feature-reference.md#chat).

## Prerequisites

To use Cody's chat, you'll need to have the following:

- A Free or Pro account through Sourcegraph.com or a Sourcegraph Enterprise account
- A supported editor extension (VS Code, JetBrains, and Neovim) installed

## How does chat work?

Cody can use several methods (including keyword search and optional embeddings context) to ask relevant questions. For VS Code extension users, Cody also uses context from the files to provide an informed response based on your codebase. Cody also tells you which code files it reads to generate its responses.

Cody can assist you with various use cases such as:

- Generating an API call: Cody can analyze your API schema to provide context for the code it generates
- Locating a specific component in your codebase: Cody can identify and describe the files where a particular component is defined
- Handling questions that involve multiple files, like understanding data population in a React app: Cody can locate React component definitions, helping you understand how data is passed and where it originates

## Ask Cody your first question

Let's use Cody VS Code extension's chat interface to answer your first question.

- Click the Cody icon in the sidebar to view the detailed panel
- Next, click the icon for **New Chat** to open a new chat window
- Here, you can directly write your question or type slash `/` to select one of the commands and then press **Enter**

For example, ask Cody "What does this file do?"

Cody will take a few seconds to process your question, providing contextual information about the files it reads and generating the answer.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/ask-cody-question.mp4" type="video/mp4">
</video>

## Ask Cody to write code

The chat feature can also write code for your questions. For example, in VS Code, ask Cody to "write a function that sorts an array in ascending order".

You are provided with code suggestions in the chat window along with the following options for using the code.

- The **Copy Code** icon to your clipboard and paste the code suggestion into your code editor
- Insert the code suggestion at the current cursor location by the **Insert Code at Cursor** icon
- The **Save Code to New File** icon to save the code suggestion to a new file in your project

During the chat, if Cody needs additional context, it can ask you to provide more information with a follow-up question. If your question is beyond the scope of the context, Cody will ask you to provide an alternate question aligned with the context of your codebase.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/cody-write-code.mp4" type="video/mp4">
</video>

### Chat vs Commands

There could be scenarios when Cody's chat might not be able to answer your question. Or the answer lacks the context that you need. In these cases, it's recommended to use Cody **commands**. Cody's responses to commands might be better at times than responses to chats since they've been pre-packaged and prompt-engineered.

> NOTE: Commands are only supported in the VS Code and JetBrains extension.
