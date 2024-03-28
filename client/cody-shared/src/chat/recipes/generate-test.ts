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

export class GenerateTest implements Recipe {
    public id: RecipeID = 'generate-unit-test'
    public title = 'Generate Unit Test'

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

        const languageName = getNormalizedLanguageName(selection.fileName)
        const promptMessage = `Generate a unit test in ${languageName} for the following code:\n\`\`\`${extension}\n${truncatedSelectedText}\n\`\`\`\n${MARKDOWN_FORMAT_PROMPT}`
        const assistantResponsePrefix = `Here is the generated unit test:\n\`\`\`${extension}\n`

        const displayText = `Generate a unit test for the following code:\n\`\`\`${extension}\n${selection.selectedText}\n\`\`\``

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
