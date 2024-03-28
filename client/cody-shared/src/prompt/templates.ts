import path from 'path'

import { getFileExtension, getNormalizedLanguageName } from '../chat/recipes/helpers'
import type { ActiveTextEditorDiagnostic, ActiveTextEditorSelection } from '../editor'

import { MAX_RECIPE_INPUT_TOKENS } from './constants'
import { truncateText, truncateTextStart } from './truncation'

const CODE_CONTEXT_TEMPLATE = `Use following code snippet from file \`{filePath}\`:
\`\`\`{language}
{text}
\`\`\``

const CODE_CONTEXT_TEMPLATE_WITH_REPO = `Use following code snippet from file \`{filePath}\` in repository \`{repoName}\`:
\`\`\`{language}
{text}
\`\`\``

export function populateCodeContextTemplate(code: string, filePath: string, repoName?: string): string {
    return (repoName ? CODE_CONTEXT_TEMPLATE_WITH_REPO.replace('{repoName}', repoName) : CODE_CONTEXT_TEMPLATE)
        .replace('{filePath}', filePath)
        .replace('{language}', getExtension(filePath))
        .replace('{text}', code)
}

const PRECISE_CONTEXT_TEMPLATE = `The symbol '{symbol}' is defined in the file {filePath} as:
\`\`\`{language}
{text}
\`\`\``

export function populatePreciseCodeContextTemplate(symbol: string, filePath: string, code: string): string {
    return PRECISE_CONTEXT_TEMPLATE.replace('{symbol}', symbol)
        .replace('{filePath}', filePath)
        .replace('{language}', getExtension(filePath))
        .replace('{text}', code)
}

const MARKDOWN_CONTEXT_TEMPLATE = 'Use the following text from file `{filePath}`:\n{text}'

const MARKDOWN_CONTEXT_TEMPLATE_WITH_REPO =
    'Use the following text from file `{filePath}` in repository `{repoName}`:\n{text}'

export function populateMarkdownContextTemplate(markdown: string, filePath: string, repoName?: string): string {
    return (repoName ? MARKDOWN_CONTEXT_TEMPLATE_WITH_REPO.replace('{repoName}', repoName) : MARKDOWN_CONTEXT_TEMPLATE)
        .replace('{filePath}', filePath)
        .replace('{text}', markdown)
}

const CURRENT_EDITOR_CODE_TEMPLATE = 'I have the `{filePath}` file opened in my editor. '

const CURRENT_EDITOR_CODE_TEMPLATE_WITH_REPO =
    'I have the `{filePath}` file from the repository `{repoName}` opened in my editor. '

export function populateCurrentEditorContextTemplate(code: string, filePath: string, repoName?: string): string {
    const context = isMarkdownFile(filePath)
        ? populateMarkdownContextTemplate(code, filePath, repoName)
        : populateCodeContextTemplate(code, filePath, repoName)
    return (
        (repoName
            ? CURRENT_EDITOR_CODE_TEMPLATE_WITH_REPO.replace('{repoName}', repoName)
            : CURRENT_EDITOR_CODE_TEMPLATE
        ).replaceAll('{filePath}', filePath) + context
    )
}

const CURRENT_EDITOR_SELECTED_CODE_TEMPLATE = 'Here is the selected {language} code from file path `{filePath}`: '

const CURRENT_EDITOR_SELECTED_CODE_TEMPLATE_WITH_REPO =
    'Here is the selected code from file `{filePath}` in the {repoName} repository, written in {language}: '

export function populateCurrentEditorSelectedContextTemplate(
    code: string,
    filePath: string,
    repoName?: string
): string {
    const extension = getFileExtension(filePath)
    const languageName = getNormalizedLanguageName(extension)
    const context = isMarkdownFile(filePath)
        ? populateMarkdownContextTemplate(code, filePath, repoName)
        : populateCodeContextTemplate(code, filePath, repoName)
    return (
        (repoName
            ? CURRENT_EDITOR_SELECTED_CODE_TEMPLATE_WITH_REPO.replace('{repoName}', repoName)
            : CURRENT_EDITOR_SELECTED_CODE_TEMPLATE
        )
            .replace('{language}', languageName)
            .replaceAll('{filePath}', filePath) + context
    )
}

const DIAGNOSTICS_CONTEXT_TEMPLATE = `Use the following {type} from the code snippet in the file \`{filePath}\`
{prefix}: {message}
Code snippet:
\`\`\`{language}
{code}
\`\`\``

export function populateCurrentEditorDiagnosticsTemplate(
    { message, type, text }: ActiveTextEditorDiagnostic,
    filePath: string
): string {
    const language = getExtension(filePath)
    return DIAGNOSTICS_CONTEXT_TEMPLATE.replace('{type}', type)
        .replace('{filePath}', filePath)
        .replace('{prefix}', type)
        .replace('{message}', message)
        .replace('{language}', language)
        .replace('{code}', text)
}

const COMMAND_OUTPUT_TEMPLATE = 'Here is the output returned from the terminal.\n'

export function populateTerminalOutputContextTemplate(output: string): string {
    return COMMAND_OUTPUT_TEMPLATE + output
}

const MARKDOWN_EXTENSIONS = new Set(['md', 'markdown'])

export function isMarkdownFile(filePath: string): boolean {
    return MARKDOWN_EXTENSIONS.has(getExtension(filePath))
}

function getExtension(filePath: string): string {
    return path.extname(filePath).slice(1)
}

const SELECTED_CODE_CONTEXT_TEMPLATE = `"My selected {languageName} code from file \`{filePath}\`:
<selected>
{code}
</selected>`

const SELECTED_CODE_CONTEXT_TEMPLATE_WITH_REPO = `"My selected {languageName} code from file \`{filePath}\` in \`{repoName}\` repository:
<selected>
{code}
</selected>`

export function populateCurrentSelectedCodeContextTemplate(code: string, filePath: string, repoName?: string): string {
    const extension = getFileExtension(filePath)
    const languageName = getNormalizedLanguageName(extension)
    return (
        repoName
            ? SELECTED_CODE_CONTEXT_TEMPLATE_WITH_REPO.replace('{repoName}', repoName)
            : SELECTED_CODE_CONTEXT_TEMPLATE
    )
        .replace('{code}', code)
        .replaceAll('{filePath}', filePath)
        .replace('{languageName}', languageName)
}

const CURRENT_FILE_CONTEXT_TEMPLATE = `My selected code from file path \`{filePath}\` in <selected> tags:
{precedingText}<selected>{selectedText}</selected>{followingText}`

export function populateCurrentFileFromEditorSelectionContextTemplate(
    selection: ActiveTextEditorSelection,
    filePath: string
): string {
    const extension = getFileExtension(filePath)
    const languageName = getNormalizedLanguageName(extension)
    const surroundingTextLength = (MAX_RECIPE_INPUT_TOKENS - selection.selectedText.length) / 2
    const truncatedSelectedText = truncateText(selection.selectedText, MAX_RECIPE_INPUT_TOKENS) || ''
    const truncatedPrecedingText = truncateTextStart(selection.precedingText, surroundingTextLength)
    const truncatedFollowingText = truncateText(selection.followingText, surroundingTextLength)

    const fileContext = CURRENT_FILE_CONTEXT_TEMPLATE.replace('{languageName}', languageName)
        .replaceAll('{filePath}', filePath)
        .replace('{followingText}', truncatedFollowingText)
        .replace('{selectedText}', truncatedSelectedText)
        .replace('{precedingText}', truncatedPrecedingText)

    return truncateText(fileContext, MAX_RECIPE_INPUT_TOKENS * 3)
}

const DIRECTORY_FILE_LIST_TEMPLATE = 'Here is a list of files from the directory contains {fileName} in my codebase: '
const ROOT_DIRECTORY_FILE_LIST_TEMPLATE = 'Here is a list of files from the root codebase directory: '

export function populateListOfFilesContextTemplate(fileList: string, fileName: string): string {
    const templateText = fileName === 'root' ? ROOT_DIRECTORY_FILE_LIST_TEMPLATE : DIRECTORY_FILE_LIST_TEMPLATE
    return templateText.replace('{fileName}', fileName) + fileList
}

export function populateContextTemplateFromText(templateText: string, content: string, fileName: string): string {
    return templateText.replace('{fileName}', fileName) + content
}

const FILE_IMPORTS_TEMPLATE = '{fileName} has imported the folowing: '

export function populateImportListContextTemplate(importList: string, fileName: string): string {
    return FILE_IMPORTS_TEMPLATE.replace('{fileName}', fileName) + importList
}
