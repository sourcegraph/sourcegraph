<style>

  .markdown-body .cards {
  display: flex;
  align-items: stretch;
}

.markdown-body .cards .card {
  flex: 1;
  margin: 0.5em;
  color: var(--text-color);
  border-radius: 4px;
  border: 1px solid var(--sidebar-nav-active-bg);
  padding: 1.5rem;
  padding-top: 1.25rem;
}

.markdown-body .cards .card:hover {
  color: var(--link-color);
}

.markdown-body .cards .card span {
  color: var(--link-color);
  font-weight: bold;
}

.markdown-body .cards {
  display: flex;
  align-items: stretch;
}

.markdown-body .cards .card {
  flex: 1;
  margin: 0.5em;
  color: var(--text-color);
  border-radius: 4px;
  border: 1px solid var(--sidebar-nav-active-bg);
  padding: 1.5rem;
  padding-top: 1.25rem;
}

.markdown-body .cards .card:hover {
  color: var(--link-color);
}

.markdown-body .cards .card span {
  color: var(--link-color);
  font-weight: bold;
}

.limg {
  list-style: none;
  margin: 3rem 0 !important;
  padding: 0 !important;
}
.limg li {
  margin-bottom: 1rem;
  padding: 0 !important;
}

.limg li:last {
  margin-bottom: 0;
}

.limg a {
    display: flex;
    flex-direction: column;
    transition-property: all;
   transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
     transition-duration: 350ms;
     border-radius: 0.75rem;
  padding-top: 1rem;
  padding-bottom: 1rem;

}

.limg a {
  padding-left: 1rem;
  padding-right: 1rem;
  background: rgb(113 220 232 / 19%);
}

.limg p {
  margin: 0rem;
}
.limg a img {
  width: 1rem;
}

.limg h3 {
  display:flex;
  gap: 0.6rem;
  margin-top: 0;
  margin-bottom: .25rem

}

</style>

# Cody for Web

<p class="subtitle">Learn how to use Cody in the web interface with Sourcegraph.com</p>

In addition to the Cody extensions for [VS Code](./install-vscode.md), [JetBrains](./install-jetbrains.md) IDEs, and [Neovim](./install-neovim.md), Cody is also available in the Sourcegraph web app. Community users can use Cody for free by logging into their accounts on Sourcegraph.com, and enterprise users can use Cody within their Sourcegraph instance.

<ul class="limg">
  <li>
    <a class="card text-left" target="_blank" href="https://sourcegraph.com/sign-in?returnTo=/search">
      <h3><img alt="Cody for Web" src="https://sourcegraph.com/.assets/img/sourcegraph-mark.svg" />Cody for Web</h3>
      <p>Use Cody in the Sourcegraph web app.</p>
    </a>
  </li>
  </ul>

## Initial setup

Create a [Sourcegraph.com account](https://sourcegraph.com/sign-up) by logging in through codehosts like GitHub and GitLab or via traditional Google sign-in. This takes you to Sourcegraph’s web interface, where you can chat with and get help from Cody by opening **Cody > Web Chat**.

![cody-web](https://storage.googleapis.com/sourcegraph-assets/Docs/cody-web.png)

Enterprise users can also log in to their Sourcegraph.com Enterprise instance and use Cody in the web interface.

## Using Cody chat

To view how Cody works in the web, open **Cody Web Chat**. Here, you can ask Cody general coding questions or questions about the repositories you care about.

You can view the default **Ask Cody** chat interface on the right sidebar. Once connected, Cody automatically gains context to your connected codebase to help you provide contextually-aware and accurate answers to your questions. There are also example questions that you can use to get started.

When using Cody chat in the web, you can select up to **10 repositories** to use as context for your question. If you’re logged in to Sourcegraph.com, these can be any **10 open source repositories** that Sourcegraph indexes. If you are an enterprise customer logged in to your company’s Sourcegraph instance, these can be **any 10 repositories** (both private or public) indexed by your admin.

You can also chat with Cody in the sidebar of any project you have open. Use Sourcegraph search to find a repo and click the **Ask Cody** button to start.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/search-cody-chat.mp4" type="video/mp4">
</video>

## Run Commands

You can also run predefined, reusable prompts [Cody Commands](./../capabilities.md#commands) on your Code Search results. These help you generate contextually aware answers for a selected code snippet or the entire codebase. You can run the following commands:

- Explain code
- Generate a docstring
- Smell code for bugs
- Generate a unit test
- Improve variable names
- Transpile code to a programming language of your choice

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/cody-commands-sg.mp4" type="video/mp4">
</video>

## More resources

For more information on what to do next, we recommend the following resources:

<div class="cards">
  <a class="card text-left" href="./../quickstart"><b>Cody Quickstart</b><p>This guide recommends how to use Cody once you have installed the extension in your VS Code editor.</p></a>
  <a class="card text-left" href="./../use-cases/generate-unit-tests"><b>Cody Use Cases</b><p>Explore some of the most common use cases of Cody that helps you with your development workflow.</p></a>
</div
