import { ProxyResult, ProxyValue, proxyValue, proxyValueSymbol } from '@sourcegraph/comlink'
import { Hover, Location } from '@sourcegraph/extension-api-types'
import { DocumentSelector, TokenType, Unsubscribable } from 'sourcegraph'
import { ProxySubscribable } from '../../extension/api/common'
import { ReferenceParams, TextDocumentPositionParams, TextDocumentRegistrationOptions } from '../../protocol'
import { ProvideTextDocumentHoverSignature } from '../services/hover'
import { TextDocumentLocationProviderIDRegistry, TextDocumentLocationProviderRegistry } from '../services/location'
import { FeatureProviderRegistry } from '../services/registry'
import { TextDocumentTokenTypeProviderRegistry } from '../services/tokenType'
import { wrapRemoteObservable } from './common'

/** @internal */
export interface ClientLanguageFeaturesAPI extends ProxyValue {
    $registerHoverProvider(
        selector: DocumentSelector,
        providerFunction: ProxyResult<
            ((params: TextDocumentPositionParams) => ProxySubscribable<Hover | null | undefined>) & ProxyValue
        >
    ): Unsubscribable & ProxyValue
    $registerDefinitionProvider(
        selector: DocumentSelector,
        providerFunction: ProxyResult<
            ((params: TextDocumentPositionParams) => ProxySubscribable<Location[]>) & ProxyValue
        >
    ): Unsubscribable & ProxyValue
    $registerReferenceProvider(
        selector: DocumentSelector,
        providerFunction: ProxyResult<((params: ReferenceParams) => ProxySubscribable<Location[]>) & ProxyValue>
    ): Unsubscribable & ProxyValue

    /**
     * @param idStr The `id` argument in the extension's {@link sourcegraph.languages.registerLocationProvider}
     * call.
     */
    $registerLocationProvider(
        idStr: string,
        selector: DocumentSelector,
        providerFunction: ProxyResult<
            ((params: TextDocumentPositionParams) => ProxySubscribable<Location[]>) & ProxyValue
        >
    ): Unsubscribable & ProxyValue

    $registerTokenTypeProvider(
        selector: DocumentSelector,
        providerFunction: ProxyResult<((params: TextDocumentPositionParams) => TokenType) & ProxyValue>
    ): Unsubscribable & ProxyValue
}

/** @internal */
export class ClientLanguageFeatures implements ClientLanguageFeaturesAPI, ProxyValue {
    public readonly [proxyValueSymbol] = true

    constructor(
        private hoverRegistry: FeatureProviderRegistry<
            TextDocumentRegistrationOptions,
            ProvideTextDocumentHoverSignature
        >,
        private definitionRegistry: TextDocumentLocationProviderRegistry,
        private referencesRegistry: TextDocumentLocationProviderRegistry<ReferenceParams>,
        private locationRegistry: TextDocumentLocationProviderIDRegistry,
        private tokenTypeRegistry: TextDocumentTokenTypeProviderRegistry
    ) {}

    public $registerHoverProvider(
        documentSelector: DocumentSelector,
        providerFunction: ProxyResult<
            ((params: TextDocumentPositionParams) => ProxySubscribable<Hover | null | undefined>) & ProxyValue
        >
    ): Unsubscribable & ProxyValue {
        return proxyValue(
            this.hoverRegistry.registerProvider({ documentSelector }, params =>
                wrapRemoteObservable(providerFunction(params))
            )
        )
    }

    public $registerDefinitionProvider(
        documentSelector: DocumentSelector,
        providerFunction: ProxyResult<
            ((params: TextDocumentPositionParams) => ProxySubscribable<Location[]>) & ProxyValue
        >
    ): Unsubscribable & ProxyValue {
        return proxyValue(
            this.definitionRegistry.registerProvider({ documentSelector }, params =>
                wrapRemoteObservable(providerFunction(params))
            )
        )
    }

    public $registerReferenceProvider(
        documentSelector: DocumentSelector,
        providerFunction: ProxyResult<
            ((params: TextDocumentPositionParams) => ProxySubscribable<Location[]>) & ProxyValue
        >
    ): Unsubscribable & ProxyValue {
        return proxyValue(
            this.referencesRegistry.registerProvider({ documentSelector }, params =>
                wrapRemoteObservable(providerFunction(params))
            )
        )
    }

    public $registerLocationProvider(
        id: string,
        documentSelector: DocumentSelector,
        providerFunction: ProxyResult<
            ((params: TextDocumentPositionParams) => ProxySubscribable<Location[]>) & ProxyValue
        >
    ): Unsubscribable & ProxyValue {
        return proxyValue(
            this.locationRegistry.registerProvider({ id, documentSelector }, params =>
                wrapRemoteObservable(providerFunction(params))
            )
        )
    }

    public $registerTokenTypeProvider(
        documentSelector: DocumentSelector,
        providerFunction: ProxyResult<((params: TextDocumentPositionParams) => TokenType) & ProxyValue>
    ): Unsubscribable & ProxyValue {
        return proxyValue(
            this.tokenTypeRegistry.registerProvider({ documentSelector }, params => providerFunction(params))
        )
    }
}
