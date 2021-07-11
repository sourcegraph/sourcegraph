import Dialog from '@reach/dialog';
import { VisuallyHidden } from '@reach/visually-hidden';
import classnames from 'classnames';
import CloseIcon from 'mdi-react/CloseIcon';
import React, { useContext, useMemo } from 'react'

import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context';
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings';
import { asError } from '@sourcegraph/shared/src/util/errors';

import { FORM_ERROR, SubmissionErrors } from '../../../../../components/form/hooks/useForm';
import { InsightsApiContext } from '../../../../../core/backend/api-provider';
import { updateDashboardInsightIds } from '../../../../../core/settings-action/dashboards';
import { RealInsightDashboard } from '../../../../../core/types';

import styles from './AddInsightModal.module.scss'
import {
    AddInsightFormValues,
    AddInsightModalContent
} from './components/add-insight-modal-content/AddInsightModalContent';
import { useReachableInsights } from './hooks/use-reachable-insights';

export interface AddInsightModalProps extends SettingsCascadeProps, PlatformContextProps {
    dashboard: RealInsightDashboard
    onClose: () => void
}

export const AddInsightModal: React.FunctionComponent<AddInsightModalProps> = props => {
    const { dashboard, settingsCascade, platformContext, onClose } = props
    const { getSubjectSettings, updateSubjectSettings } = useContext(InsightsApiContext)

    const insights = useReachableInsights({ ownerId: dashboard.owner.id, settingsCascade })

    const initialValues = useMemo<AddInsightFormValues>(() => ({
        searchInput: '',
        insightIds: dashboard.insightIds ?? []}),
        [dashboard]
    )

    const handleSubmit = async (values: AddInsightFormValues): Promise<void | SubmissionErrors> => {

        try {
            const { insightIds } = values;
            const settings = await getSubjectSettings(dashboard.owner.id).toPromise()

            const editedSettings = updateDashboardInsightIds(settings.contents, dashboard.id, insightIds)

            await updateSubjectSettings(platformContext, dashboard.owner.id, editedSettings).toPromise()
        } catch (error) {
            return { [FORM_ERROR]: asError(error) }
        }
    }

    return (
        <Dialog className={styles.modal} onDismiss={close}>
            <button
                type='button'
                className={classnames('btn btn-icon', styles.closeButton)}
                onClick={onClose}>

                <VisuallyHidden>Close</VisuallyHidden>
                <CloseIcon/>
            </button>

            <h2 className=''>
                Add insight to the {' '}
                <span className='font-italic'>"{ dashboard.title }"</span> {' '}
                 dashboard
            </h2>

            <span className='text-muted d-block mb-4'>
                Dashboards group your insights and let you share them with others. {' '}
                <a href="https://docs.sourcegraph.com/code_insights" target="_blank" rel="noopener">Learn more</a>
            </span>

            {
                !insights.length &&
                    <span>There are no insights for this dashboard.</span>
            }

            {
                insights.length > 0 &&
                    <AddInsightModalContent
                        initialValues={initialValues}
                        insights={insights}
                        onCancel={onClose}
                        onSubmit={handleSubmit}/>
            }
        </Dialog>
    )
}

