# Extension Source Code

This folder contains the source code for the extension.

## Contexts

As detailed in our CONTRIBUTING.md file, this extension runs code in the following execution contexts:

1. Core App

- The main app where all the commands, webviews, file system, and all other components are put together and registered
  as a single extension
  - Runs code with Node.js on Desktop clients (for example, VS Code on Desktop) as a regular extension, or Web Worker
    on Web clients (for example, github.dev on browser) as a web extension
  - [Web extensions](https://code.visualstudio.com/api/extension-guides/web-extensions) run in the web extension host
    in a Browser [Web Worker](https://developer.mozilla.org/en-US/docs/Web/API/Worker) environment, and do not have
    access to Node.js globals and libraries at runtime

2. Sidebars

- This is the UI for all the sidebars used in the core app
  - Uses the [VS Code Webview API](https://code.visualstudio.com/api/extension-guides/webview) to render the webview
    views in search sidebar, search history sidebar, auth sidebar, and help and feedback sidebar

3. Search Panel

- This is the UI for the extension search homepage and to display search results
  - Uses the [VS Code Webview API](https://code.visualstudio.com/api/extension-guides/webview) to render the webview
    panel for the Sourcegraph search homepage as a distinct editor tab

4. Extension Host

- It processes the code-intel data
  - Bring Sourcegraph code-intel to IDEs via
    the [VS Code langauges API](https://code.visualstudio.com/api/language-extensions/programmatic-language-features)
  - Runs in [Web Workers](https://developer.mozilla.org/en-US/docs/Web/API/Web_Workers_API)

5. Custom File System

- This is a custom file system implemented to display file structure of selected repositories from a search result
  - Displays file-tree for the selected repositories when opening
    a [virtual document](https://code.visualstudio.com/api/extension-guides/virtual-documents) using the following VS
    Code APIs:
  - [File System](https://code.visualstudio.com/api/references/vscode-api#FileSystemProvider)
  - [Tree View](https://code.visualstudio.com/api/extension-guides/tree-view)
  - [Virtual Document](https://code.visualstudio.com/api/extension-guides/virtual-documents)
    - A virtual document is a document file located remotely on cloud and not on the local disk
  - [Virtual Workspaces](https://code.visualstudio.com/api/extension-guides/virtual-workspaces)
  - [Text Document Content Provider](https://code.visualstudio.com/api/extension-guides/virtual-documents#textdocumentcontentprovider)

## List of Files and Folders

See the File Structure section in our CONTRIBUTING.md file to learn more about the folders and files in this directory.
Hello World
