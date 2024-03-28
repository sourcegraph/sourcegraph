import { MAX_RECIPE_INPUT_TOKENS, MAX_RECIPE_SURROUNDING_TOKENS } from '../../prompt/constants'
import { truncateText, truncateTextStart } from '../../prompt/truncation'
import { Interaction } from '../transcript/interaction'

import {
    getContextMessagesFromSelection,
    getFileExtension,
    getNormalizedLanguageName,
    MARKDOWN_FORMAT_PROMPT,
} from './helpers'
import type { Recipe, RecipeContext, RecipeID } from './recipe'

export class ImproveVariableNames implements Recipe {
    public id: RecipeID = 'improve-variable-names'
    public title = 'Improve Variable Names'

    public async getInteraction(_humanChatInput: string, context: RecipeContext): Promise<Interaction | null> {
        const selection = context.editor.getActiveTextEditorSelectionOrEntireFile()
        if (!selection) {
            await context.editor.showWarningMessage('No code selected. Please select some code and try again.')
            return Promise.resolve(null)
        }

        const truncatedSelectedText = truncateText(selection.selectedText, MAX_RECIPE_INPUT_TOKENS)
        const truncatedPrecedingText = truncateTextStart(selection.precedingText, MAX_RECIPE_SURROUNDING_TOKENS)
        const truncatedFollowingText = truncateText(selection.followingText, MAX_RECIPE_SURROUNDING_TOKENS)
        const extension = getFileExtension(selection.fileName)

        const displayText = `Improve the variable names in the following code:\n\`\`\`\n${selection.selectedText}\n\`\`\``

        const languageName = getNormalizedLanguageName(selection.fileName)
        const promptMessage = `Improve the variable names in this ${languageName} code by replacing the variable names with new identifiers which succinctly capture the purpose of the variable. We want the new code to be a drop-in replacement, so do not change names bound outside the scope of this code, like function names or members defined elsewhere. Only change the names of local variables and parameters:\n\n\`\`\`${extension}\n${truncatedSelectedText}\n\`\`\`\n${MARKDOWN_FORMAT_PROMPT}`
        const assistantResponsePrefix = `Here is the improved code:\n\`\`\`${extension}\n`

        return new Interaction(
            { speaker: 'human', text: promptMessage, displayText },
            {
                speaker: 'assistant',
                prefix: assistantResponsePrefix,
                text: assistantResponsePrefix,
            },
            getContextMessagesFromSelection(
                truncatedSelectedText,
                truncatedPrecedingText,
                truncatedFollowingText,
                selection,
                context.codebaseContext
            ),
            []
        )
    }
}
