import { MAX_RECIPE_INPUT_TOKENS, MAX_RECIPE_SURROUNDING_TOKENS } from '../../prompt/constants'
import { truncateText, truncateTextStart } from '../../prompt/truncation'
import { Interaction } from '../transcript/interaction'

import { getContextMessagesFromSelection, getNormalizedLanguageName, MARKDOWN_FORMAT_PROMPT } from './helpers'
import type { Recipe, RecipeContext, RecipeID } from './recipe'

export class ExplainCodeHighLevel implements Recipe {
    public id: RecipeID = 'explain-code-high-level'
    public title = 'Explain Selected Code (High Level)'

    public async getInteraction(_humanChatInput: string, context: RecipeContext): Promise<Interaction | null> {
        const selection = context.editor.getActiveTextEditorSelectionOrEntireFile()
        if (!selection) {
            await context.editor.showWarningMessage('No code selected. Please select some code and try again.')
            return Promise.resolve(null)
        }

        const truncatedSelectedText = truncateText(selection.selectedText, MAX_RECIPE_INPUT_TOKENS)
        const truncatedPrecedingText = truncateTextStart(selection.precedingText, MAX_RECIPE_SURROUNDING_TOKENS)
        const truncatedFollowingText = truncateText(selection.followingText, MAX_RECIPE_SURROUNDING_TOKENS)

        const languageName = getNormalizedLanguageName(selection.fileName)
        const promptMessage = `Explain the following ${languageName} code at a high level. Only include details that are essential to an overall understanding of what's happening in the code.\n\`\`\`\n${truncatedSelectedText}\n\`\`\`\n${MARKDOWN_FORMAT_PROMPT}`
        const displayText = `Explain the following code at a high level:\n\`\`\`\n${selection.selectedText}\n\`\`\``

        return new Interaction(
            { speaker: 'human', text: promptMessage, displayText },
            { speaker: 'assistant' },
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
