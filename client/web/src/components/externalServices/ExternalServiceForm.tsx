import React, { type ReactNode, useCallback, useMemo } from 'react'

import AJV from 'ajv'
import addFormats from 'ajv-formats'
import { parse } from 'jsonc-parser'

import type { ErrorLike } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import {
    Button,
    LoadingSpinner,
    Alert,
    H4,
    Text,
    Input,
    ErrorAlert,
    ErrorMessage,
    Form,
    ButtonLink,
    Label,
} from '@sourcegraph/wildcard'

import type { AddExternalServiceInput } from '../../graphql-operations'
import { DynamicallyImportedMonacoSettingsEditor } from '../../settings/DynamicallyImportedMonacoSettingsEditor'

import { ExternalServiceEditingDisabledAlert } from './ExternalServiceEditingDisabledAlert'
import { ExternalServiceEditingTemporaryAlert } from './ExternalServiceEditingTemporaryAlert'
import type { AddExternalServiceOptions } from './externalServices'

interface Props
    extends Pick<AddExternalServiceOptions, 'jsonSchema' | 'editorActions'>,
        TelemetryProps,
        TelemetryV2Props {
    input: AddExternalServiceInput
    externalServiceID?: string
    error?: ErrorLike
    warning?: string | null
    mode: 'edit' | 'create'
    loading: boolean
    hideDisplayNameField?: boolean
    submitName?: string
    onSubmit: (event?: React.FormEvent<HTMLFormElement>) => void
    onChange: (change: AddExternalServiceInput) => void
    autoFocus?: boolean
    externalServicesFromFile: boolean
    allowEditExternalServicesWithFile: boolean
    additionalFormComponent?: ReactNode
}

const ajv = new AJV({ strict: false, $comment: true })
addFormats(ajv)

/**
 * Form for submitting a new or updated external service.
 */
export const ExternalServiceForm: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    telemetryService,
    jsonSchema,
    editorActions,
    input,
    externalServiceID,
    error,
    warning,
    mode,
    loading,
    hideDisplayNameField,
    submitName,
    onSubmit,
    onChange,
    externalServicesFromFile,
    allowEditExternalServicesWithFile,
    autoFocus = true,
    additionalFormComponent,
    telemetryRecorder,
}) => {
    const isLightTheme = useIsLightTheme()
    const onDisplayNameChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => {
            onChange({ ...input, displayName: event.currentTarget.value })
        },
        [input, onChange]
    )
    const onConfigChange = useCallback(
        (config: string): void => {
            onChange({ ...input, config })
        },
        [input, onChange]
    )
    const validate = useMemo(() => ajv.compile(jsonSchema), [jsonSchema])
    const configValid = useMemo<boolean>(() => {
        if (input.config) {
            const config = parse(input.config)
            return validate(config)
        }
        return false
    }, [input.config, validate])

    const disabled = externalServicesFromFile && !allowEditExternalServicesWithFile

    return (
        <Form className="external-service-form" onSubmit={onSubmit}>
            {error && <ErrorAlert error={error} />}
            {warning && (
                <Alert variant="warning">
                    <H4>Warning</H4>
                    <ErrorMessage error={warning} />
                </Alert>
            )}

            {disabled && <ExternalServiceEditingDisabledAlert />}
            {externalServicesFromFile && allowEditExternalServicesWithFile && <ExternalServiceEditingTemporaryAlert />}

            {hideDisplayNameField || (
                <Label className="w-100">
                    <Text className="mb-2">Display name</Text>
                    <Input
                        id="test-external-service-form-display-name"
                        required={true}
                        autoCorrect="off"
                        autoComplete="off"
                        autoFocus={autoFocus}
                        spellCheck={false}
                        value={input.displayName}
                        onChange={onDisplayNameChange}
                        disabled={loading || disabled}
                        className="mb-0"
                    />
                </Label>
            )}
            {additionalFormComponent && <>{additionalFormComponent}</>}
            <div className="form-group mt-3">
                <DynamicallyImportedMonacoSettingsEditor
                    // DynamicallyImportedMonacoSettingsEditor does not re-render the passed input.config
                    // if it thinks the config is dirty. We want to always replace the config if the kind changes
                    // so the editor is keyed on the kind.
                    value={input.config}
                    jsonSchema={jsonSchema}
                    canEdit={false}
                    controlled={true}
                    loading={loading}
                    height={350}
                    readOnly={disabled}
                    isLightTheme={isLightTheme}
                    onChange={onConfigChange}
                    actions={editorActions}
                    className="test-external-service-editor"
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                    explanation={
                        <Text className="form-text text-muted">
                            <small>
                                Use Ctrl+Space for completion, and hover over JSON properties for documentation.
                            </small>
                        </Text>
                    }
                />
            </div>
            {mode === 'edit' ? (
                <div className="d-flex flex-shrink-0 mt-2">
                    <div>
                        <Button
                            type="submit"
                            className="test-update-external-service-button"
                            disabled={loading || disabled}
                            variant="primary"
                        >
                            {loading && <LoadingSpinner />}
                            {submitName ?? 'Update configuration'}
                        </Button>
                    </div>
                    <div className="ml-1">
                        <ButtonLink
                            to={`/site-admin/external-services/${encodeURIComponent(externalServiceID ?? '')}`}
                            variant="secondary"
                        >
                            Cancel
                        </ButtonLink>
                    </div>
                </div>
            ) : (
                <Button
                    type="submit"
                    className="test-add-external-service-button"
                    disabled={loading || disabled || !configValid}
                    variant="primary"
                >
                    {loading && <LoadingSpinner />}
                    {submitName ?? 'Add connection'}
                </Button>
            )}
        </Form>
    )
}
