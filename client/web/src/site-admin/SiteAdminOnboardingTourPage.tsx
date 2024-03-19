import { type FC, type PropsWithChildren, useState, useMemo } from 'react'

import AJV from 'ajv'
import addFormats from 'ajv-formats'

import { useMutation, useQuery } from '@sourcegraph/http-client'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import {
    PageHeader,
    Text,
    Container,
    BeforeUnloadPrompt,
    LoadingSpinner,
    Button,
    Alert,
    Badge,
} from '@sourcegraph/wildcard'

import onboardingSchemaJSON from '../../../../schema/onboardingtour.schema.json'
import { PageTitle } from '../components/PageTitle'
import { SaveToolbar } from '../components/SaveToolbar'
import type {
    OnboardingTourConfigMutationResult,
    OnboardingTourConfigMutationVariables,
    OnboardingTourConfigResult,
    OnboardingTourConfigVariables,
} from '../graphql-operations'
import { MonacoSettingsEditor } from '../settings/MonacoSettingsEditor'
import {
    ONBOARDING_TOUR_MUTATION,
    ONBOARDING_TOUR_QUERY,
    authenticatedTasks,
    defaultSnippets,
    parseTourConfig,
} from '../tour/data'

import { TourPreview } from './SiteAdminOnboardingTourPage/Preview'

const DEFAULT_VALUE = JSON.stringify(
    {
        tasks: authenticatedTasks,
        defaultSnippets,
    },
    null,
    2
)

const ajv = new AJV({ strict: false })
addFormats(ajv)

interface Props extends TelemetryProps {}

export const SiteAdminOnboardingTourPage: FC<PropsWithChildren<Props>> = () => {
    const isLightTheme = useIsLightTheme()
    const [value, setValue] = useState<string | null>(null)
    const { data, loading, error, previousData } = useQuery<OnboardingTourConfigResult, OnboardingTourConfigVariables>(
        ONBOARDING_TOUR_QUERY,
        {}
    )
    const existingConfiguration = data?.onboardingTourContent.current?.value
    const initialLoad = loading && !previousData
    const config = loading ? value ?? '' : value ?? existingConfiguration ?? DEFAULT_VALUE
    const dirty = !loading && config !== existingConfiguration

    const discard = (): void => {
        if (dirty && window.confirm('Discard onboarding tour changes?')) {
            setValue(existingConfiguration ?? DEFAULT_VALUE)
        }
    }

    const [updateOnboardinTourConfig, { loading: saving, error: mutationError }] = useMutation<
        OnboardingTourConfigMutationResult,
        OnboardingTourConfigMutationVariables
    >(ONBOARDING_TOUR_MUTATION, { refetchQueries: ['OnboardingTourConfig'] })

    function save(): void {
        if (value !== null) {
            // Conflicts with @typescript-eslint/no-floating-promises
            // eslint-disable-next-line no-void
            void updateOnboardinTourConfig({ variables: { json: value } })
        }
    }

    function reset(): void {
        if (window.confirm('Reset to default tour configuration?')) {
            setValue(DEFAULT_VALUE)
        }
    }

    const [parsedConfig, validationError] = useMemo(() => {
        if (!config) {
            return [null, null]
        }

        try {
            const parsedConfig = parseTourConfig(config)
            const isValid = ajv.validate(onboardingSchemaJSON, parsedConfig)
            if (!isValid) {
                throw new Error(ajv.errorsText(ajv.errors, { dataVar: 'config' }))
            }
            return [parsedConfig, null]
        } catch (error) {
            return [null, error]
        }
    }, [config])

    return (
        <>
            <PageTitle title="End user onboarding" />
            <PageHeader className="mb-3">
                <PageHeader.Heading as="h3" styleAs="h2">
                    <PageHeader.Breadcrumb>
                        <span className="d-inline-flex align-items-center">
                            <span>End user onboarding</span>{' '}
                            <Badge className="ml-2" variant="warning">
                                Experimental
                            </Badge>
                        </span>
                    </PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>
            <Text>This settings controls the onboarding task list that is displayed to all users by default.</Text>
            <Container>
                {initialLoad && <LoadingSpinner title="Loading onboarding configuration" />}
                {!initialLoad && (
                    <>
                        <BeforeUnloadPrompt when={saving || dirty} message="Discard settings changes?" />
                        <MonacoSettingsEditor
                            isLightTheme={isLightTheme}
                            language="json"
                            jsonSchema={onboardingSchemaJSON}
                            value={config}
                            onChange={setValue}
                            height={450}
                        />
                        <SaveToolbar
                            dirty={!validationError && dirty}
                            error={error || mutationError}
                            saving={saving}
                            onSave={save}
                            onDiscard={discard}
                        >
                            <Button
                                disabled={config === DEFAULT_VALUE}
                                onClick={reset}
                                className="ml-auto"
                                variant="secondary"
                            >
                                Reset
                            </Button>
                        </SaveToolbar>
                    </>
                )}
            </Container>
            <div className="mt-3">
                {validationError ? <Alert variant="danger">{validationError.message}</Alert> : null}
                {parsedConfig && <TourPreview config={parsedConfig} />}
            </div>
        </>
    )
}
