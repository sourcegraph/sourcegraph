import classnames from 'classnames'
import React, { useCallback, useContext, useEffect, useMemo } from 'react'
import { useHistory, useParams } from 'react-router-dom'

import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { asError } from '@sourcegraph/shared/src/util/errors'
import { useLocalStorage } from '@sourcegraph/shared/src/util/useLocalStorage'

import { Page } from '../../../../../components/Page'
import { PageTitle } from '../../../../../components/PageTitle'
import { FORM_ERROR, FormChangeEvent } from '../../../../components/form/hooks/useForm'
import { InsightsApiContext } from '../../../../core/backend/api-provider'
import { addInsightToSettings } from '../../../../core/settings-action/insights'
import { isVirtualDashboard } from '../../../../core/types'
import { useDashboard } from '../../../../hooks/use-dashboard'
import { useInsightSubjects } from '../../../../hooks/use-insight-subjects/use-insight-subjects'

import {
    LangStatsInsightCreationContent,
    LangStatsInsightCreationContentProps,
} from './components/lang-stats-insight-creation-content/LangStatsInsightCreationContent'
import styles from './LangStatsInsightCreationPage.module.scss'
import { LangStatsCreationFormFields } from './types'
import { getSanitizedLangStatsInsight } from './utils/insight-sanitizer'

const DEFAULT_FINAL_SETTINGS = {}

export interface LangStatsInsightCreationPageProps
    extends PlatformContextProps<'updateSettings'>,
        SettingsCascadeProps,
        TelemetryProps {}

export const LangStatsInsightCreationPage: React.FunctionComponent<LangStatsInsightCreationPageProps> = props => {
    const { settingsCascade, platformContext, telemetryService } = props
    const { getSubjectSettings, updateSubjectSettings } = useContext(InsightsApiContext)
    const history = useHistory()

    const [initialFormValues, setInitialFormValues] = useLocalStorage<LangStatsCreationFormFields | undefined>(
        'insights.code-stats-creation-ui',
        undefined
    )

    const { dashboardId } = useParams<{ dashboardId?: string }>()
    const dashboard = useDashboard({ settingsCascade, dashboardId })

    // Set dashboard scope as initial value for the insight visibility
    const mergedInitialValues = useMemo<Partial<LangStatsCreationFormFields>>(() => {
        if (!dashboard || isVirtualDashboard(dashboard)) {
            return initialFormValues ?? {}
        }

        return { ...(initialFormValues ?? {}), visibility: dashboard.owner.id }
    }, [dashboard, initialFormValues])

    const insightSubjects = useInsightSubjects({ settingsCascade })

    useEffect(() => {
        telemetryService.logViewEvent('CodeInsightsCodeStatsCreationPage')
    }, [telemetryService])

    const handleSubmit = useCallback<LangStatsInsightCreationContentProps['onSubmit']>(
        async values => {
            const subjectID = values.visibility

            try {
                const settings = await getSubjectSettings(subjectID).toPromise()

                const insight = getSanitizedLangStatsInsight(values)
                const editedSettings = addInsightToSettings(settings.contents, insight)

                await updateSubjectSettings(platformContext, subjectID, editedSettings).toPromise()

                // Clear initial values if user successfully created search insight
                setInitialFormValues(undefined)
                telemetryService.log('CodeInsightsCodeStatsCreationPageSubmitClick')

                if (!dashboard || isVirtualDashboard(dashboard)) {
                    // Navigate user to the dashboard page with new created dashboard
                    history.push(`/insights/dashboards/${insight.visibility}`)

                    return
                }

                if (dashboard.owner.id === insight.visibility) {
                    history.push(`/insights/dashboards/${dashboard.id}`)
                } else {
                    history.push(`/insights/dashboards/${insight.visibility}`)
                }
            } catch (error) {
                return { [FORM_ERROR]: asError(error) }
            }

            return
        },
        [
            dashboard,
            getSubjectSettings,
            updateSubjectSettings,
            platformContext,
            setInitialFormValues,
            telemetryService,
            history,
        ]
    )

    const handleCancel = useCallback(() => {
        // Clear initial values if user successfully created search insight
        setInitialFormValues(undefined)
        telemetryService.log('CodeInsightsCodeStatsCreationPageCancelClick')
        history.push(`/insights/dashboards/${dashboard?.id ?? 'all'}`)
    }, [dashboard, history, setInitialFormValues, telemetryService])

    const handleChange = (event: FormChangeEvent<LangStatsCreationFormFields>): void => {
        setInitialFormValues(event.values)
    }

    return (
        <Page className={classnames(styles.creationPage, 'col-10')}>
            <PageTitle title="Create new code insight" />

            <div className="mb-5">
                <h2>Set up new language usage insight</h2>

                <p className="text-muted">
                    Shows language usage in your repository based on number of lines of code.{' '}
                    <a href="https://docs.sourcegraph.com/code_insights" target="_blank" rel="noopener">
                        Learn more.
                    </a>
                </p>
            </div>

            <LangStatsInsightCreationContent
                className="pb-5"
                settings={settingsCascade.final ?? DEFAULT_FINAL_SETTINGS}
                initialValues={mergedInitialValues}
                subjects={insightSubjects}
                onSubmit={handleSubmit}
                onCancel={handleCancel}
                onChange={handleChange}
            />
        </Page>
    )
}
