import classnames from 'classnames'
import React, { useCallback, useContext } from 'react'
import { Redirect } from 'react-router'
import { RouteComponentProps } from 'react-router-dom'

import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { asError } from '@sourcegraph/shared/src/util/errors'

import { AuthenticatedUser } from '../../../../auth'
import { Page } from '../../../../components/Page'
import { PageTitle } from '../../../../components/PageTitle'
import { FORM_ERROR } from '../../../components/form/hooks/useForm'
import { InsightsApiContext } from '../../../core/backend/api-provider'
import { addInsightToCascadeSetting } from '../../../core/jsonc-operation'

import {
    LangStatsInsightCreationContent,
    LangStatsInsightCreationContentProps,
} from './components/lang-stats-insight-creation-content/LangStatsInsightCreationContent'
import styles from './LangStatsInsightCreationPage.module.scss'
import { getSanitizedLangStatsInsight } from './utils/insight-sanitizer'

const DEFAULT_FINAL_SETTINGS = {}

export interface LangStatsInsightCreationPageProps
    extends PlatformContextProps<'updateSettings'>,
        Pick<RouteComponentProps, 'history'>,
        SettingsCascadeProps {
    /**
     * Authenticated user info, Used to decide where code insight will appears
     * in personal dashboard (private) or in organization dashboard (public)
     * */
    authenticatedUser: Pick<AuthenticatedUser, 'id' | 'organizations'> | null
}

export const LangStatsInsightCreationPage: React.FunctionComponent<LangStatsInsightCreationPageProps> = props => {
    const { history, authenticatedUser, settingsCascade, platformContext } = props
    const { getSubjectSettings, updateSubjectSettings } = useContext(InsightsApiContext)

    const handleSubmit = useCallback<LangStatsInsightCreationContentProps['onSubmit']>(
        async values => {
            if (!authenticatedUser) {
                return
            }

            const {
                id: userID,
                organizations: { nodes: orgs },
            } = authenticatedUser
            const subjectID =
                values.visibility === 'personal'
                    ? userID
                    : // TODO [VK] Add org picker in creation UI and not just pick first organization
                      orgs[0].id

            try {
                const settings = await getSubjectSettings(subjectID).toPromise()

                const insight = getSanitizedLangStatsInsight(values)
                const editedSettings = addInsightToCascadeSetting(settings.contents, insight)

                await updateSubjectSettings(platformContext, subjectID, editedSettings).toPromise()

                history.push('/insights')
            } catch (error) {
                return { [FORM_ERROR]: asError(error) }
            }

            return
        },
        [history, updateSubjectSettings, getSubjectSettings, platformContext, authenticatedUser]
    )

    const handleCancel = useCallback(() => {
        history.push('/insights')
    }, [history])

    if (authenticatedUser === null) {
        return <Redirect to="/" />
    }

    return (
        <Page className={classnames(styles.creationPage, 'col-10')}>
            <PageTitle title="Create new code insight" />

            <div className="mb-5">
                <h2>Set up new language usage insight</h2>

                <p className="text-muted">
                    Shows language usage in your repository based on number of lines of code.{' '}
                    <a
                        href="https://docs.sourcegraph.com/dev/background-information/insights"
                        target="_blank"
                        rel="noopener"
                    >
                        Learn more.
                    </a>
                </p>
            </div>

            <LangStatsInsightCreationContent
                className="pb-5"
                settings={settingsCascade.final ?? DEFAULT_FINAL_SETTINGS}
                onSubmit={handleSubmit}
                onCancel={handleCancel}
            />
        </Page>
    )
}
