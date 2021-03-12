import { Remote, proxy } from 'comlink'
import { updateSettings } from './services/settings'
import { Subscription, from, Observable, of, combineLatest, Subject } from 'rxjs'
import { PlatformContext } from '../../platform/context'
import { isSettingsValid } from '../../settings/settings'
import { catchError, distinctUntilChanged, map, publishReplay, refCount, switchMap } from 'rxjs/operators'
import { FlatExtensionHostAPI, MainThreadAPI, NotificationType, PlainNotification } from '../contract'
import { ProxySubscription } from './api/common'
import * as sourcegraph from 'sourcegraph'
import { proxySubscribable } from '../extension/api/common'
import { ConfiguredExtension, isExtensionEnabled } from '../../extensions/extension'
import { viewerConfiguredExtensions } from '../../extensions/helpers'
import { asError, isErrorLike } from '../../util/errors'
import { fromFetch } from 'rxjs/fetch'
import { ExtensionManifest } from '../../extensions/extensionManifest'
import { checkOk } from '../../backend/fetch'
import { areExtensionsSame } from '../../extensions/extensions'
import { registerBuiltinClientCommands } from '../../commands/commands'

/** A registered command in the command registry. */
export interface CommandEntry {
    /** The command ID (conventionally, e.g., "myextension.mycommand"). */
    command: string

    /** The function called to run the command and return an async value. */
    run: (...args: any[]) => Promise<any>
}

export interface ExecuteCommandParameters {
    /**
     * The identifier of the actual command handler.
     */
    command: string

    /**
     * Arguments that the command should be invoked with.
     */
    args?: any[]
}

function messageFromExtension(message: string): string {
    return `From extension:\n\n${message}`
}

/**
 * For state that needs to live in the main thread.
 * Returned to Controller for access by client applications.
 */
export interface ExposedToClient {
    registerCommand: (entryToRegister: CommandEntry) => sourcegraph.Unsubscribable
    executeCommand: (parameters: ExecuteCommandParameters, suppressNotificationOnError?: boolean) => Promise<any>

    /**
     * Observable of error notifications as a result of client applications executing commands.
     */
    commandErrors: Observable<PlainNotification>
}

export const initMainThreadAPI = (
    extensionHost: Remote<FlatExtensionHostAPI>,
    platformContext: Pick<
        PlatformContext,
        | 'updateSettings'
        | 'settings'
        | 'requestGraphQL'
        | 'showMessage'
        | 'showInputBox'
        | 'sideloadedExtensionURL'
        | 'getScriptURLForExtension'
        | 'getStaticExtensions'
    >
): { api: MainThreadAPI; exposedToClient: ExposedToClient; subscription: Subscription } => {
    const subscription = new Subscription()

    subscription.add(
        from(platformContext.settings)
            .pipe(
                switchMap(settings => {
                    if (isSettingsValid(settings)) {
                        return extensionHost.syncSettingsData(settings)
                    }
                    return []
                })
            )
            .subscribe()
    )

    // Commands
    const commands = new Map<string, CommandEntry>()
    const registerCommand = ({ command, run }: CommandEntry): sourcegraph.Unsubscribable => {
        if (commands.has(command)) {
            throw new Error(`command is already registered: ${JSON.stringify(command)}`)
        }

        commands.set(command, { command, run })
        return {
            unsubscribe: () => commands.delete(command),
        }
    }
    const executeCommand = ({ args, command }: ExecuteCommandParameters): Promise<any> => {
        const commandEntry = commands.get(command)
        if (!commandEntry) {
            throw new Error(`command not found: ${JSON.stringify(command)}`)
        }
        return Promise.resolve(commandEntry.run(...(args || [])))
    }

    subscription.add(registerBuiltinClientCommands(platformContext, registerCommand))

    const commandErrors = new Subject<PlainNotification>()
    const exposedToClient: ExposedToClient = {
        registerCommand,
        executeCommand: (parameters, suppressNotificationOnError) =>
            executeCommand(parameters).catch(error => {
                if (!suppressNotificationOnError) {
                    commandErrors.next({
                        message: asError(error).message,
                        type: NotificationType.Error,
                        source: parameters.command,
                    })
                }
                return Promise.reject(error)
            }),
        commandErrors,
    }

    const api: MainThreadAPI = {
        applySettingsEdit: edit => updateSettings(platformContext, edit),
        requestGraphQL: (request, variables) =>
            platformContext
                .requestGraphQL({
                    request,
                    variables,
                    mightContainPrivateInfo: true,
                })
                .toPromise(),
        // Commands
        executeCommand: (command, args) => executeCommand({ command, args }),
        registerCommand: (command, run) => {
            const subscription = new Subscription()
            subscription.add(registerCommand({ command, run }))
            subscription.add(new ProxySubscription(run))
            return proxy(subscription)
        },
        // User interaction methods
        showMessage: message =>
            platformContext.showMessage ? platformContext.showMessage(message) : defaultShowMessage(message),
        showInputBox: options =>
            platformContext.showInputBox ? platformContext.showInputBox(options) : defaultShowInputBox(options),

        getSideloadedExtensionURL: () => proxySubscribable(platformContext.sideloadedExtensionURL),
        getScriptURLForExtension: () => {
            const getScriptURL = platformContext.getScriptURLForExtension()
            if (!getScriptURL) {
                return undefined
            }

            return proxy(getScriptURL)
        },
        getEnabledExtensions: () => {
            const staticExtensions = platformContext.getStaticExtensions?.()
            if (staticExtensions) {
                // Ensure that the observable never completes while subscribed to
                return proxySubscribable(from(staticExtensions).pipe(publishReplay(1), refCount()))
            }

            return proxySubscribable(getEnabledExtensions(platformContext))
        },
    }

    return { api, exposedToClient, subscription }
}

function defaultShowMessage(message: string): Promise<void> {
    return new Promise<void>(resolve => {
        alert(messageFromExtension(message))
        resolve()
    })
}

function defaultShowInputBox(options?: sourcegraph.InputBoxOptions): Promise<string | undefined> {
    return new Promise<string | undefined>(resolve => {
        const response = prompt(messageFromExtension(options?.prompt ?? ''), options?.value)
        resolve(response ?? undefined)
    })
}

/**
 * The manifest of an extension sideloaded during local development.
 *
 * Doesn't include {@link ExtensionManifest#url}, as this is added when
 * publishing an extension to the registry.
 * Instead, the bundle URL is computed from the manifest's `main` field.
 */
interface SideloadedExtensionManifest extends Omit<ExtensionManifest, 'url'> {
    name: string
    main: string
}

const getConfiguredSideloadedExtension = (baseUrl: string): Observable<ConfiguredExtension> =>
    fromFetch(`${baseUrl}/package.json`, { selector: response => checkOk(response).json() }).pipe(
        map(
            (response: SideloadedExtensionManifest): ConfiguredExtension => ({
                id: response.name,
                manifest: {
                    ...response,
                    url: `${baseUrl}/${response.main.replace('dist/', '')}`,
                },
                rawManifest: null,
            })
        )
    )

function getEnabledExtensions(
    context: Pick<
        PlatformContext,
        'settings' | 'requestGraphQL' | 'sideloadedExtensionURL' | 'getScriptURLForExtension'
    >
): Observable<ConfiguredExtension[]> {
    const sideloadedExtension: Observable<ConfiguredExtension | null> = from(context.sideloadedExtensionURL).pipe(
        switchMap(url => (url ? getConfiguredSideloadedExtension(url) : of(null))),
        catchError(error => {
            console.error('Error sideloading extension', error)
            return of(null)
        })
    )

    return combineLatest([viewerConfiguredExtensions(context), sideloadedExtension, context.settings]).pipe(
        map(([configuredExtensions, sideloadedExtension, settings]) => {
            let enabled = configuredExtensions.filter(extension => isExtensionEnabled(settings.final, extension.id))
            if (sideloadedExtension) {
                if (!isErrorLike(sideloadedExtension.manifest) && sideloadedExtension.manifest?.publisher) {
                    // Disable extension with the same ID while this extension is sideloaded
                    const constructedID = `${sideloadedExtension.manifest.publisher}/${sideloadedExtension.id}`
                    enabled = enabled.filter(extension => extension.id !== constructedID)
                }

                enabled.push(sideloadedExtension)
            }
            return enabled
        }),
        distinctUntilChanged((a, b) => areExtensionsSame(a, b)),
        publishReplay(1),
        refCount()
    )
}
