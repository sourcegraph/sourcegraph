import vscode from 'vscode'

export async function displayWarning(warning: string): Promise<void> {
    await vscode.window.showErrorMessage(warning)
}
