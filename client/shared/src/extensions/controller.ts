import { from, Observable, Subscription, Unsubscribable } from 'rxjs'
import { InitData } from '../api/extension/extensionHost'
import { PlatformContext } from '../platform/context'
import { createExtensionHostClientConnection } from '../api/client/connection'
import { Remote } from 'comlink'
import { FlatExtensionHostAPI, PlainNotification } from '../api/contract'
import { CommandEntry, ExecuteCommandParameters } from '../api/client/mainthread-api'
import { switchMap } from 'rxjs/operators'
import { syncPromiseSubscription } from '../api/util'

export interface Controller extends Unsubscribable {
    /**
     * Executes the command (registered in the CommandRegistry) specified in params. If an error is thrown, the
     * error is returned *and* emitted on the {@link Controller#notifications} observable.
     *
     * All callers should execute commands using this method instead of calling
     * {@link sourcegraph:CommandRegistry#executeCommand} directly (to ensure errors are emitted as notifications).
     *
     * @param suppressNotificationOnError By default, if command execution throws (or rejects with) an error, the
     * error will be shown in the global notification UI component. Pass suppressNotificationOnError as true to
     * skip this. The error is always returned to the caller.
     */
    executeCommand(parameters: ExecuteCommandParameters, suppressNotificationOnError?: boolean): Promise<any>

    registerCommand(entryToRegister: CommandEntry): Unsubscribable

    commandErrors: Observable<PlainNotification>

    /**
     * Frees all resources associated with this client.
     */
    unsubscribe(): void

    extHostAPI: Promise<Remote<FlatExtensionHostAPI>>
}

/**
 * React props or state containing the client. There should be only a single client for the whole
 * application.
 */
export interface ExtensionsControllerProps<K extends keyof Controller = keyof Controller> {
    /**
     * The client, which is used to communicate with and manage extensions.
     */
    extensionsController: Pick<Controller, K>
}

/**
 * Creates the controller, which handles all communication between the client application and extensions.
 *
 * There should only be a single controller for the entire client application. The controller's model represents
 * all of the client application state that the client needs to know.
 */
export function createController(context: PlatformContext): Controller {
    const subscriptions = new Subscription()

    const initData: Omit<InitData, 'initialSettings'> = {
        sourcegraphURL: context.sourcegraphURL,
        clientApplication: context.clientApplication,
    }

    const extensionHostClientPromise = createExtensionHostClientConnection(
        context.createExtensionHost(),
        initData,
        context
    )

    subscriptions.add(() => extensionHostClientPromise.then(({ subscription }) => subscription.unsubscribe()))

    // TODO: Debug helpers, logging

    return {
        executeCommand: (parameters, suppressNotificationOnError) =>
            extensionHostClientPromise.then(({ exposedToClient }) =>
                exposedToClient.executeCommand(parameters, suppressNotificationOnError)
            ),
        commandErrors: from(extensionHostClientPromise).pipe(
            switchMap(({ exposedToClient }) => exposedToClient.commandErrors)
        ),
        registerCommand: entryToRegister =>
            syncPromiseSubscription(
                extensionHostClientPromise.then(({ exposedToClient }) =>
                    exposedToClient.registerCommand(entryToRegister)
                )
            ),
        extHostAPI: extensionHostClientPromise.then(({ api }) => api),
        unsubscribe: () => subscriptions.unsubscribe(),
    }
}
