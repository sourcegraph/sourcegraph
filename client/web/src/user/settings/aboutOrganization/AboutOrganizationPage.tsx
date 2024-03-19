import React, { useEffect } from 'react'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { PageHeader, Text } from '@sourcegraph/wildcard'

import { PageTitle } from '../../../components/PageTitle'
import { SelfHostedCta } from '../../../components/SelfHostedCta'

import styles from './AboutOrganizationPage.module.scss'

interface AboutOrganizationPageProps extends TelemetryProps, TelemetryV2Props {}

export const AboutOrganizationPage: React.FunctionComponent<React.PropsWithChildren<AboutOrganizationPageProps>> = ({
    telemetryService,
    telemetryRecorder,
}) => {
    useEffect(() => {
        telemetryService.logViewEvent('AboutOrg')
        telemetryRecorder.recordEvent('settings.aboutOrganizations', 'view')
    }, [telemetryService, telemetryRecorder])

    return (
        <>
            <PageTitle title="Organizations" />
            <PageHeader
                headingElement="h2"
                path={[{ text: 'Organizations' }]}
                description="Support for organizations is not currently available on Sourcegraph.com."
                className="mb-3"
            />
            <SelfHostedCta
                contentClassName={styles.selfHostedCtaContent}
                page="organizations"
                telemetryService={telemetryService}
                telemetryRecorder={telemetryRecorder}
            >
                <Text className="mb-2">
                    <strong>Need more enterprise features? Run Sourcegraph self-hosted</strong>
                </Text>
                <Text className="mb-2">
                    For additional code hosts and enterprise only features, install Sourcegraph self-hosted.
                </Text>
            </SelfHostedCta>
        </>
    )
}
