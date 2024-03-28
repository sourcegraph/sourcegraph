import * as defaultPrompts from './default-prompts.json'
import { toSlashCommand } from './utils'

export function getDefaultCommandsMap(editorCommands: CodyPrompt[] = []): Map<string, CodyPrompt> {
    const map = new Map<string, CodyPrompt>()

    // Add editor specifc commands
    for (const command of editorCommands) {
        if (command.slashCommand) {
            map.set(command.slashCommand, command)
        }
    }

    // Add default commands
    const prompts = defaultPrompts.commands as Record<string, unknown>
    for (const key in prompts) {
        if (Object.prototype.hasOwnProperty.call(prompts, key)) {
            const prompt = prompts[key] as CodyPrompt
            prompt.type = 'default'
            prompt.slashCommand = toSlashCommand(key)
            map.set(prompt.slashCommand, prompt)
        }
    }

    return map
}

export interface MyPrompts {
    // A set of reusable commands where instructions (prompts) and context can be configured.
    commands: Map<string, CodyPrompt>
    // backward compatibility
    recipes?: Map<string, CodyPrompt>
}

// JSON format of MyPrompts
export interface MyPromptsJSON {
    commands: { [id: string]: Omit<CodyPrompt, 'slashCommand'> }
    recipes?: { [id: string]: CodyPrompt }
}

export interface CodyPrompt {
    description?: string
    prompt: string
    context?: CodyPromptContext
    type?: CodyPromptType
    slashCommand: string
}

// Type of context available for prompt building
export interface CodyPromptContext {
    codebase: boolean
    openTabs?: boolean
    currentDir?: boolean
    currentFile?: boolean
    selection?: boolean
    command?: string
    output?: string
    filePath?: string
    directoryPath?: string
    none?: boolean
}

export type CodyPromptType = 'workspace' | 'user' | 'default' | 'recently used'

export type CustomCommandType = 'workspace' | 'user'

export const ConfigFileName = {
    vscode: '.vscode/cody.json',
}

// Default to not include codebase context
export const defaultCodyPromptContext: CodyPromptContext = {
    codebase: false,
}
