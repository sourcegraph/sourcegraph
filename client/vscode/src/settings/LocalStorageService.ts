// VS Code Docs https://code.visualstudio.com/api/references/vscode-api#Memento
// A memento represents a storage utility. It can store and retrieve values.
import type { Memento } from 'vscode'

export class LocalStorageService {
    constructor(private storage: Memento) {}

    public getValue(key: string): string {
        return this.storage.get<string>(key, '')
    }

    public async setValue(key: string, value: string): Promise<boolean> {
        try {
            await this.storage.update(key, value)
            return true
        } catch {
            return false
        }
    }
}

export const SELECTED_SEARCH_CONTEXT_SPEC_KEY = 'selected-search-context-spec'
export const INSTANCE_VERSION_NUMBER_KEY = 'sourcegraphVersionNumber'
export const ANONYMOUS_USER_ID_KEY = 'sourcegraphAnonymousUid'
export const DISMISS_WORKSPACERECS_CTA_KEY = 'sourcegraphWorkspaceRecsCtaDismissed'
