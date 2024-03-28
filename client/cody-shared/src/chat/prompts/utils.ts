import { basename, extname } from 'path'

import type { ContextMessage } from '../../codebase-context/messages'
import type { ActiveTextEditorSelection } from '../../editor'
import { CHARS_PER_TOKEN, MAX_AVAILABLE_PROMPT_LENGTH, MAX_RECIPE_INPUT_TOKENS } from '../../prompt/constants'
import { truncateText } from '../../prompt/truncation'
import { getFileExtension, getNormalizedLanguageName } from '../recipes/helpers'
import { Interaction } from '../transcript/interaction'

import type { CodyPromptContext } from '.'
import { prompts } from './templates'

/**
 * Creates a new Interaction object with the given parameters.
 */
export async function newInteraction(args: {
    text?: string
    displayText: string
    contextMessages?: Promise<ContextMessage[]>
    assistantText?: string
    assistantDisplayText?: string
}): Promise<Interaction> {
    const { text, displayText, contextMessages, assistantText, assistantDisplayText } = args
    return Promise.resolve(
        new Interaction(
            { speaker: 'human', text, displayText },
            { speaker: 'assistant', text: assistantText, displayText: assistantDisplayText },
            Promise.resolve(contextMessages || []),
            []
        )
    )
}

/**
 * Returns a Promise resolving to an Interaction object representing an error response from the assistant.
 * @param errorMsg - The error message text to include in the assistant response.
 * @param displayText - Optional human-readable display text for the request.
 * @returns A Promise resolving to the Interaction object.
 */
export async function newInteractionWithError(errorMsg: string, displayText = ''): Promise<Interaction> {
    return Promise.resolve(
        new Interaction(
            { speaker: 'human', displayText },
            { speaker: 'assistant', displayText: errorMsg, error: errorMsg },
            Promise.resolve([]),
            []
        )
    )
}

/**
 * Generates a prompt text string with the provided prompt and code
 * @param prompt - The prompt text to include after the code snippet.
 * @param selection - The ActiveTextEditorSelection containing the code snippet.
 * @returns The constructed prompt text string, or null if no selection provided.
 */
export function promptTextWithCodeSelection(
    prompt: string,
    selection?: ActiveTextEditorSelection | null
): string | null {
    if (!selection) {
        return null
    }
    const extension = getFileExtension(selection.fileName)
    const languageName = getNormalizedLanguageName(extension)
    const codePrefix = `I have this ${languageName} code selected in my editor from my codebase file ${selection.fileName}:`

    // Use the whole context window for the prompt because we're attaching no files
    const maxTokenCount = MAX_AVAILABLE_PROMPT_LENGTH - (codePrefix.length + prompt.length) / CHARS_PER_TOKEN
    const truncatedCode = truncateText(selection.selectedText, Math.min(maxTokenCount, MAX_RECIPE_INPUT_TOKENS))
    const promptText = `${codePrefix}\n\n<selected>\n${truncatedCode}\n</selected>\n\n${prompt}`.replaceAll(
        '{languageName}',
        languageName
    )
    return promptText
}

/**
 * Checks if only the code selection context is required for the prompt.
 * @param contextConfig - The context configuration object.
 * @returns True if only the code selection is required based on the contextConfig, false otherwise.
 *
 * This checks if the contextConfig only contains the "selection" property, or if it contains no properties.
 * In those cases, only the selection context is needed.
 */
export function isOnlySelectionRequired(contextConfig: CodyPromptContext): boolean {
    const contextConfigLength = Object.entries(contextConfig).length
    return !contextConfig.none && ((contextConfig.selection && contextConfigLength === 1) || !contextConfigLength)
}

/**
 * Extracts the test type from the given text.
 * @param text - The text to extract the test type from.
 * @returns The extracted test type, which will be "unit", "e2e", or "integration" if found.
 * Returns an empty string if no match is found.
 */
export function extractTestType(text: string): string {
    // match "unit", "e2e", or "integration" that is follow by the word test, but don't include the word test in the matches
    const testTypeRegex = /(unit|e2e|integration)(?= test)/i
    return text.match(testTypeRegex)?.[0] || ''
}

/**
 * Generates the prompt text to send to the human LLM model.
 * @param commandInstructions - The human's instructions for the command. This will be inserted into the prompt template.
 * @param currentFileName - Optional current file name. If provided, will insert the normalized language name.
 * @returns The constructed prompt text string.
 */
export function getHumanLLMText(commandInstructions: string, currentFileName?: string): string {
    const promptText = prompts.instruction.replace('{humanInput}', commandInstructions)
    if (!currentFileName) {
        return promptText
    }
    return promptText.replaceAll('{languageName}', getNormalizedLanguageName(getFileExtension(currentFileName)))
}

const leadingForwardSlashRegex = /^\/+/

/**
 * Removes leading forward slashes from slash command string.
 */
export function fromSlashCommand(slashCommand: string): string {
    return slashCommand.replace(leadingForwardSlashRegex, '')
}

/**
 * Returns command starting with a forward slash.
 */
export function toSlashCommand(command: string): string {
    // ensure there is only one leading forward slash
    return command.replace(leadingForwardSlashRegex, '').replace(/^/, '/')
}

/**
 * Creates a VS Code search pattern to find files matching the given file path.
 * @param fsPath - The file system path of the file to generate a search pattern for.
 * @param fromRoot - Whether to search from the root directory. Default false.
 * @returns A search pattern string to find matching files.
 *
 * This generates a search pattern by taking the base file name without extension
 * and appending wildcards.
 *
 * If fromRoot is true, the pattern will search recursively from the repo root.
 * Otherwise, it will search only the current directory.
 */
export function createVSCodeSearchPattern(fsPath: string, fromRoot = false): string {
    const fileName = basename(fsPath)
    const fileExtension = extname(fsPath)
    const fileNameWithoutExt = fileName.replace(fileExtension, '')

    const root = fromRoot ? '**' : ''

    const currentFilePattern = `/*${fileNameWithoutExt}*${fileExtension}`
    return root + currentFilePattern
}

export function createVSCodeTestSearchPattern(fsPath: string, allTestFiles?: boolean): string {
    const fileExtension = extname(fsPath)
    const fileName = basename(fsPath, fileExtension)

    const root = '**'
    const defaultTestFilePattern = `/*test*${fileExtension}`
    const currentTestFilePattern = `/*{test_${fileName},${fileName}_test,test.${fileName},${fileName}.test,${fileName}Test}${fileExtension}`

    if (allTestFiles) {
        return `${root}${defaultTestFilePattern}`
    }

    // pattern to search for test files with the same name as current file
    return `${root}${currentTestFilePattern}`
}

/**
 * Creates an object containing the start line and line range
 * of the given editor selection.
 * @param selection - The active text editor selection
 * @returns An object with the following properties:
 * - range: The line range of the selection as a string, e.g. "5-10"
 * - start: The start line of the selection as a string
 * If no selection, range and start will be empty strings.
 */
export function createSelectionDisplayText(selection: ActiveTextEditorSelection): {
    range: string
    start: string
} {
    const start = selection.selectionRange ? `${selection.selectionRange.start.line + 1}` : ''
    const range = selection.selectionRange
        ? `${selection.selectionRange.start.line + 1}-${selection.selectionRange.end.line + 1}`
        : start
    return { range, start }
}

/**
 * Checks if the given file path is a valid test file name.
 * @param fsPath - The file system path to check
 * @returns boolean - True if the path is a valid test file name, false otherwise.
 *
 * Removes file extension and checks if file name starts with 'test' or
 * ends with 'test', excluding files starting with 'test-'.
 * Also returns false for any files in node_modules directory.
 */
export function isValidTestFileName(fsPath: string): boolean {
    // Check if file path contains 'node_modules'
    if (fsPath.includes('node_modules')) {
        return false
    }

    const fileNameWithoutExt = basename(fsPath, extname(fsPath)).toLowerCase()
    // Invalid test file name pattern
    if (fileNameWithoutExt.startsWith('test-')) {
        return false
    }

    // Check if file name starts with 'test' or ends with 'test'
    return fileNameWithoutExt.startsWith('test') || fileNameWithoutExt.endsWith('test')
}
