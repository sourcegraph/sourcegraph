import vscode from 'vscode'

const outputChannel = vscode.window.createOutputChannel('Sourcegraph')

export const log = {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    error: (what: string, error?: any): void => {
        outputChannel.appendLine(`ERROR ${errorMessage(what, error)}`)
    },
    errorAndThrow: (what: string, error?: any): never => {
        log.error(what, error)
        throw new Error(errorMessage(what, error))
    },
    debug: (what: any): void => {
        for (const key of Object.keys(what)) {
            const value = JSON.stringify(what[key])
            outputChannel.appendLine(`${key}=${value}`)
        }
    },
    appendLine: (message: string): void => {
        outputChannel.appendLine(message)
    },
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function errorMessage(what: string, error?: any): string {
    const errorMessage =
        error instanceof Error
            ? ` ${error.message} ${error.stack || ''}`
            : error !== undefined
            ? ` ${JSON.stringify(error)}`
            : ''
    return what + errorMessage
}
