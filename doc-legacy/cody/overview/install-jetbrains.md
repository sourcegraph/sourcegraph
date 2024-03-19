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

# Install Cody for JetBrains

<p class="subtitle">Learn how to use Cody and its features with the JetBrains IntelliJ editor.</p>

<aside class="beta">
<p>
<span style="margin-right:0.25rem;" class="badge badge-beta">Beta</span> Cody support for JetBrains is in the Beta stage.
<br />
For any feedback, you can <a href="https://sourcegraph.com/contact">contact us</a> directly, file an <a href="https://github.com/sourcegraph/cody/issues">issue</a>, join our <a href="https://discord.com/servers/sourcegraph-969688426372825169">Discord</a>, or <a href="https://twitter.com/sourcegraphcody">tweet</a>.
</p>
</aside>

The Cody extension by Sourcegraph enhances your coding experience in your IDE by providing intelligent code suggestions, context-aware completions, and advanced code analysis. This guide will walk you through the steps to install and set up Cody within your JetBrains environment.

<ul class="limg">
  <li>
    <a class="card text-left" target="_blank" href="https://plugins.jetbrains.com/plugin/9682-cody-ai-by-sourcegraph">
      <h3><img alt="JetBrains" src="https://storage.googleapis.com/sourcegraph-assets/docs/images/cody/jb_beam.svg" />JetBrains Extension (Beta)</h3>
      <p>Install Cody's free extension for JetBrains.</p>
    </a>
  </li>
  </ul>

## Prerequisites

- You have the latest version of <a href="https://www.jetbrains.com/idea/" target="_blank">JetBrains IDEs</a> installed
- You have a Free or Pro account through Sourcegraph.com or a Sourcegraph Enterprise account
- Cody is compatible with the following JetBrains IDEs:
  - [Android Studio](https://developer.android.com/studio)
  - [AppCode](https://www.jetbrains.com/objc/)
  - [CLion](https://www.jetbrains.com/clion/)
  - [DataGrip](https://www.jetbrains.com/datagrip/)
  - [GoLand](https://www.jetbrains.com/go/)
  - [IntelliJ IDEA](https://www.jetbrains.com/idea/) (Community and Ultimate editions)
  - [PhpStorm](https://www.jetbrains.com/phpstorm/)
  - [PyCharm](https://www.jetbrains.com/pycharm/) (Community and Professional editions)
  - [Rider](https://www.jetbrains.com/rider/)
  - [RubyMine](https://www.jetbrains.com/ruby/)
  - [WebStorm](https://www.jetbrains.com/webstorm/)

## Install the JetBrains IntelliJ Cody extension

Follow these steps to install the Cody plugin:

- Open JetBrains IntelliJ editor on your local machine
- Open **Settings** (Mac: `⌘+,` Windows: `Ctrl+Alt+S`) and select **Plugins**
- Type and search **Sourcegraph Cody + Code Search** extension and click **Install**

![cody-for-intellij](https://storage.googleapis.com/sourcegraph-assets/Docs/Media/cody-for-intellij.png)

Alternatively, you can [Download and install the extension from the JetBrains marketplace](https://plugins.jetbrains.com/plugin/9682-sourcegraph).

## Connect the extension to Sourcegraph

After a successful installation, Cody's icon appears in the sidebar. Clicking it prompts you to start with codehosts like GitHub, GitLab, and your Google login. This allows Cody to access your Sourcegraph.com account.

Alternatively, you can also click the **Sign in with an Enterprise Instance** to connect to your enterprise instance.

![cody-for-intellij-login](https://storage.googleapis.com/sourcegraph-assets/Docs/Media/sign-in-cody-jb.png)

### Context Selection

Cody automatically understands your codebase context based on the project opened in your workspace. However, at any point, you can override the **automatic codebase detection** by clicking on the repo name below the Cody chat and then adding the Git URL. This will allow Cody to start using the codebase context you’ve selected.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/override-context.mp4" type="video/mp4">
</video>

### For Sourcegraph Enterprise users

To connect the extension with your Enterprise instance,

- Log in to your Sourcegraph instance
- Go to **Settings > Access tokens** (`https://<your-instance>.sourcegraph.com/users/<your-username>/settings/tokens`)
- From here, click **Generate new token**
- Next, go back to **Cody Settings...** in the IntelliJ editor
- Click the plus icon (+) under **Authentication** and then select **Log In with Token to Sourcegraph Enterprise**
- Finally, paste your newly created access token and instance URL in the respective fields and click **Add Account** to complete the process

### For Sourcegraph.com users

Alternatively, you can click `Log In to Sourcegraph` to authenticate the IntelliJ extension with your Sourcegraph.com account.

## Verifying the installation

Once Cody is connected successfully to your Sourcegraph instance, click the Cody icon from the sidebar again, and a panel will open. To verify that the Cody extension has been successfully installed and is working as expected, let's create an autocomplete suggestion.

Cody provides intelligent code suggestions and context-aware autocompletions for numerous programming languages like JavaScript, Python, TypeScript, Go, etc.

- Create a new file in IntelliJ, for example, `code.js`
- Next, type the following algorithm function to sort an array of numbers

```js
function bubbleSort(array){
```

- As you start typing, Cody will automatically provide suggestions and context-aware completions based on your coding patterns and the code context
- These autocomplete suggestions appear as grayed text. To accept the suggestion, press the `Tab` key

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/jb-first-suggestion.mp4">
</video>

## Autocomplete

Autocomplete suggestions appear as inlay suggestions and are enabled by default in your JetBrains IntelliJ editor. With this setting, there is a list of programming languages supported and enabled by default.

To manually configure the Autocomplete feature,

- Go to the **Cody Settings...** from the Cody icon in the sidebar
- Next, click the **Sourcegraph & Cody** dropdown and select **Cody**
- The **Autocomplete** settings will appear with the list of **Enabled Languages**

In addition, Autocomplete suggestions use the same color as inline parameter hints according to your configured editor theme. However, you can optionally enable the **Custom color for completions** checkbox to customize the color of your choice.

## Commands

Cody with JetBrains offers quick, ready-to-use Commands for common actions to write, describe, fix, and smell code. These allow you to run predefined actions with smart context-fetching anywhere in the editor, like:

- Explain Selected Code
- Generate Docstring
- Generate Unit Test
- Improve Variable Names
- Smell Code

Let's understand how the **Generate Docstring** command generates code documentation for the sorting function.

<video width="1920" height="1080" loop playsinline controls style="width: 100%; height: auto; max-width: 50rem">
  <source src="https://storage.googleapis.com/sourcegraph-assets/Docs/Media/jb-generate-docstring.mp4" type="video/mp4">
</video>

## Updating the extension

JetBrains IntelliJ will typically notify you when updates are available for installed extensions. Follow the prompts to update the Cody extension to the latest version.

## More resources

For more information on what to do next, we recommend the following resources:

<div class="cards">
  <a class="card text-left" href="./../quickstart"><b>Cody Quickstart</b><p>This guide recommends how to use Cody once you have installed the extension in your VS Code editor.</p></a>
  <a class="card text-left" href="./../use-cases/generate-unit-tests"><b>Cody Use Cases</b><p>Explore some of the most common use cases of Cody that helps you with your development workflow.</p></a>
</div>
