import React, { useEffect } from 'react'

import classNames from 'classnames'
import { parseISO, differenceInDays } from 'date-fns'

import { renderMarkdown } from '@sourcegraph/common'
import { gql, useQuery } from '@sourcegraph/http-client'
import { useSettings } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Link, Markdown } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { DismissibleAlert } from '../components/DismissibleAlert'
import { useFeatureFlag } from '../featureFlags/useFeatureFlag'
import type { GlobalAlertsSiteFlagsResult, GlobalAlertsSiteFlagsVariables } from '../graphql-operations'
import { FreeUsersExceededAlert } from '../site/FreeUsersExceededAlert'
import { LicenseExpirationAlert } from '../site/LicenseExpirationAlert'
import { NeedsRepositoryConfigurationAlert } from '../site/NeedsRepositoryConfigurationAlert'
import { siteFlagFieldsFragment } from '../storm/pages/LayoutPage/LayoutPage.loader'

import { GlobalAlert } from './GlobalAlert'
import { Notices, VerifyEmailNotices } from './Notices'

import styles from './GlobalAlerts.module.scss'

interface Props extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
}

// NOTE: The name of the query is also added in the refreshSiteFlags() function
// found in client/web/src/site/backend.tsx
const QUERY = gql`
    query GlobalAlertsSiteFlags {
        site {
            ...SiteFlagFields
        }
    }

    ${siteFlagFieldsFragment}
`
/**
 * Alerts that should not be visible when admin onboarding is enabled
 */
const adminOnboardingRemovedAlerts = ['externalURL', 'email.smtp', 'enable repository permissions']

/**
 * Fetches and displays relevant global alerts at the top of the page
 */
export const GlobalAlerts: React.FunctionComponent<Props> = ({ authenticatedUser, telemetryRecorder }) => {
    const settings = useSettings()
    const [isAdminOnboardingEnabled] = useFeatureFlag('admin-onboarding', true)
    const { data } = useQuery<GlobalAlertsSiteFlagsResult, GlobalAlertsSiteFlagsVariables>(QUERY, {
        fetchPolicy: 'cache-and-network',
    })

    useEffect(() => {
        if (settings?.motd && Array.isArray(settings.motd)) {
            telemetryRecorder.recordEvent('alert.motd', 'view')
        }
        if (process.env.SOURCEGRAPH_API_URL) {
            telemetryRecorder.recordEvent('alert.proxyAPI', 'view')
        }
    }, [settings?.motd, telemetryRecorder])

    const siteFlagsValue = data?.site
    let alerts = siteFlagsValue?.alerts ?? []

    if (isAdminOnboardingEnabled) {
        alerts =
            siteFlagsValue?.alerts.filter(
                ({ message }) => !adminOnboardingRemovedAlerts.some(alt => message.includes(alt))
            ) ?? []
    }

    return (
        <div className={classNames('test-global-alert', styles.globalAlerts)}>
            {siteFlagsValue && (
                <>
                    {siteFlagsValue?.needsRepositoryConfiguration && (
                        <NeedsRepositoryConfigurationAlert
                            className={styles.alert}
                            telemetryRecorder={telemetryRecorder}
                        />
                    )}
                    {siteFlagsValue.freeUsersExceeded && (
                        <FreeUsersExceededAlert
                            noLicenseWarningUserCount={siteFlagsValue.productSubscription.noLicenseWarningUserCount}
                            className={styles.alert}
                            telemetryRecorder={telemetryRecorder}
                        />
                    )}
                    {alerts.map((alert, index) => (
                        <GlobalAlert
                            key={index}
                            alert={alert}
                            className={styles.alert}
                            telemetryRecorder={telemetryRecorder}
                        />
                    ))}
                    {siteFlagsValue.productSubscription.license &&
                        (() => {
                            const expiresAt = parseISO(siteFlagsValue.productSubscription.license.expiresAt)
                            return (
                                differenceInDays(expiresAt, Date.now()) <= 7 && (
                                    <LicenseExpirationAlert
                                        expiresAt={expiresAt}
                                        daysLeft={Math.floor(differenceInDays(expiresAt, Date.now()))}
                                        className={styles.alert}
                                        telemetryRecorder={telemetryRecorder}
                                    />
                                )
                            )
                        })()}
                </>
            )}
            {settings?.motd &&
                Array.isArray(settings.motd) &&
                settings.motd.map(motd => (
                    <DismissibleAlert
                        key={motd}
                        partialStorageKey={`motd.${motd}`}
                        variant="info"
                        className={styles.alert}
                    >
                        <Markdown dangerousInnerHTML={renderMarkdown(motd)} />
                    </DismissibleAlert>
                ))}
            {process.env.SOURCEGRAPH_API_URL && (
                <DismissibleAlert
                    key="dev-web-server-alert"
                    partialStorageKey="dev-web-server-alert"
                    variant="danger"
                    className={styles.alert}
                >
                    <div>
                        <strong>Warning!</strong> This build uses data from the proxied API:{' '}
                        <Link className={styles.proxyLink} target="__blank" to={process.env.SOURCEGRAPH_API_URL}>
                            {process.env.SOURCEGRAPH_API_URL}
                        </Link>
                    </div>
                    .
                </DismissibleAlert>
            )}
            <Notices alertClassName={styles.alert} location="top" telemetryRecorder={telemetryRecorder} />
            <VerifyEmailNotices
                authenticatedUser={authenticatedUser}
                alertClassName={styles.alert}
                telemetryRecorder={telemetryRecorder}
            />
        </div>
    )
}
