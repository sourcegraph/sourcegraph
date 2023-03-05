import { Remote } from 'comlink'
import { BehaviorSubject, combineLatest, from, Observable, of, Subscription } from 'rxjs'
import { catchError, concatMap, distinctUntilChanged, first, map, switchMap, tap } from 'rxjs/operators'
import sourcegraph from 'sourcegraph'

import { asError, isErrorLike, hashCode, logger } from '@sourcegraph/common'

import { ConfiguredExtension, getScriptURLFromExtensionManifest, splitExtensionID } from '../../extensions/extension'
import { areExtensionsSame, getEnabledExtensionsForSubject } from '../../extensions/extensions'
import { wrapRemoteObservable } from '../client/api/common'
import { MainThreadAPI } from '../contract'
import { tryCatchPromise } from '../util'

import { ExtensionHostState } from './extensionHostState'

export function observeActiveExtensions(
    mainAPI: Remote<MainThreadAPI>,
    mainThreadAPIInitializations: Observable<boolean>
): {
    activeLanguages: ExtensionHostState['activeLanguages']
    activeExtensions: ExtensionHostState['activeExtensions']
} {
    const activeLanguages = new BehaviorSubject<ReadonlySet<string>>(new Set())
    // Wait until the main thread API has initialized since this runs during extension host init.
    const enabledExtensions = mainThreadAPIInitializations.pipe(
        first(initialized => initialized),
        switchMap(() => wrapRemoteObservable(mainAPI.getEnabledExtensions()))
    )
    const activatedExtensionIDs = new Set<string>()

    const activeExtensions: Observable<(ConfiguredExtension | ExecutableExtension)[]> = combineLatest([
        activeLanguages,
        enabledExtensions,
    ]).pipe(
        tap(([activeLanguages, enabledExtensions]) => {
            const activeExtensions = extensionsWithMatchedActivationEvent(enabledExtensions, activeLanguages)
            for (const extension of activeExtensions) {
                if (!activatedExtensionIDs.has(extension.id)) {
                    activatedExtensionIDs.add(extension.id)
                }
            }
        }),
        map(([, extensions]) =>
            extensions ? extensions.filter(extension => activatedExtensionIDs.has(extension.id)) : []
        ),
        distinctUntilChanged((a, b) => areExtensionsSame(a, b))
    )

    return {
        activeLanguages,
        activeExtensions,
    }
}

/**
 * List of insight-like extension ids. These insights worked via extensions before,
 * but at the moment they work via insight built-in data-fetchers.
 */
const DEPRECATED_EXTENSION_IDS = new Set(['sourcegraph/code-stats-insights', 'sourcegraph/search-insights'])

export function activateExtensions(
    state: Pick<ExtensionHostState, 'activeExtensions' | 'haveInitialExtensionsLoaded' | 'settings'>,
    mainAPI: Remote<Pick<MainThreadAPI, 'logEvent'>>,
    createExtensionAPI: (extensionID: string) => typeof sourcegraph,
    mainThreadAPIInitializations: Observable<boolean>,
    /**
     * Function that activates an extension.
     * Returns a promise that resolves once the extension is activated.
     * */
    activate = activateExtension,
    /**
     * Function that de-activates an extension.
     * Returns a promise that resolves once the extension is de-activated.
     * */
    deactivate = deactivateExtension
): Subscription {
    const previouslyActivatedExtensions = new Set<string>()
    const extensionsSubscription = combineLatest([state.activeExtensions])
        .pipe(
            concatMap(([activeExtensions]) => {
                const toDeactivate = new Set<string>()
                const toActivate = new Map<string, ConfiguredExtension | ExecutableExtension>()
                const activeExtensionIDs = new Set<string>()

                for (const extension of activeExtensions) {
                    // Ignore extensions that now work via built-in insights fetchers
                    if (DEPRECATED_EXTENSION_IDS.has(extension.id)) {
                        continue
                    }

                    // Populate set of currently active extension IDs
                    activeExtensionIDs.add(extension.id)
                    // Populate map of extensions to activate
                    if (!previouslyActivatedExtensions.has(extension.id)) {
                        toActivate.set(extension.id, extension)
                    }
                }

                for (const id of previouslyActivatedExtensions) {
                    // Populate map of extensions to deactivate
                    if (!activeExtensionIDs.has(id)) {
                        toDeactivate.add(id)
                    }
                }

                const scriptURLs = [...toActivate.values()].map(extension => {
                    if ('scriptURL' in extension) {
                        // This is already an executable extension (inline extension)
                        return extension.scriptURL
                    }

                    return getScriptURLFromExtensionManifest(extension)
                })

                const executableExtensionsToActivate: ExecutableExtension[] = [...toActivate.values()]
                    .map((extension, index) => ({
                        id: extension.id,
                        manifest: extension.manifest,
                        scriptURL: scriptURLs[index],
                    }))
                    .filter((extension): extension is ExecutableExtension => typeof extension.scriptURL === 'string')

                return of({ toActivate: executableExtensionsToActivate, toDeactivate }).pipe(
                    map(({ toActivate, toDeactivate }) => {
                        // We could log the event after the activation promise resolves to ensure that there wasn't
                        // an error during activation, but we want to track the maximum number of times an extension could have been useful.
                        // Since extension activation is passive from the user's perspective, and we don't yet track extension usage events,
                        // there's no way that we could measure how often extensions are actually useful anyways.
                        const defaultExtensions =
                            getEnabledExtensionsForSubject(state.settings.value, 'DefaultSettings') || {}

                        return from(
                            Promise.all([
                                toActivate.map(async ({ id, scriptURL }) => {
                                    // We only want to log non-default extension events
                                    if (!defaultExtensions[id]) {
                                        // Hash extension IDs that specify host, since that means that it's a private registry extension.
                                        try {
                                            const telemetryExtensionID = splitExtensionID(id).host
                                                ? await hashCode(id)
                                                : id
                                            mainAPI
                                                .logEvent('ExtensionActivation', {
                                                    extension_id: telemetryExtensionID,
                                                })
                                                .catch(() => {
                                                    // noop
                                                })
                                        } catch (error) {
                                            logger.error(
                                                `Fail to log ExtensionActivation event for extension ${id}:`,
                                                asError(error)
                                            )
                                        }
                                    }
                                    logger.log(`Activating Sourcegraph extension: ${id}`)
                                    return activate(id, scriptURL, createExtensionAPI).catch(error =>
                                        logger.error(`Error activating extension ${id}:`, asError(error))
                                    )
                                }),
                                [...toDeactivate].map(id =>
                                    deactivate(id).catch(error =>
                                        logger.error(`Error deactivating extension ${id}:`, asError(error))
                                    )
                                ),
                            ])
                        )
                    }),
                    map(() => ({ activated: toActivate, deactivated: toDeactivate })),
                    catchError(error => {
                        logger.error('Uncaught error during extension activation', error)
                        return []
                    })
                )
            })
        )
        .subscribe(({ activated, deactivated }) => {
            for (const id of deactivated) {
                previouslyActivatedExtensions.delete(id)
            }

            for (const [id] of activated) {
                previouslyActivatedExtensions.add(id)
            }

            if (state.haveInitialExtensionsLoaded.value === false) {
                state.haveInitialExtensionsLoaded.next(true)
            }
        })

    return extensionsSubscription
}

/** The WebWorker's global scope */
declare const self: any

/** Extensions' deactivate functions. */
const extensionDeactivates = new Map<string, () => void | Promise<void>>()

async function activateExtension(
    extensionID: string,
    bundleURL: string,
    createExtensionAPI: (extensionID: string) => typeof sourcegraph
): Promise<void> {
    // Load the extension bundle and retrieve the extension entrypoint module's exports on
    // the global `module` property.
    try {
        const extensionAPI = createExtensionAPI(extensionID)
        // Make `import 'sourcegraph'` or `require('sourcegraph')` return the extension API.
        replaceAPIRequire(extensionAPI)

        const exports = {}
        self.exports = exports
        self.module = { exports }
        self.importScripts(bundleURL)
    } catch (error) {
        throw new Error(
            `error thrown while executing extension ${JSON.stringify(
                extensionID
            )} bundle (in importScripts of ${bundleURL}): ${String(error)}`
        )
    }
    const extensionExports = self.module.exports
    delete self.exports
    delete self.module

    if (!('activate' in extensionExports)) {
        throw new Error(
            `extension bundle for ${JSON.stringify(extensionID)} has no exported activate function (in ${bundleURL})`
        )
    }

    // During extension deactivation, we first call the extension's `deactivate` function and then unsubscribe
    // the Subscription passed to the `activate` function.
    const extensionSubscriptions = new Subscription()
    const extensionDeactivate = typeof extensionExports.deactivate === 'function' ? extensionExports.deactivate : null
    extensionDeactivates.set(extensionID, async () => {
        try {
            if (extensionDeactivate) {
                await Promise.resolve(extensionDeactivate())
            }
        } finally {
            extensionSubscriptions.unsubscribe()
        }
    })

    // The behavior should be consistent for both sync and async activate functions that throw
    // errors or reject. Both cases should yield a rejected promise.
    //
    // TODO(sqs): Add timeouts to prevent long-running activate or deactivate functions from
    // significantly delaying other extensions.
    await tryCatchPromise<void>(() => extensionExports.activate({ subscriptions: extensionSubscriptions })).catch(
        error => {
            error = asError(error)
            throw Object.assign(
                new Error(
                    `error during extension ${JSON.stringify(extensionID)} activate function: ${String(
                        error.stack || error
                    )}`
                ),
                { error }
            )
        }
    )
}

async function deactivateExtension(extensionID: string): Promise<void> {
    const deactivate = extensionDeactivates.get(extensionID)
    if (deactivate) {
        extensionDeactivates.delete(extensionID)
        await Promise.resolve(deactivate())
    }
}

export function extensionsWithMatchedActivationEvent<Extension extends ConfiguredExtension | ExecutableExtension>(
    enabledExtensions: Extension[],
    visibleTextDocumentLanguages: ReadonlySet<string>
): Extension[] {
    const languageActivationEvents = new Set(
        [...visibleTextDocumentLanguages].map(language => `onLanguage:${language}`)
    )
    return enabledExtensions.filter(extension => {
        try {
            if (!extension.manifest) {
                const match = /^sourcegraph\/lang-(.*)$/.exec(extension.id)
                if (match) {
                    logger.warn(
                        `Extension ${extension.id} has been renamed to sourcegraph/${match[1]}. It's safe to remove ${extension.id} from your settings.`
                    )
                } else {
                    logger.warn(
                        `Extension ${extension.id} was not found. Remove it from settings to suppress this warning.`
                    )
                }
                return false
            }
            if (isErrorLike(extension.manifest)) {
                logger.warn(extension.manifest)
                return false
            }
            if (!extension.manifest.activationEvents) {
                logger.warn(`Extension ${extension.id} has no activation events, so it will never be activated.`)
                return false
            }
            return extension.manifest.activationEvents.some(
                event => event === '*' || languageActivationEvents.has(event)
            )
        } catch (error) {
            logger.error(error)
        }
        return false
    })
}

/**
 * The information about an extension necessary to execute and activate it.
 */
export interface ExecutableExtension extends Pick<ConfiguredExtension, 'id' | 'manifest'> {
    /** The URL to the JavaScript bundle of the extension. */
    scriptURL: string
}

/**
 * Make `import 'sourcegraph'` or `require('sourcegraph')` return the extension API.
 *
 * Because `require` is replaced on each extension activation with the API created for that extension,
 * the API can only be imported once to prevent extensions importing APIs created for other extensions.
 *
 * @param extensionAPI The extension API instance for the extension to be activated.
 * @throws error to give extension authors feedback if they try to import an API instance that was
 * already imported (e.g. if they asynchronously import the extension API and the current `require` was
 * called by another extension)
 */
export function replaceAPIRequire(extensionAPI: typeof sourcegraph): void {
    let alreadyImported = false

    globalThis.require = ((modulePath: string): any => {
        if (modulePath === 'sourcegraph') {
            if (!alreadyImported) {
                alreadyImported = true
                return extensionAPI
            }

            throw new Error('require: Sourcegraph extension API can only be imported once')
        }
        // All other requires/imports in the extension's code should not reach here because their JS
        // bundler should have resolved them locally.
        throw new Error(`require: module not found: ${modulePath}`)
    }) as any
}
