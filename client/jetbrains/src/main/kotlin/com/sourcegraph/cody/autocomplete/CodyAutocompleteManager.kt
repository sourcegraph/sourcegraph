package com.sourcegraph.cody.autocomplete

import com.intellij.codeInsight.lookup.LookupManager
import com.intellij.lang.Language
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.command.CommandProcessor
import com.intellij.openapi.components.Service
import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.editor.Inlay
import com.intellij.openapi.editor.InlayModel
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Disposer
import com.intellij.util.concurrency.annotations.RequiresEdt
import com.sourcegraph.cody.agent.CodyAgent.Companion.getServer
import com.sourcegraph.cody.agent.CodyAgentManager.tryRestartingAgentIfNotRunning
import com.sourcegraph.cody.agent.protocol.AutocompleteExecuteParams
import com.sourcegraph.cody.agent.protocol.Position
import com.sourcegraph.cody.autocomplete.InlayModelUtil.getAllInlaysForEditor
import com.sourcegraph.cody.autocomplete.render.AutocompleteRendererType
import com.sourcegraph.cody.autocomplete.render.CodyAutocompleteBlockElementRenderer
import com.sourcegraph.cody.autocomplete.render.CodyAutocompleteElementRenderer
import com.sourcegraph.cody.autocomplete.render.CodyAutocompleteSingleLineRenderer
import com.sourcegraph.cody.statusbar.CodyAutocompleteStatus
import com.sourcegraph.cody.statusbar.CodyAutocompleteStatusService.Companion.notifyApplication
import com.sourcegraph.cody.statusbar.CodyAutocompleteStatusService.Companion.resetApplication
import com.sourcegraph.cody.vscode.CancellationToken
import com.sourcegraph.cody.vscode.InlineAutocompleteItem
import com.sourcegraph.cody.vscode.InlineAutocompleteList
import com.sourcegraph.cody.vscode.InlineCompletionTriggerKind
import com.sourcegraph.cody.vscode.IntelliJTextDocument
import com.sourcegraph.cody.vscode.TextDocument
import com.sourcegraph.config.ConfigUtil.isCodyEnabled
import com.sourcegraph.config.UserLevelConfig
import com.sourcegraph.telemetry.GraphQlLogger
import com.sourcegraph.utils.CodyEditorUtil.getAllOpenEditors
import com.sourcegraph.utils.CodyEditorUtil.getLanguage
import com.sourcegraph.utils.CodyEditorUtil.getTextRange
import com.sourcegraph.utils.CodyEditorUtil.isCommandExcluded
import com.sourcegraph.utils.CodyEditorUtil.isEditorValidForAutocomplete
import com.sourcegraph.utils.CodyEditorUtil.isImplicitAutocompleteEnabledForEditor
import difflib.Delta
import difflib.DiffUtils
import difflib.Patch
import java.util.*
import java.util.concurrent.CancellationException
import java.util.concurrent.CompletableFuture
import java.util.concurrent.CompletionException
import java.util.concurrent.atomic.AtomicReference
import java.util.function.Consumer
import java.util.stream.Collectors

/** Responsible for triggering and clearing inline code completions (the autocomplete feature). */
@Service
class CodyAutocompleteManager {
  private val logger = Logger.getInstance(CodyAutocompleteManager::class.java)
  private val currentJob = AtomicReference(CancellationToken())
  var currentAutocompleteTelemetry: AutocompleteTelemetry? = null

  /**
   * Clears any already rendered autocomplete suggestions for the given editor and cancels any
   * pending ones.
   *
   * @param editor the editor to clear autocomplete suggestions for
   */
  @RequiresEdt
  fun clearAutocompleteSuggestions(editor: Editor) {
    // Log "suggested" event and clear current autocompletion
    Optional.ofNullable(editor.project).ifPresent { p: Project? ->
      if (currentAutocompleteTelemetry != null &&
          currentAutocompleteTelemetry!!.getStatus() !=
              AutocompletionStatus.TRIGGERED_NOT_DISPLAYED) {
        currentAutocompleteTelemetry!!.markCompletionHidden()
        GraphQlLogger.logAutocompleteSuggestedEvent(
            p!!,
            currentAutocompleteTelemetry!!.latencyMs,
            currentAutocompleteTelemetry!!.displayDurationMs,
            currentAutocompleteTelemetry!!.params())
        currentAutocompleteTelemetry = null
      }
    }

    // Cancel any running job
    cancelCurrentJob(editor.project)

    // Clear any existing inline elements
    disposeInlays(editor)
  }

  /**
   * Clears any already rendered autocomplete suggestions for all open editors and cancels any
   * pending ones.
   */
  @RequiresEdt
  fun clearAutocompleteSuggestionsForAllProjects() {
    getAllOpenEditors().forEach(Consumer { editor: Editor -> clearAutocompleteSuggestions(editor) })
  }

  @RequiresEdt
  fun clearAutocompleteSuggestionsForLanguageIds(languageIds: List<String?>) {
    getAllOpenEditors()
        .stream()
        .filter { e: Editor? ->
          Optional.ofNullable(getLanguage(e!!))
              .map { l: Language -> languageIds.contains(l.id) }
              .orElse(false)
        }
        .forEach { editor: Editor -> clearAutocompleteSuggestions(editor) }
  }

  @RequiresEdt
  fun clearAutocompleteSuggestionsForLanguageId(languageId: String) {
    clearAutocompleteSuggestionsForLanguageIds(listOf(languageId))
  }

  @RequiresEdt
  fun disposeInlays(editor: Editor) {
    if (editor.isDisposed) {
      return
    }
    getAllInlaysForEditor(editor)
        .stream()
        .filter { inlay: Inlay<*> -> inlay.renderer is CodyAutocompleteElementRenderer }
        .forEach { disposable: Inlay<*>? -> Disposer.dispose(disposable!!) }
  }

  /**
   * Triggers auto-complete suggestions for the given editor at the specified offset.
   *
   * @param editor The editor instance to provide autocomplete for.
   * @param offset The character offset in the editor to trigger auto-complete at.
   */
  fun triggerAutocomplete(editor: Editor, offset: Int, triggerKind: InlineCompletionTriggerKind) {
    val isTriggeredExplicitly = triggerKind == InlineCompletionTriggerKind.INVOKE
    val isTriggeredImplicitly = !isTriggeredExplicitly
    if (!isCodyEnabled()) {
      if (isTriggeredExplicitly) {
        logger.warn("ignoring explicit autocomplete because Cody is disabled")
      }
      return
    }
    if (!isEditorValidForAutocomplete(editor)) {
      if (isTriggeredExplicitly) {
        logger.warn("triggered autocomplete with invalid editor $editor")
      }
      return
    }
    if (isTriggeredImplicitly && !isImplicitAutocompleteEnabledForEditor(editor)) {
      return
    }
    val currentCommand = CommandProcessor.getInstance().currentCommandName
    if (isTriggeredImplicitly && isCommandExcluded(currentCommand)) {
      return
    }
    val project = editor.project
    if (project == null) {
      logger.warn("triggered autocomplete with null project")
      return
    }
    currentAutocompleteTelemetry = AutocompleteTelemetry.createAndMarkTriggered()
    val textDocument: TextDocument = IntelliJTextDocument(editor, project)
    val autoCompleteDocumentContext = textDocument.getAutocompleteContext(offset)
    if (isTriggeredImplicitly && !autoCompleteDocumentContext.isCompletionTriggerValid()) {
      return
    }
    cancelCurrentJob(project)
    val cancellationToken = CancellationToken()
    currentJob.set(cancellationToken)
    val autocompleteRequest =
        triggerAutocompleteAsync(
            project, editor, offset, textDocument, triggerKind, cancellationToken)
    cancellationToken.onCancellationRequested { autocompleteRequest.cancel(true) }
  }

  /** Asynchronously triggers auto-complete for the given editor and offset. */
  private fun triggerAutocompleteAsync(
      project: Project,
      editor: Editor,
      offset: Int,
      textDocument: TextDocument,
      triggerKind: InlineCompletionTriggerKind,
      cancellationToken: CancellationToken
  ): CompletableFuture<Void?> {
    if (triggerKind == InlineCompletionTriggerKind.INVOKE) {
      tryRestartingAgentIfNotRunning(project)
    }
    val server = getServer(project)
    val isAgentAutocomplete = server != null
    if (!isAgentAutocomplete) {
      logger.warn("Doing nothing, Agent is not running")
      return CompletableFuture.completedFuture(null)
    }
    val position = textDocument.positionAt(offset)
    val virtualFile =
        FileDocumentManager.getInstance().getFile(editor.document)
            ?: return CompletableFuture.completedFuture(null)
    val params =
        AutocompleteExecuteParams()
            .setFilePath(virtualFile.path)
            .setPosition(Position().setLine(position.line).setCharacter(position.character))
    notifyApplication(CodyAutocompleteStatus.AutocompleteInProgress)
    val completions = server!!.autocompleteExecute(params)

    // Important: we have to `.cancel()` the original `CompletableFuture<T>` from lsp4j. As soon as
    // we use `thenAccept()` we get a new instance of `CompletableFuture<Void>` which does not
    // correctly propagate the cancellation to the agent.
    cancellationToken.onCancellationRequested { completions.cancel(true) }
    return completions
        .thenAccept { result: InlineAutocompleteList ->
          processAutocompleteResult(project, editor, offset, triggerKind, result, cancellationToken)
        }
        .exceptionally { error: Throwable? ->
          if (!(error is CancellationException || error is CompletionException)) {
            logger.warn("failed autocomplete request $params", error)
          }
          null
        }
        .thenAccept { resetApplication(project) }
  }

  private fun processAutocompleteResult(
      project: Project,
      editor: Editor,
      offset: Int,
      triggerKind: InlineCompletionTriggerKind,
      result: InlineAutocompleteList,
      cancellationToken: CancellationToken
  ) {
    if (currentAutocompleteTelemetry != null) {
      currentAutocompleteTelemetry!!.markCompletionEvent(result.completionEvent)
    }
    if (Thread.interrupted() || cancellationToken.isCancelled) {
      if (triggerKind == InlineCompletionTriggerKind.INVOKE) {
        logger.warn("autocomplete canceled")
      }
      return
    }
    val inlayModel = editor.inlayModel
    val maybeItem = result.items.stream().findFirst()
    if (maybeItem.isEmpty) {
      if (triggerKind == InlineCompletionTriggerKind.INVOKE) {
        logger.warn("explicit autocomplete returned empty suggestions")
        // NOTE(olafur): it would be nice to give the user a visual hint when this happens.
        // We don't do anything now because it's unclear what would be the most idiomatic
        // IntelliJ API to use.
      }
      return
    }
    val item = maybeItem.get()
    ApplicationManager.getApplication().invokeLater {
      if (cancellationToken.isCancelled) {
        return@invokeLater
      }
      cancellationToken.dispose()
      clearAutocompleteSuggestions(editor)
      currentAutocompleteTelemetry?.markCompletionDisplayed()

      // Avoid displaying autocomplete when IntelliJ is already displaying
      // built-in completions. When built-in completions are visible, we can't
      // accept the Cody autocomplete with TAB because it accepts the built-in
      // completion.
      if (LookupManager.getInstance(project).activeLookup != null) {
        if (triggerKind == InlineCompletionTriggerKind.INVOKE ||
            UserLevelConfig.isVerboseLoggingEnabled()) {
          logger.warn("Skipping autocomplete because lookup is active: $item")
        }
        return@invokeLater
      }
      displayAgentAutocomplete(editor, offset, item, inlayModel, triggerKind)
    }
  }

  /**
   * Render inlay hints for unprocessed autocomplete results from the agent.
   *
   * The reason we have a custom code path to render hints for agent autocompletions is because we
   * can use `insertText` directly and the `range` encloses the entire line.
   */
  private fun displayAgentAutocomplete(
      editor: Editor,
      offset: Int,
      item: InlineAutocompleteItem,
      inlayModel: InlayModel,
      triggerKind: InlineCompletionTriggerKind
  ) {
    val range = getTextRange(editor.document, item.range)
    val originalText = editor.document.getText(range)
    val insertTextFirstLine: String = item.insertText.lines().firstOrNull() ?: ""
    val multilineInsertText: String = item.insertText.lines().drop(1).joinToString(separator = "\n")

    // Run Myers diff between the existing text in the document and the first line of the
    // `insertText` that is returned from the agent.
    // The diff algorithm returns a list of "deltas" that give us the minimal number of additions we
    // need to make to the document.
    val patch = diff(originalText, insertTextFirstLine)
    if (!patch.getDeltas().stream().allMatch { delta: Delta<String> ->
      delta.type == Delta.TYPE.INSERT
    }) {
      if (triggerKind == InlineCompletionTriggerKind.INVOKE ||
          UserLevelConfig.isVerboseLoggingEnabled()) {
        logger.warn("Skipping autocomplete with non-insert deltas: $patch")
      }
      // Skip completions that need to delete or change characters in the existing document. We only
      // want completions to add changes to the document.
      return
    }

    // Insert one inlay hint per delta in the first line.
    for (delta in patch.getDeltas()) {
      val text = java.lang.String.join("", delta.revised.lines)
      inlayModel.addInlineElement(
          range.startOffset + delta.original.position,
          true,
          CodyAutocompleteSingleLineRenderer(text, item, editor, AutocompleteRendererType.INLINE))
    }

    // Insert remaining lines of multiline completions as a single block element under the
    // (potentially false?) assumption that we don't need to compute diffs for them. My
    // understanding of multiline completions is that they are only supposed to be triggered in
    // situations where we insert a large block of code in an empty block.
    if (multilineInsertText.isNotEmpty()) {
      inlayModel.addBlockElement(
          offset,
          true,
          false,
          Int.MAX_VALUE,
          CodyAutocompleteBlockElementRenderer(multilineInsertText, item, editor))
    }
  }

  private fun cancelCurrentJob(project: Project?) {
    currentJob.get().abort()
    resetApplication(project!!)
  }

  companion object {
    val instance: CodyAutocompleteManager
      get() = ApplicationManager.getApplication().getService(CodyAutocompleteManager::class.java)

    private fun diff(a: String, b: String): Patch<String> =
        DiffUtils.diff(characterList(a), characterList(b))

    private fun characterList(value: String): List<String> =
        value.chars().mapToObj { c: Int -> c.toChar().toString() }.collect(Collectors.toList())
  }
}
