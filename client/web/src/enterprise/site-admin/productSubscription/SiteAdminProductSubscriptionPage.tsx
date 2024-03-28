import React, { useEffect } from 'react'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'

import { PageTitle } from '../../../components/PageTitle'

import { ProductSubscriptionStatus } from './ProductSubscriptionStatus'

interface SiteAdminProductSubscriptionPageProps extends TelemetryV2Props {}

/**
 * Displays the product subscription information from the license key in site configuration.
 */
export const SiteAdminProductSubscriptionPage: React.FunctionComponent<SiteAdminProductSubscriptionPageProps> = ({
    telemetryRecorder,
}) => {
    useEffect(() => telemetryRecorder.recordEvent('admin.productSubscription', 'view'), [telemetryRecorder])

    return (
        <div className="site-admin-product-subscription-page">
            <PageTitle title="Sourcegraph product subscription" />
            <ProductSubscriptionStatus telemetryRecorder={telemetryRecorder} />
        </div>
    )
}
