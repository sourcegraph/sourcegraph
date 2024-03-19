# Getting started with running batch changes server-side

## Creating your first batch change

To get started, click on the "Create batch change" button on the Batch Changes page, or go to `/batch-changes/create`.
You will be prompted to choose a name for your namespace and optionally define a custom namespace to put your batch change in.

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/ssbc_create_form.png" class="screenshot">

Once done, click "Create".

### Editing the spec file

You should see the editor view now. This view consists of three main areas:

- The library sidebar panel on the left
- The editor in the middle
- The workspaces preview panel on the right

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/ssbc_editor_panels.png" class="screenshot">

You can pick from the examples in the library pane to get started quickly, or begin working on your batch spec in the editor right away. The editor will provide documentation as you hover over tokens in the YAML spec and supports auto-completion.

### Previewing workspaces

Once satisfied or to test your batch change's scope, you can at any time run a new preview from the right hand side panel. After resolution, it will show all the workspaces in repositories that matched the given `on` statements. You can search through them and determine if your query is satisfying before starting execution. You can also exclude single workspaces from this list.

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/ssbc_workspace_preview.png" class="screenshot">

### Executing your batch spec

When the spec is ready to run, make sure the preview is up to date and then click "Run batch spec". You will be taken to the execution screen.

On this page, you see

- Run statistics at the top
- All the workspaces including status and diff stat in the left panel
- Details on a particular workspace on the right hand side panel where you can see
  - Steps with
    - Command
    - Logs
    - Output variables
    - Per-step diffs
  - Results
  - Execution timelines for debugging

Once finished, you can proceed to the batch spec preview as you know it from before.

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/ssbc_execution_screen.png" class="screenshot">

### Previewing and applying the batch spec

On this page, you can review the changes proposed one more time and also review the operations taken by Sourcegraph on each changeset. Once satisfied, click "Apply".

Congratulations, you ran your first batch change server-side 🎊

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/ssbc_preview_screen.png" class="screenshot">

## Updating your batch change

This section is a work in progress.
