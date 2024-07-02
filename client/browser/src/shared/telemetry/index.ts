import { type Observable, Subscription } from 'rxjs'

import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import type { BillingCategory, BillingProduct } from '@sourcegraph/shared/src/telemetry'
import {
    type TelemetryRecorder as BaseTelemetryRecorder,
    TelemetryRecorderProvider as BaseTelemetryRecorderProvider,
    type KnownKeys,
    type KnownString,
    NoOpTelemetryExporter,
    type TelemetryEventParameters,
} from '@sourcegraph/telemetry'

import { getTelemetryClientName } from '../util/context'

import { GraphQLTelemetryExporter } from './gqlTelemetryExporter'

/**
 * TelemetryRecorderProvider is the default provider implementation for the
 * Sourcegraph web app.
 */
export class ConditionalTelemetryRecorderProvider extends BaseTelemetryRecorderProvider<
    BillingCategory,
    BillingProduct
> {
    constructor(private telemetryEnabled: Observable<boolean>, requestGraphQL: PlatformContext['requestGraphQL']) {
        super(
            {
                client: getTelemetryClientName(),
            },
            new GraphQLTelemetryExporter(requestGraphQL),
            [],
            {
                /**
                 * Disable buffering for now
                 */
                bufferTimeMs: 0,
                bufferMaxSize: 1,
                errorHandler: error => {
                    throw new Error(error)
                },
            }
        )
    }

    public getRecorder(): ConditionalTelemetryRecorder<BillingCategory, BillingProduct> {
        return new ConditionalTelemetryRecorder(this.telemetryEnabled, super.getRecorder())
    }
}

export class ConditionalTelemetryRecorder<BillingCategory extends string, BillingProduct extends string>
    implements BaseTelemetryRecorder<BillingCategory, BillingProduct>
{
    /** The enabled state set by an observable, provided upon instantiation */
    private isEnabled = false
    /** Log events are passed on to the inner TelemetryService */
    private subscription = new Subscription()

    constructor(
        telemetryEnabled: Observable<boolean>,
        private innerRecorder: BaseTelemetryRecorder<BillingCategory, BillingProduct>
    ) {
        this.subscription.add(
            telemetryEnabled.subscribe(enabled => {
                this.isEnabled = enabled
            })
        )
    }

    public recordEvent<Feature extends string, Action extends string, MetadataKey extends string>(
        feature: KnownString<Feature>,
        action: KnownString<Action>,
        parameters?:
            | TelemetryEventParameters<
                  KnownKeys<MetadataKey, { [key in MetadataKey]: number }>,
                  BillingCategory,
                  BillingProduct
              >
            | undefined
    ): void {
        setTimeout(() => {
            if (this.isEnabled) {
                this.innerRecorder.recordEvent(feature, action, parameters)
            }
        })
    }

    public unsubscribe(): void {
        this.isEnabled = false
        this.subscription.unsubscribe()
    }
}

export class NoOpTelemetryRecorderProvider extends BaseTelemetryRecorderProvider<BillingCategory, BillingProduct> {
    constructor() {
        super({ client: '' }, new NoOpTelemetryExporter(), [])
    }
}

export const noOptelemetryRecorderProvider = new NoOpTelemetryRecorderProvider()
export const noOpTelemetryRecorder = noOptelemetryRecorderProvider.getRecorder() as ConditionalTelemetryRecorder<
    BillingCategory,
    BillingProduct
>
