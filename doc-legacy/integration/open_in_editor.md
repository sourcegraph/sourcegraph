# Open in Editor

With a click of a button, you can jump from any file in Sourcegraph to your preferred editor. It only takes a minute to
set up.

<img src="img/editors/appcode.svg" width="32" height="30" alt="AppCode logo" />
<img src="img/editors/clion.svg" width="32" height="30" alt="CLion logo" />
<img src="img/editors/goland.svg" width="32" height="30" alt="GoLand logo" />
<img src="img/editors/idea.svg" width="32" height="30" alt="IntelliJ logo" />
<img src="img/editors/phpstorm.svg" width="32" height="30" alt="PhpStorm logo" />
<img src="img/editors/pycharm.svg" width="32" height="30" alt="PyCharm logo" />
<img src="img/editors/rider.svg" width="32" height="30" alt="Rider logo" />
<img src="img/editors/rubymine.svg" width="32" height="30" alt="RubyMine logo" />
<img src="img/editors/webstorm.svg" width="32" height="30" alt="WebStorm logo" />
<img src="img/editors/sublime.svg" width="32" height="30" alt="Sublime logo" />
<img src="img/editors/vscode.svg" width="32" height="30" alt="Visual logo" />
<img src="img/editors/atom.svg" width="32" height="30" alt="Atom logo" />

## Get started

If you haven’t used the feature, find this button on the right side of your screen:

<img src="img/open-in-editor.svg" width="20" height="20" alt="Open in Editor icon" style="margin:0 5px;" /> 

When you click it, a small popup appears where you’ll need to set two things: a projects path and your editor.

- The **projects path** is the folder where you store your projects. Enter any Linux, Windows, or macOS absolute path.
  - Example: If your projects path is `/home/username/projects`, one of your repos
    is https://github.com/sourcegraph/sourcegraph/, and you want to open `README.md` in the root folder of the repo,
    then the feature will look for the file at `/home/username/project/sourcegraph/README.md`.
- Then choose your **preferred editor** from the list. We support VS Code, Atom, Sublime Text, and JetBrains IDEs (
  AppCode, CLion, GoLand, IntelliJ IDEA, PhpStorm, PyCharm, Rider, RubyMine, WebStorm, and all new IDEs that JetBrains
  releases). If you use a different editor, check out “Advanced config” below.

Once you click **Save**, the icon will you’ll be able to jump to your editor from any file in Sourcegraph.

## Advanced config

If you want to dig deeper, there are a bunch of optional settings to play with. Click your name at the top-right, select
Settings, and add the `"openInEditor": {}` key to your user setting JSON.

- **editorIds:** an array of short names of an editor (“vscode”, “idea”, “sublime”, “pycharm”, etc.) or “custom”. This is what you set in the dropdown when you first set up the feature.
  - Note that the initial setup UI only allows you to select one editor, but if you use multiple editors, you can manually add more. Each of them will have an icon on your toolbar.
- **custom.urlPattern:** If you set editorId to “custom” then this must be set, too. Use the placeholders “%file”, “%line”, and “%col” to mark where the file path, line number, and column number must be inserted. Example URL pattern for IntelliJ IDEA: `idea://open?file=%file&line=%line&column=%col`
- **projectPaths.default**: This is what you set in the form when you first set up the feature: The absolute path on your computer where your git repositories live. All git repos to open have to be cloned under this path with their original names. `/Users/yourusername/src` is a valid absolute path, `~/src` is not. Works both with and without a trailing slash.
- **projectPaths.linux**: Overwrites the default path on Linux. Handy if you use different environments. Works both with and without a trailing slash.
- **projectPaths.mac**: Overwrites the default path on macOS. Works both with and without a trailing slash.
- **projectPaths.windows**: Overwrites the default path on Windows. Works both with and without a trailing backslash.
- **replacements**: Key-value pairs. Each key will be replaced by the corresponding value in the final URL. Keys are regular expressions, values can contain backreferences ($1, $2, ...).
- **jetbrains.forceApi**: Forces using protocol handlers (like `idea://open?file=...`) or the built-in REST API (`http://localhost:63342/api/file...`). If omitted, **protocol handlers** are used if available, otherwise the built-in REST API is used.
- **vscode.isProjectPathUNCPath**: Indicates that the given projects path is a UNC (Universal Naming Convention) path.
- **vscode.useInsiders**: If set, files will open in VS Code Insiders rather than VS Code.
- **vscode.useSSH**: If set, files will open on a remote server via SSH. This requires vscode.remoteHostForSSH to be specified and VS Code extension “Remote Development by Microsoft” installed in your VS Code.
- **vscode.remoteHostForSSH**: The remote host as `USER@HOSTNAME`. This needs you to install the extension called “Remote Development by Microsoft” in your VS Code.

Just save your settings and enjoy being totally advanced! 😎

## Examples

### IntelliJ IDEA on macOS

To open repository files in your Documents directory:

```json
{
  "openInEditor": {
    "editorId": "idea",
    "projectPaths.default": "/Users/USERNAME/Documents"
  }
}
```

### JetBrains IDEs on Windows

**Note:** We talk about IntelliJ here, but it’s similar for all JetBrains IDEs.

The `idea://` protocol handler does not always work on Windows. If it fails for you, a workaround is to use IntelliJ’s built-in REST API to open files directly from a URL with some extra configuration steps.

Add this to your Sourcegraph _User settings_:

```json
{
  "openInEditor": {
    ...
    "jetbrains.forceApi": "builtInServer"
  }
}
```

Then open IntelliJ settings, go to `Build, Execution, Deployment` | `Debugger` | `Built-in Server`, and enable `Allow unsigned requests`. This allows Sourcegraph to make requests to the built-in server, as stated in JetBrains’ [docs](https://www.jetbrains.com/help/idea/php-built-in-web-server.html#configuring-built-in-web-server).

**Note:** with this workaround, “Open in Editor” will only work if your IDE is running.

### VS Code on macOS

To open repository files in your Documents directory:

```json
{
  "openInEditor": {
    "editorId": "vscode",
    "projectPaths.default": "/Users/USERNAME/Documents"
  }
}
```

### VS Code on Windows

To open repository files in your Documents directory:

```json
{
  "openInEditor": {
    "editorId": "vscode",
    "projectPaths.default": "C:\Users\USERNAME\Documents"
  }
}
```

### VS Code on WSL

To open repository files in your Home directory:

```json
{
  "openInEditor": {
    "editorId": "vscode",
    "projectPaths.default": "//wsl$/Ubuntu-18.04/home"
  }
}
```

### VS Code Insider on Mac

```json
{
  "openInEditor": {
    ...
    "vscode.useInsiders": true
  }
}
```

### VS Code with different base paths configured for different platforms

Uses the assigned path for the detected Operating System when available:

```json
{
  "openInEditor": {
    "editorId": "vscode",
    // basePath is required as the default path when no Operating System is detected
    "projectPaths.default": "/Users/USERNAME/Documents/",
    "projectPaths.windows": "/C:/Users/USERNAME/folder/",
    "projectPaths.mac": "/Users/USERNAME/folder/",
    "projectPaths.linux": "/home/USERNAME/folder/"
  }
}
```

### Replacement Example 1: Open Remote folders with VS Code on Mac by removing file names

**This requires VS Code extension [Remote Development by Microsoft](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.vscode-remote-extensionpack) to work.**

To open directory where the repository files reside in a remote server:

```json
{
  "openInEditor": {
    "editorId": "custom",
    "projectPaths.default": "/Users/USERNAME/Documents/",
    // Replaces USER@HOSTNAME as appropriate.
    "custom.urlPattern": "vscode://vscode-remote/ssh-remote+USER@HOSTNAME%file",
    //removes file name as the vscode-remote protocol handler only supports directory-opening
    "replacements": { "\/[^\/]*$": "" }
  }
}
```

### Replacement Example 2: Add string to final file path

Adds `sourcegraph-` in front of the string that matches the `(?<=Documents\/)(.*[\\\/])` RegExp pattern, which is the string after `Documents/` and before the final slash.

```json
{
  "openInEditor": {
    "editorId": "vscode",
    "projectPaths.default": "/Users/USERNAME/Documents/",
    "replacements": { "(?<=Documents\/)(.*[\\\/])": "sourcegraph-$1" }
    // vscode://file//Users/USERNAME/Documents/REPO-NAME/package.json => vscode://file//Users/USERNAME/Documents/sourcegraph-REPO-NAME/package.json
  }
}
```

### Replacement Example 3: Remove string from the final file path

Removes `sourcegraph-` from the final URL.

```json
{
  "openInEditor": {
    "editorId": "vscode",
    "projectPaths.default": "/Users/USERNAME/Documents/",
    "replacements": { "sourcegraph-": "" }
    // vscode://file//Users/USERNAME/Documents/sourcegraph-REST-OF_REPO-NAME/package.json => vscode://file//Users/USERNAME/Documents/REST-OF_REPO-NAME/package.json
  }
}
```
