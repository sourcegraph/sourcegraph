import { ProxyValue, proxyValue, proxyValueSymbol } from '@sourcegraph/comlink'
import { Subject } from 'rxjs'
import * as sourcegraph from 'sourcegraph'
import {
    MessageActionItem,
    MessageType,
    ShowInputParams,
    ShowMessageParams,
    ShowMessageRequestParams,
} from '../services/notifications'

/** @internal */
export interface ClientWindowsAPI extends ProxyValue {
    $showNotification(message: string): void
    $showMessage(message: string): Promise<void>
    $showInputBox(options?: sourcegraph.InputBoxOptions): Promise<string | undefined>
    $showProgress(options: sourcegraph.ProgressOptions): sourcegraph.ProgressReporter & ProxyValue
}

/** @internal */
export class ClientWindows implements ClientWindowsAPI {
    public readonly [proxyValueSymbol] = true

    constructor(
        /** Called when the client receives a window/showMessage notification. */
        private showMessage: (params: ShowMessageParams) => void,
        /**
         * Called when the client receives a window/showMessageRequest request and expected to return a promise
         * that resolves to the selected action.
         */
        private showMessageRequest: (params: ShowMessageRequestParams) => Promise<MessageActionItem | null>,
        /**
         * Called when the client receives a window/showInput request and expected to return a promise that
         * resolves to the user's input.
         */
        private showInput: (params: ShowInputParams) => Promise<string | null>,
        private createProgressReporter: (options: sourcegraph.ProgressOptions) => Subject<sourcegraph.Progress>
    ) {}

    public $showNotification(message: string): void {
        this.showMessage({ type: MessageType.Info, message })
    }

    public $showMessage(message: string): Promise<void> {
        return this.showMessageRequest({ type: MessageType.Info, message }).then(
            () =>
                // TODO(sqs): update the showInput API to unify null/undefined etc between the old internal API and the new
                // external API.
                undefined
        )
    }

    public $showInputBox(options?: sourcegraph.InputBoxOptions): Promise<string | undefined> {
        return this.showInput({
            message: options && options.prompt ? options.prompt : '',
            defaultValue: options && options.value,
        }).then(v =>
            // TODO(sqs): update the showInput API to unify null/undefined etc between the old internal API and the new
            // external API.
            v === null ? undefined : v
        )
    }

    public $showProgress(options: sourcegraph.ProgressOptions): sourcegraph.ProgressReporter & ProxyValue {
        return proxyValue(this.createProgressReporter(options))
    }
}
