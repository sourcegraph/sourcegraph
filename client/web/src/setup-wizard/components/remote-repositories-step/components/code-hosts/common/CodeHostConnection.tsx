import { type FC, type ReactElement, type ReactNode, useState } from 'react'

import { mdiChevronDown, mdiChevronUp } from '@mdi/js'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import {
    Button,
    Collapse,
    CollapseHeader,
    CollapsePanel,
    Icon,
    Input,
    Text,
    FormGroup,
    getDefaultInputProps,
    type SubmissionErrors,
    useField,
    type useFieldAPI,
    useForm,
    ErrorAlert,
    FORM_ERROR,
    type FormChangeEvent,
} from '@sourcegraph/wildcard'

import type { AddExternalServiceOptions } from '../../../../../../components/externalServices/externalServices'
import { DynamicallyImportedMonacoSettingsEditor } from '../../../../../../settings/DynamicallyImportedMonacoSettingsEditor'

import styles from './CodeHostConnection.module.scss'

export interface CodeHostConnectFormFields {
    displayName: string
    config: string
}

export interface CodeHostJSONFormState {
    submitting: boolean
    submitErrors: SubmissionErrors
}

interface CodeHostJSONFormProps extends TelemetryV2Props {
    initialValues: CodeHostConnectFormFields
    children: (state: CodeHostJSONFormState) => ReactNode
    externalServiceOptions: AddExternalServiceOptions
    onSubmit: (values: CodeHostConnectFormFields) => Promise<void>
    onChange?: (event: FormChangeEvent<CodeHostConnectFormFields>) => void
}

export function CodeHostJSONForm(props: CodeHostJSONFormProps): ReactElement {
    const { initialValues, children, externalServiceOptions, onSubmit, onChange } = props

    const form = useForm<CodeHostConnectFormFields>({ initialValues, onChange, onSubmit })

    const displayName = useField({
        formApi: form.formAPI,
        name: 'displayName',
        required: true,
    })

    const configuration = useField({
        formApi: form.formAPI,
        name: 'config',
    })

    return (
        // eslint-disable-next-line react/forbid-elements
        <form ref={form.ref} className={styles.formView} onSubmit={form.handleSubmit}>
            <CodeHostJSONFormContent
                displayNameField={displayName}
                configurationField={configuration}
                externalServiceOptions={externalServiceOptions}
                telemetryRecorder={props.telemetryRecorder}
            />

            <>
                {form.formAPI.submitErrors && (
                    <ErrorAlert className="w-100" error={form.formAPI.submitErrors[FORM_ERROR]} />
                )}
            </>

            <div className={styles.footer}>{children(form.formAPI)}</div>
        </form>
    )
}

interface CodeHostJSONFormContentProps extends TelemetryV2Props {
    displayNameField: useFieldAPI<string>
    configurationField: useFieldAPI<string>
    externalServiceOptions: AddExternalServiceOptions
}

export function CodeHostJSONFormContent(props: CodeHostJSONFormContentProps): ReactElement {
    const { displayNameField, configurationField, externalServiceOptions } = props
    const isLightTheme = useIsLightTheme()

    // Fragment to avoid nesting since it's rendered within TabPanel fieldset
    return (
        <>
            <Input label="Display name" {...getDefaultInputProps(displayNameField)} />

            <FormGroup
                name="Configuration"
                title="Configuration"
                subtitle={<CodeHostInstructions instructions={externalServiceOptions.Instructions} />}
                labelClassName={styles.configurationGroupLabel}
            >
                <DynamicallyImportedMonacoSettingsEditor
                    // DynamicallyImportedMonacoSettingsEditor does not re-render the passed input.config
                    // if it thinks the config is dirty. We want to always replace the config if the kind changes
                    // so the editor is keyed on the kind.
                    value={configurationField.input.value}
                    actions={externalServiceOptions.editorActions}
                    jsonSchema={externalServiceOptions.jsonSchema}
                    canEdit={false}
                    controlled={true}
                    loading={true}
                    height={400}
                    readOnly={false}
                    isLightTheme={isLightTheme}
                    blockNavigationIfDirty={false}
                    onChange={configurationField.input.onChange}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                    telemetryRecorder={props.telemetryRecorder}
                    className={styles.configurationGroupEditor}
                    explanation={
                        <Text className="form-text text-muted" size="small">
                            Use Ctrl+Space for completion, and hover over JSON properties for documentation.
                        </Text>
                    }
                />
            </FormGroup>
        </>
    )
}

interface CodeHostInstructionsProps {
    instructions?: React.FunctionComponent
}

const CodeHostInstructions: FC<CodeHostInstructionsProps> = props => {
    const { instructions: Instructions } = props
    const [isInstructionOpen, setInstructionOpen] = useState(false)

    if (!Instructions) {
        return null
    }

    return (
        <Collapse isOpen={isInstructionOpen} onOpenChange={setInstructionOpen}>
            <CollapseHeader
                as={Button}
                outline={false}
                variant="link"
                size="sm"
                className={styles.configurationGroupInstructionButton}
            >
                See instructions how to fill out JSONC configuration{' '}
                <Icon aria-hidden={true} svgPath={isInstructionOpen ? mdiChevronDown : mdiChevronUp} className="mr-1" />
            </CollapseHeader>
            <CollapsePanel className={styles.configurationGroupInstructions}>
                <Instructions />
            </CollapsePanel>
        </Collapse>
    )
}
