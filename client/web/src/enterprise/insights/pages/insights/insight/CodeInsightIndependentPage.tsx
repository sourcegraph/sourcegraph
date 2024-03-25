import { type FunctionComponent, useContext, useEffect, useMemo } from 'react'

import { useParams } from 'react-router-dom'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { LoadingSpinner, PageHeader, useObservable } from '@sourcegraph/wildcard'

import { PageTitle } from '../../../../../components/PageTitle'
import { CodeInsightsIcon } from '../../../../../insights/Icons'
import { CodeInsightsPage } from '../../../components'
import { CodeInsightsBackendContext } from '../../../core'

import { CodeInsightIndependentPageActions } from './components/actions/CodeInsightIndependentPageActions'
import { StandaloneInsightDashboardPills } from './components/dashboard-pills/StandaloneInsightDashboardPills'
import { SmartStandaloneInsight } from './components/SmartStandaloneInsight'
import { Standalone404Insight } from './components/standalone-404-insight/Standalone404Insight'

import styles from './CodeInsightIndependentPage.module.scss'

interface CodeInsightIndependentPage extends TelemetryProps, TelemetryV2Props {}

export const CodeInsightIndependentPage: FunctionComponent<CodeInsightIndependentPage> = props => {
    const { telemetryService, telemetryRecorder } = props

    const { insightId } = useParams()
    const { getInsightById } = useContext(CodeInsightsBackendContext)

    const insight = useObservable(useMemo(() => getInsightById(insightId!), [getInsightById, insightId]))

    useEffect(() => {
        telemetryService.logPageView('StandaloneInsightPage')
        telemetryRecorder.recordEvent('insight', 'view')
    }, [telemetryService, telemetryRecorder])

    if (insight === undefined) {
        return <LoadingSpinner inline={false} />
    }

    if (!insight) {
        return <Standalone404Insight />
    }

    return (
        <CodeInsightsPage className={styles.root}>
            <PageTitle title={`${insight.title} - Code Insights`} />
            <PageHeader
                path={[{ to: '/insights/all', icon: CodeInsightsIcon }, { text: insight.title }]}
                actions={
                    <CodeInsightIndependentPageActions
                        insight={insight}
                        telemetryService={telemetryService}
                        telemetryRecorder={telemetryRecorder}
                    />
                }
            />

            <StandaloneInsightDashboardPills
                telemetryService={telemetryService}
                telemetryRecorder={telemetryRecorder}
                dashboards={insight.dashboards}
                insightId={insight.id}
                className={styles.dashboards}
            />

            <div className={styles.content}>
                <SmartStandaloneInsight
                    insight={insight}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                />
            </div>
        </CodeInsightsPage>
    )
}
