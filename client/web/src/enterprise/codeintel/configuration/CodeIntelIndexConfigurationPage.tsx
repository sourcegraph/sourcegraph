import * as H from 'history'
import React, { FunctionComponent, useCallback, useEffect, useState } from 'react'
import { RouteComponentProps } from 'react-router'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import { ThemeProps } from '../../../../../shared/src/theme'
import { ErrorAlert } from '../../../components/alerts'
import { PageTitle } from '../../../components/PageTitle'
import { SettingsAreaRepositoryFields } from '../../../graphql-operations'
import { DynamicallyImportedMonacoSettingsEditor } from '../../../settings/DynamicallyImportedMonacoSettingsEditor'
import { getConfiguration as defaultGetConfiguration, updateConfiguration, enqueueIndexJob } from './backend'
import allConfigSchema from './schema.json'
import { CodeIntelAutoIndexSaveToolbar, AutoIndexProps } from '../../../components/CodeIntelAutoIndexSaveToolbar'
import { SaveToolbarPropsGenerator, SaveToolbarProps } from '../../../components/SaveToolbar'

export interface CodeIntelIndexConfigurationPageProps extends RouteComponentProps<{}>, ThemeProps, TelemetryProps {
    repo: Pick<SettingsAreaRepositoryFields, 'id'>
    history: H.History
    getConfiguration?: typeof defaultGetConfiguration
}

enum CodeIntelIndexEditorState {
    Idle,
    Saving,
    Queueing,
}

export const CodeIntelIndexConfigurationPage: FunctionComponent<CodeIntelIndexConfigurationPageProps> = ({
    repo,
    isLightTheme,
    telemetryService,
    history,
    getConfiguration = defaultGetConfiguration,
}) => {
    useEffect(() => telemetryService.logViewEvent('CodeIntelIndexConfigurationPage'), [telemetryService])

    const [fetchError, setFetchError] = useState<Error>()
    const [saveError, setSaveError] = useState<Error>()
    const [state, setState] = useState(() => CodeIntelIndexEditorState.Idle)
    const [configuration, setConfiguration] = useState<string>()
    const [dirty, setDirty] = useState<boolean>()

    useEffect(() => {
        const subscription = getConfiguration({ id: repo.id }).subscribe(configuration => {
            setConfiguration(configuration?.indexConfiguration?.configuration || '')
        }, setFetchError)

        return () => subscription.unsubscribe()
    }, [repo, getConfiguration])

    const save = useCallback(
        async (content: string) => {
            setState(CodeIntelIndexEditorState.Saving)
            setSaveError(undefined)

            try {
                await updateConfiguration({ id: repo.id, content }).toPromise()
                setConfiguration(content)
            } catch (error) {
                setSaveError(error)
            } finally {
                setState(CodeIntelIndexEditorState.Idle)
            }
        },
        [repo]
    )
    const enqueue = useCallback(async () => {
        setState(CodeIntelIndexEditorState.Queueing)
        setSaveError(undefined)

        try {
            await enqueueIndexJob(repo.id).toPromise()
        } catch (error) {
            setSaveError(error)
        } finally {
            setState(CodeIntelIndexEditorState.Idle)
        }
    }, [repo])

    const onDirtyChange = useCallback((dirty: boolean) => {
        setDirty(dirty)
    }, [])

    const saving = state === CodeIntelIndexEditorState.Saving
    const queueing = state === CodeIntelIndexEditorState.Queueing

    const customToolbar: {
        propsGenerator: SaveToolbarPropsGenerator<AutoIndexProps>
        saveToolbar: React.FunctionComponent<SaveToolbarProps & AutoIndexProps>
    } = {
        propsGenerator: (props: Readonly<SaveToolbarProps> & Readonly<{}>): SaveToolbarProps & AutoIndexProps => {
            const autoIndexProps: AutoIndexProps = {
                onQueueJob: enqueue,
                enqueueing: queueing,
            }

            const mergedProps = { ...props, ...autoIndexProps }
            mergedProps.willShowError = (): boolean => !queueing && !mergedProps.saving
            mergedProps.saveDiscardDisabled = (): boolean => saving || !dirty || queueing
            return mergedProps
        },
        saveToolbar: CodeIntelAutoIndexSaveToolbar,
    }

    return fetchError ? (
        <ErrorAlert prefix="Error fetching index configuration" error={fetchError} />
    ) : (
        <div className="code-intel-index-configuration web-content">
            <PageTitle title="Precise code intelligence index configuration" />
            <h2>Precise code intelligence index configuration</h2>
            <p>
                Override the inferred configuration when automatically indexing repositories on{' '}
                <a href="https://sourcegraph.com" target="_blank" rel="noreferrer noopener">
                    Sourcegraph.com
                </a>
                .
            </p>

            {saveError && <ErrorAlert prefix="Error saving index configuration" error={saveError} />}

            <DynamicallyImportedMonacoSettingsEditor
                value={configuration || ''}
                jsonSchema={allConfigSchema}
                canEdit={true}
                onSave={save}
                saving={saving}
                height={600}
                isLightTheme={isLightTheme}
                history={history}
                telemetryService={telemetryService}
                customSaveToolbar={customToolbar}
                onDirtyChange={onDirtyChange}
            />
        </div>
    )
}
