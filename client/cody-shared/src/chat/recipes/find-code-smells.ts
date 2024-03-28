import { CHARS_PER_TOKEN, MAX_AVAILABLE_PROMPT_LENGTH, MAX_RECIPE_INPUT_TOKENS } from '../../prompt/constants'
import { truncateText } from '../../prompt/truncation'
import { Interaction } from '../transcript/interaction'

import { getNormalizedLanguageName } from './helpers'
import type { Recipe, RecipeContext, RecipeID } from './recipe'

export class FindCodeSmells implements Recipe {
    public id: RecipeID = 'find-code-smells'
    public title = 'Smell Code'

    public async getInteraction(_humanChatInput: string, context: RecipeContext): Promise<Interaction | null> {
        const selection = context.editor.getActiveTextEditorSelectionOrEntireFile()
        if (!selection) {
            await context.editor.showWarningMessage('No code selected. Please select some code and try again.')
            return Promise.resolve(null)
        }

        const languageName = getNormalizedLanguageName(selection.fileName)
        const promptPrefix = `Find code smells, potential bugs, and unhandled errors in my ${languageName} code:`
        const promptSuffix = `List maximum five of them as a list (if you have more in mind, mention that these are the top five), with a short context, reasoning, and suggestion on each.
If you have no ideas because the code looks fine, feel free to say that it already looks fine.`

        // Use the whole context window for the prompt because we're attaching no files
        const maxTokenCount =
            MAX_AVAILABLE_PROMPT_LENGTH - (promptPrefix.length + promptSuffix.length) / CHARS_PER_TOKEN
        const truncatedSelectedText = truncateText(
            selection.selectedText,
            Math.min(maxTokenCount, MAX_RECIPE_INPUT_TOKENS)
        )
        const promptMessage = `${promptPrefix}\n\n\`\`\`\n${truncatedSelectedText}\n\`\`\`\n\n${promptSuffix}`

        const displayText = `Find code smells in the following code: \n\`\`\`\n${selection.selectedText}\n\`\`\``

        const assistantResponsePrefix = ''
        return new Interaction(
            { speaker: 'human', text: promptMessage, displayText },
            {
                speaker: 'assistant',
                prefix: assistantResponsePrefix,
                text: assistantResponsePrefix,
            },
            new Promise(resolve => resolve([])),
            []
        )
    }
}
