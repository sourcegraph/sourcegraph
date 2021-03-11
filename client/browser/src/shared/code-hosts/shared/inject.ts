import { Observable, Subscription } from 'rxjs'
import { startWith } from 'rxjs/operators'
import { MutationRecordLike, observeMutations as defaultObserveMutations } from '../../util/dom'

import { determineCodeHost, CodeHost, injectCodeIntelligenceToCodeHost, ObserveMutations } from './codeHost'
import { SourcegraphIntegrationURLs } from '../../platform/context'

/**
 * Checks if the current page is a known code host. If it is,
 * injects features for the lifetime of the script in reaction to DOM mutations.
 *
 * @param isExtension `true` when executing in the browser extension.
 * @param onCodeHostFound setup logic to run when a code host is found (e.g. loading stylesheet) to avoid doing
 * such work on unsupported websites
 */
export async function injectCodeIntelligence(
    urls: SourcegraphIntegrationURLs,
    isExtension: boolean,
    onCodeHostFound?: (codeHost: CodeHost) => Promise<void>
): Promise<Subscription> {
    const subscriptions = new Subscription()
    const codeHost = determineCodeHost()
    if (codeHost) {
        console.log('Sourcegraph: Detected code host:', codeHost.type)

        if (onCodeHostFound) {
            await onCodeHostFound(codeHost)
        }

        const observeMutations: ObserveMutations = codeHost.observeMutations || defaultObserveMutations
        const mutations: Observable<MutationRecordLike[]> = observeMutations(document.body, {
            childList: true,
            subtree: true,
        }).pipe(startWith([{ addedNodes: [document.body], removedNodes: [] }]))
        subscriptions.add(injectCodeIntelligenceToCodeHost(mutations, codeHost, urls, isExtension))
    }
    return subscriptions
}
