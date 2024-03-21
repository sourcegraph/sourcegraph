import {
    authentication,
    type AuthenticationProvider,
    type AuthenticationProviderAuthenticationSessionsChangeEvent,
    type AuthenticationSession,
    commands,
    Disposable,
    type Event,
    EventEmitter,
    type SecretStorage,
} from 'vscode'

import polyfillEventSource from '@sourcegraph/shared/src/polyfills/vendor/eventSource'

import { getProxyAgent } from '../../backend/fetch'
import { endpointRequestHeadersSetting, endpointSetting, setEndpoint } from '../../settings/endpointSetting'

export const scretTokenKey = 'SOURCEGRAPH_AUTH'

class SourcegraphAuthSession implements AuthenticationSession {
    public readonly account = {
        id: SourcegraphAuthProvider.id,
        label: SourcegraphAuthProvider.label,
    }
    public readonly id = SourcegraphAuthProvider.id
    public readonly scopes = []

    constructor(public readonly accessToken: string) {}
}

export class SourcegraphAuthProvider implements AuthenticationProvider, Disposable {
    public static id = endpointSetting()
    private static secretKey = scretTokenKey
    public static label = scretTokenKey

    // Kept track of token changes through out the session
    private currentToken: string | undefined
    private initializedDisposable: Disposable | undefined
    private _onDidChangeSessions = new EventEmitter<AuthenticationProviderAuthenticationSessionsChangeEvent>()
    public get onDidChangeSessions(): Event<AuthenticationProviderAuthenticationSessionsChangeEvent> {
        return this._onDidChangeSessions.event
    }

    constructor(private readonly secretStorage: SecretStorage) {}

    public dispose(): void {
        this.initializedDisposable?.dispose()
    }

    private async ensureInitialized(): Promise<void> {
        if (this.initializedDisposable === undefined) {
            await this.cacheTokenFromStorage()
            this.initializedDisposable = Disposable.from(
                this.secretStorage.onDidChange(async event => {
                    if (event.key === SourcegraphAuthProvider.secretKey) {
                        await this.checkForUpdates()
                    }
                }),
                authentication.onDidChangeSessions(async event => {
                    if (event.provider.id === SourcegraphAuthProvider.id) {
                        await this.checkForUpdates()
                    }
                })
            )
        }
    }

    // Check if token has been updated across VS Code
    private async checkForUpdates(): Promise<void> {
        const added: AuthenticationSession[] = []
        const removed: AuthenticationSession[] = []
        const changed: AuthenticationSession[] = []
        const previousToken = this.currentToken
        const session = (await this.getSessions())[0]
        if (session?.accessToken && !previousToken) {
            added.push(session)
        } else if (!session?.accessToken && previousToken) {
            removed.push(session)
        } else if (session?.accessToken !== previousToken) {
            changed.push(session)
        } else {
            return
        }
        await this.cacheTokenFromStorage()
        // Update the polyfillEventSource on token changes
        polyfillEventSource(
            this.currentToken
                ? { Authorization: `token ${this.currentToken}`, ...endpointRequestHeadersSetting() }
                : {},
            getProxyAgent()
        )
        this._onDidChangeSessions.fire({ added, removed, changed })
    }

    // Get token from Storage
    private async cacheTokenFromStorage(): Promise<string | undefined> {
        const token = await this.secretStorage.get(SourcegraphAuthProvider.secretKey)
        this.currentToken = token
        return this.currentToken
    }

    // This is called first when `vscode.authentication.getSessions` is called.
    public async getSessions(_scopes?: string[]): Promise<readonly AuthenticationSession[]> {
        await this.ensureInitialized()
        const token = await this.cacheTokenFromStorage()
        return token ? [new SourcegraphAuthSession(token)] : []
    }

    // This is called after `this.getSessions` is called,
    // and only when `createIfNone` or `forceNewSession` are set to true
    public async createSession(_scopes: string[]): Promise<AuthenticationSession> {
        await this.ensureInitialized()
        // Get token from scret storage
        let token = await this.secretStorage.get(SourcegraphAuthProvider.secretKey)
        if (token) {
            console.log('Successfully logged in to Sourcegraph', token)
            await this.secretStorage.store(SourcegraphAuthProvider.secretKey, token)
        }
        if (!token) {
            token = ''
        }
        return new SourcegraphAuthSession(token)
    }

    // To sign out
    public async removeSession(_sessionId: string): Promise<void> {
        await this.secretStorage.delete(SourcegraphAuthProvider.secretKey)
        console.log('Successfully logged out of Sourcegraph')
    }
}

export class SourcegraphAuthActions {
    private currentEndpoint = endpointSetting()

    constructor(private readonly secretStorage: SecretStorage) {}

    public async login(newtoken: string, newuri: string): Promise<void> {
        try {
            await this.secretStorage.store(scretTokenKey, newtoken)
            if (this.currentEndpoint !== newuri) {
                await setEndpoint(newuri)
            }
            return
        } catch (error) {
            console.error(error)
        }
    }

    public async logout(): Promise<void> {
        await this.secretStorage.delete(scretTokenKey)
        await commands.executeCommand('workbench.action.reloadWindow')
        return
    }
}
