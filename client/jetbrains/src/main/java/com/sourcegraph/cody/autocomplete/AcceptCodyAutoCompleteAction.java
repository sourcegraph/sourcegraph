package com.sourcegraph.cody.autocomplete;

import com.intellij.openapi.actionSystem.DataContext;
import com.intellij.openapi.command.WriteCommandAction;
import com.intellij.openapi.editor.*;
import com.intellij.openapi.editor.actionSystem.EditorAction;
import com.intellij.openapi.editor.actionSystem.EditorActionHandler;
import com.intellij.openapi.util.TextRange;
import com.sourcegraph.cody.autocomplete.render.*;
import com.sourcegraph.telemetry.GraphQlLogger;
import java.util.List;
import java.util.Optional;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

/**
 * The action that gets triggered when the user accepts a Cody completion.
 *
 * <p>The action works by reading the Inlay at the caret position and inserting the completion text
 * into the editor.
 */
public class AcceptCodyAutoCompleteAction extends EditorAction {
  public AcceptCodyAutoCompleteAction() {
    super(new AcceptCompletionActionHandler());
  }

  private static class AcceptCompletionActionHandler extends EditorActionHandler {

    @Override
    protected boolean isEnabledForCaret(
        @NotNull Editor editor, @NotNull Caret caret, DataContext dataContext) {
      // Returns false to fall back to normal TAB character if there is no suggestion at the caret.
      return CodyAutoCompleteManager.isEditorInstanceSupported(editor)
          && AutoCompleteText.atCaret(caret).isPresent();
    }

    @Override
    protected void doExecute(
        @NotNull Editor editor, @Nullable Caret maybeCaret, DataContext dataContext) {
      Optional.ofNullable(maybeCaret)
          .or(
              () -> {
                List<Caret> allCarets = editor.getCaretModel().getAllCarets();
                if (allCarets.size() < 2) { // Only accept completion if there's a single caret.
                  return allCarets.stream().findFirst();
                } else {
                  return Optional.empty();
                }
              })
          .flatMap(AutoCompleteText::atCaret)
          .ifPresent(
              autoComplete -> {
                /* Log the event */
                Optional.ofNullable(editor.getProject())
                    .ifPresent(p -> GraphQlLogger.logCodyEvent(p, "completion", "accepted"));
                WriteCommandAction.runWriteCommandAction(
                    editor.getProject(),
                    "Accept Cody Completion",
                    "Cody", // TODO: what groupID should we use here?
                    () -> {
                      Document document = editor.getDocument();
                      int lineEndOffset =
                          document.getLineEndOffset(
                              document.getLineNumber(autoComplete.caret.getOffset()));
                      document.replaceString(
                          autoComplete.caret.getOffset(),
                          lineEndOffset,
                          autoComplete.autoCompleteText.getAutoCompletionString(
                              document.getText(
                                  TextRange.create(
                                      autoComplete.caret.getOffset(), lineEndOffset))));
                    });
              });
    }
  }
}
