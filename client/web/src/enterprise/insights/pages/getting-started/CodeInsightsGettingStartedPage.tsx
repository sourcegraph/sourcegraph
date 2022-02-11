import React, { useEffect } from 'react'

import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'

import styles from './CodeInsightsGettingStartedPage.module.scss'
import { CodeInsightsExamples } from './components/code-insights-examples/CodeInsightsExamples'
import { CodeInsightsLearnMore } from './components/code-insights-learn-more/CodeInsightsLearnMore'
import { CodeInsightsTemplates } from './components/code-insights-templates/CodeInsightsTemplates'
import { DynamicCodeInsightExample } from './components/dynamic-code-insight-example/DynamicCodeInsightExample'

interface CodeInsightsGettingStartedPageProps extends TelemetryProps {}

export const CodeInsightsGettingStartedPage: React.FunctionComponent<CodeInsightsGettingStartedPageProps> = props => {
    const { telemetryService } = props

    useEffect(() => {
        telemetryService.logViewEvent('CodeInsightsGettingStartedPage')
    }, [telemetryService])

    return (
        <main className="pb-5">
            <DynamicCodeInsightExample telemetryService={telemetryService} />
            <CodeInsightsExamples telemetryService={telemetryService} className={styles.section} />
            <CodeInsightsTemplates className={styles.section} />
            <CodeInsightsLearnMore className={styles.section} />
        </main>
    )
}
