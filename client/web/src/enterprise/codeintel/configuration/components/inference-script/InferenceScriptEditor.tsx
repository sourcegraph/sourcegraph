import { type FunctionComponent, useCallback, useMemo, useState } from 'react'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { screenReaderAnnounce, ErrorAlert } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../../../auth'
import {
    SaveToolbar,
    type SaveToolbarProps,
    type SaveToolbarPropsGenerator,
} from '../../../../../components/SaveToolbar'
import { DynamicallyImportedMonacoSettingsEditor } from '../../../../../settings/DynamicallyImportedMonacoSettingsEditor'
import { INFERENCE_SCRIPT } from '../../hooks/useInferenceScript'
import { useUpdateInferenceScript } from '../../hooks/useUpdateInferenceScript'

export interface InferenceScriptEditorProps extends TelemetryProps, TelemetryV2Props {
    script: string
    authenticatedUser: AuthenticatedUser | null
    setPreviewScript: (script: string) => void
}

export const InferenceScriptEditor: FunctionComponent<InferenceScriptEditorProps> = ({
    script: inferenceScript,
    setPreviewScript,
    authenticatedUser,
    telemetryService,
    telemetryRecorder,
}) => {
    const { updateInferenceScript, isUpdating, updatingError } = useUpdateInferenceScript()

    const save = useCallback(
        async (script: string) =>
            updateInferenceScript({
                variables: { script },
                refetchQueries: [INFERENCE_SCRIPT],
            }).then(() => {
                screenReaderAnnounce('Saved successfully')
                setDirty(false)
            }),
        [updateInferenceScript]
    )

    const [dirty, setDirty] = useState<boolean>()
    const isLightTheme = useIsLightTheme()

    const customToolbar = useMemo<{
        saveToolbar: FunctionComponent<SaveToolbarProps>
        propsGenerator: SaveToolbarPropsGenerator<{}>
    }>(
        () => ({
            saveToolbar: props => <SaveToolbar childrenPosition="start" {...props} />,
            propsGenerator: props => {
                const mergedProps = {
                    ...props,
                    loading: isUpdating,
                }
                mergedProps.willShowError = () => !mergedProps.saving
                mergedProps.saveDiscardDisabled = () => mergedProps.saving || !dirty

                return mergedProps
            },
        }),
        [dirty, isUpdating]
    )

    return (
        <>
            {updatingError && <ErrorAlert prefix="Error saving index configuration" error={updatingError} />}
            <DynamicallyImportedMonacoSettingsEditor
                value={inferenceScript}
                language="lua"
                canEdit={authenticatedUser?.siteAdmin}
                readOnly={!authenticatedUser?.siteAdmin}
                onSave={save}
                onChange={setPreviewScript}
                saving={isUpdating}
                height={600}
                isLightTheme={isLightTheme}
                telemetryService={telemetryService}
                telemetryRecorder={telemetryRecorder}
                customSaveToolbar={authenticatedUser?.siteAdmin ? customToolbar : undefined}
                onDirtyChange={setDirty}
            />
        </>
    )
}
