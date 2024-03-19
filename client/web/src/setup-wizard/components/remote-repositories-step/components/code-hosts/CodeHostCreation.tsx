import { type FC, type ReactNode, useMemo, useEffect } from 'react'

import type { Reference } from '@apollo/client'
import { useNavigate, useParams } from 'react-router-dom'

import { useMutation } from '@sourcegraph/http-client'
import { ExternalServiceKind } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Alert, Button, type FormChangeEvent, H4, Link, useLocalStorage } from '@sourcegraph/wildcard'

import { defaultExternalServices } from '../../../../../components/externalServices/externalServices'
import { LoaderButton } from '../../../../../components/LoaderButton'
import type { AddRemoteCodeHostResult, AddRemoteCodeHostVariables } from '../../../../../graphql-operations'
import { ADD_CODE_HOST, CODE_HOST_FRAGMENT } from '../../../../queries'
import { getCodeHostKindFromURLParam } from '../../helpers'

import { type CodeHostConnectFormFields, CodeHostJSONForm, type CodeHostJSONFormState } from './common'
import { GithubConnectView } from './github/GithubConnectView'
import { getRepositoriesSettings } from './github/helpers'

import styles from './CodeHostCreation.module.scss'

interface CodeHostCreationProps extends TelemetryProps, TelemetryV2Props {}

// For telemetry
const v2CodeHostKind: { [key in ExternalServiceKind]: number } = {
    AWSCODECOMMIT: 0,
    AZUREDEVOPS: 1,
    BITBUCKETCLOUD: 2,
    BITBUCKETSERVER: 3,
    GERRIT: 4,
    GITHUB: 5,
    GITLAB: 6,
    GITOLITE: 7,
    GOMODULES: 8,
    JVMPACKAGES: 9,
    NPMPACKAGES: 10,
    OTHER: 11,
    PAGURE: 12,
    PERFORCE: 13,
    PHABRICATOR: 14,
    PYTHONPACKAGES: 15,
    RUBYPACKAGES: 16,
    RUSTPACKAGES: 17,
}

/**
 * Renders creation UI for any supported code hosts (GitHub, Gitlab) based on
 * "codeHostType" URL param see root component routing logic.
 */
export const CodeHostCreation: FC<CodeHostCreationProps> = props => {
    const { telemetryService, telemetryRecorder } = props

    const { codeHostType } = useParams()
    const codeHostKind = getCodeHostKindFromURLParam(codeHostType!)

    useEffect(() => {
        if (codeHostKind === null) {
            return
        }

        telemetryService.log('SetupWizardCodeHostCreation', { kind: codeHostKind }, { kind: codeHostKind })
        telemetryRecorder.recordEvent('setupWizard.codeHost.creation', 'view', {
            metadata: { kind: v2CodeHostKind[codeHostKind] },
        })
    }, [telemetryService, codeHostKind, telemetryRecorder])

    if (codeHostKind === null) {
        return (
            <Alert variant="warning">
                <H4>We either couldn't find "{codeHostType}" code host option or we do not support this</H4>
                Pick one of supported code host option <Link to="..">here</Link>
            </Alert>
        )
    }

    // We render content inside react fragment because this view is rendered
    // within Container UI (avoid unnecessary DOM nesting)
    return (
        <CodeHostCreationView
            codeHostKind={codeHostKind}
            telemetryService={telemetryService}
            telemetryRecorder={telemetryRecorder}
        >
            {state => (
                <footer className={styles.footer}>
                    <LoaderButton
                        type="submit"
                        variant="primary"
                        size="sm"
                        label={state.submitting ? 'Connecting' : 'Connect'}
                        alwaysShowLabel={true}
                        loading={state.submitting}
                        disabled={state.submitting}
                    />
                    <Button as={Link} size="sm" to=".." variant="secondary">
                        Cancel
                    </Button>
                </footer>
            )}
        </CodeHostCreationView>
    )
}

interface CodeHostCreationFormProps extends TelemetryProps, TelemetryV2Props {
    codeHostKind: ExternalServiceKind
    children: (state: CodeHostJSONFormState) => ReactNode
}

/**
 * Renders specific creation UI form for particular code host type. Most of
 * the code hosts have similar UI but some of them (like GitHub) have special enhanced
 * UI with pickers and other form UI.
 */
const CodeHostCreationView: FC<CodeHostCreationFormProps> = props => {
    const { codeHostKind, children, telemetryService, telemetryRecorder } = props

    const navigate = useNavigate()
    const externalServiceOptions = defaultExternalServices[codeHostKind]

    const defaultConnectionValues = useMemo(
        () => ({
            displayName: externalServiceOptions.defaultDisplayName,
            config: getDefaultConfiguration(externalServiceOptions.defaultConfig, externalServiceOptions.kind),
        }),
        [externalServiceOptions.defaultConfig, externalServiceOptions.defaultDisplayName, externalServiceOptions.kind]
    )

    const [localValues, setLocalValues] = useLocalStorage<CodeHostConnectFormFields>(
        `${codeHostKind}-connect-form`,
        defaultConnectionValues
    )

    const [addRemoteCodeHost] = useMutation<AddRemoteCodeHostResult, AddRemoteCodeHostVariables>(ADD_CODE_HOST)

    const handleFormChange = (event: FormChangeEvent<CodeHostConnectFormFields>): void => {
        if (event.valid) {
            setLocalValues(event.values)
        }
    }

    const handleFormSubmit = async (values: CodeHostConnectFormFields): Promise<void> => {
        await addRemoteCodeHost({
            variables: { input: { ...values, kind: codeHostKind } },
            refetchQueries: ['StatusAndRepoStats'],
            update: (cache, result) => {
                const { data } = result

                if (!data) {
                    return
                }

                cache.modify({
                    fields: {
                        externalServices(codeHosts: { nodes: Reference[] }) {
                            const newCodeHost = cache.writeFragment({
                                data: data.addExternalService,
                                fragment: CODE_HOST_FRAGMENT,
                            })

                            const existingNodesRefs = codeHosts?.nodes ?? []
                            const nodes = [
                                newCodeHost,
                                // There was a problem that update function could be run twice and
                                // cache will have duplicate ref, so filter out newly created ref
                                // in case if we already have it in the cache
                                ...existingNodesRefs.filter(node => node.__ref !== newCodeHost?.__ref),
                            ]

                            // Update local cache and put newly created/connected code host
                            // to the beginning of code hosts list
                            return { nodes }
                        },
                    },
                })
            },
        })

        // GitHub supports Form UI and particular configuration schema where we can track
        // what modes user used to configure their GITHUB code host, so in case of GITHUB
        // we send these modes with SetupWizardConnectRemoteCodeHost event
        if (codeHostKind === ExternalServiceKind.GITHUB) {
            const { isAffiliatedRepositories, isOrgsRepositories, isSetRepositories } = getRepositoriesSettings(
                values.config
            )
            const eventProperties = {
                code_host: codeHostKind,
                isAffiliatedRepositories,
                isOrgsRepositories,
                isSetRepositories,
            }

            telemetryService.log('SetupWizardConnectRemoteCodeHost', eventProperties, eventProperties)
            telemetryRecorder.recordEvent('setupWizard.codeHost.creation', 'connect', {
                metadata: {
                    kind: v2CodeHostKind[codeHostKind],
                    isAffiliatedRepositories: isAffiliatedRepositories ? 1 : 0,
                    isOrgsRepositories: isOrgsRepositories ? 1 : 0,
                    isSetRepositories: isSetRepositories ? 1 : 0,
                },
            })
        } else {
            telemetryService.log(
                'SetupWizardConnectRemoteCodeHost',
                { code_host: codeHostKind },
                { code_host: codeHostKind }
            )
            telemetryRecorder.recordEvent('setupWizard.codeHost.creation', 'connect', {
                metadata: {
                    kind: v2CodeHostKind[codeHostKind],
                },
            })
        }

        // Reset local storage values
        setLocalValues(defaultConnectionValues)
        navigate('..')
        // TODO show notification UI that code host has been added successfully
    }

    if (codeHostKind === ExternalServiceKind.GITHUB) {
        return (
            <GithubConnectView
                initialValues={localValues}
                telemetryService={telemetryService}
                onChange={handleFormChange}
                onSubmit={handleFormSubmit}
            >
                {children}
            </GithubConnectView>
        )
    }

    return (
        <CodeHostJSONForm
            initialValues={localValues}
            externalServiceOptions={defaultExternalServices[codeHostKind]}
            onChange={handleFormChange}
            onSubmit={handleFormSubmit}
            telemetryRecorder={telemetryRecorder}
        >
            {children}
        </CodeHostJSONForm>
    )
}

const DEFAULT_GITHUB_CONNECTION_CONFIG = `
{
    "url": "https://github.com",
    "token": ""
}
`.trim()

function getDefaultConfiguration(defaultConfig: string, kind: ExternalServiceKind): string {
    // GitHub's connection form should have no orgs field by default
    // because it opens orgs option by default which should be hided in
    // setup wizard until user clicks the checkbox for organizations.
    if (kind === ExternalServiceKind.GITHUB) {
        return DEFAULT_GITHUB_CONNECTION_CONFIG
    }

    return defaultConfig
}
