import React, { useEffect } from 'react'

import { mdiPlus } from '@mdi/js'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Button, Container, Icon, Link, PageHeader } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../../auth'
import { FilteredConnection } from '../../../../components/FilteredConnection'
import { PageTitle } from '../../../../components/PageTitle'
import type { SiteAdminProductSubscriptionFields } from '../../../../graphql-operations'

import { queryProductSubscriptions } from './backend'
import {
    SiteAdminProductSubscriptionNode,
    SiteAdminProductSubscriptionNodeHeader,
    type SiteAdminProductSubscriptionNodeProps,
} from './SiteAdminProductSubscriptionNode'

import styles from './SiteAdminCreateProductSubscriptionPage.module.scss'

interface Props extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser
}

/**
 * Displays the enterprise subscriptions (formerly known as "product subscriptions") that have been
 * created on Sourcegraph.com.
 */
export const SiteAdminProductSubscriptionsPage: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    authenticatedUser,
    telemetryRecorder,
}) => {
    useEffect(() => telemetryRecorder.recordEvent('admin.productSubscriptions', 'view'), [telemetryRecorder])

    return (
        <div className="site-admin-product-subscriptions-page">
            <PageTitle title="Enterprise subscriptions" />
            <PageHeader
                headingElement="h2"
                path={[{ text: 'Enterprise subscriptions' }]}
                actions={
                    <Button to="./new" variant="primary" as={Link}>
                        <Icon aria-hidden={true} svgPath={mdiPlus} />
                        Create Enterprise subscription
                    </Button>
                }
                className="mb-3"
            />

            <Container>
                <FilteredConnection<SiteAdminProductSubscriptionFields, SiteAdminProductSubscriptionNodeProps>
                    listComponent="table"
                    listClassName="table"
                    contentWrapperComponent={ListContentWrapper}
                    noun="Enterprise subscription"
                    pluralNoun="Enterprise subscriptions"
                    queryConnection={queryProductSubscriptions}
                    headComponent={SiteAdminProductSubscriptionNodeHeader}
                    nodeComponent={SiteAdminProductSubscriptionNode}
                />
            </Container>
        </div>
    )
}

const ListContentWrapper: React.FunctionComponent<React.PropsWithChildren<{}>> = ({ children }) => (
    <div className={styles.contentWrapper}>{children}</div>
)
