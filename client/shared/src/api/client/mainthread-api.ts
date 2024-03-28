import { type Remote, proxy } from 'comlink'
import { type Unsubscribable, Subscription, from, of, lastValueFrom, ReplaySubject } from 'rxjs'
import { share, switchMap } from 'rxjs/operators'

import { logger } from '@sourcegraph/common'

import { registerBuiltinClientCommands } from '../../commands/commands'
import type { PlatformContext } from '../../platform/context'
import { isSettingsValid } from '../../settings/settings'
import type { FlatExtensionHostAPI, MainThreadAPI } from '../contract'
import { proxySubscribable } from '../extension/api/common'

import { ProxySubscription } from './api/common'
import { updateSettings } from './services/settings'

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

/**
 * For state that needs to live in the main thread.
 * Returned to Controller for access by client applications.
 */
export interface ExposedToClient {
    registerCommand: (entryToRegister: CommandEntry) => Unsubscribable
    executeCommand: (parameters: ExecuteCommandParameters) => Promise<any>
}

export const initMainThreadAPI = (
    extensionHost: Remote<FlatExtensionHostAPI>,
    platformContext: Pick<
        PlatformContext,
        | 'updateSettings'
        | 'settings'
        | 'getGraphQLClient'
        | 'requestGraphQL'
        | 'getStaticExtensions'
        | 'telemetryService'
        | 'clientApplication'
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
    const registerCommand = ({ command, run }: CommandEntry): Unsubscribable => {
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

    subscription.add(registerBuiltinClientCommands(platformContext, extensionHost, registerCommand))

    const exposedToClient: ExposedToClient = {
        registerCommand,
        executeCommand,
    }

    const api: MainThreadAPI = {
        applySettingsEdit: edit => updateSettings(platformContext, edit),
        requestGraphQL: (request, variables) =>
            lastValueFrom(
                platformContext.requestGraphQL({
                    request,
                    variables,
                    mightContainPrivateInfo: true,
                })
            ),
        // Commands
        executeCommand: (command, args) => executeCommand({ command, args }),
        registerCommand: (command, run) => {
            const subscription = new Subscription()
            subscription.add(registerCommand({ command, run }))
            subscription.add(new ProxySubscription(run))
            return proxy(subscription)
        },

        getEnabledExtensions: () => {
            if (platformContext.getStaticExtensions) {
                return proxySubscribable(
                    platformContext.getStaticExtensions().pipe(
                        switchMap(staticExtensions =>
                            staticExtensions
                                ? of(staticExtensions).pipe(
                                      share({
                                          connector: () => new ReplaySubject(1),
                                          resetOnError: false,
                                          resetOnComplete: false,
                                          resetOnRefCountZero: false,
                                      })
                                  )
                                : of([])
                        )
                    )
                )
            }

            return proxySubscribable(of([]))
        },
        logEvent: (eventName, eventProperties) => platformContext.telemetryService?.log(eventName, eventProperties),
        logExtensionMessage: (...data) => logger.log(...data),
    }

    return { api, exposedToClient, subscription }
}
