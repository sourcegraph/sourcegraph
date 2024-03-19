import { type FC, type ReactNode, useEffect } from 'react'

import { useQuery } from '@apollo/client'
import { useNavigate, useParams } from 'react-router-dom'

import { useMutation } from '@sourcegraph/http-client'
import { ExternalServiceKind } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Alert, Button, ErrorAlert, H4, Link, LoadingSpinner } from '@sourcegraph/wildcard'

import { defaultExternalServices } from '../../../../../components/externalServices/externalServices'
import { LoaderButton } from '../../../../../components/LoaderButton'
import type {
    GetExternalServiceByIdResult,
    GetExternalServiceByIdVariables,
    UpdateRemoteCodeHostResult,
    UpdateRemoteCodeHostVariables,
} from '../../../../../graphql-operations'
import { UPDATE_CODE_HOST } from '../../../../queries'
import { GET_CODE_HOST_BY_ID } from '../../queries'

import { type CodeHostConnectFormFields, CodeHostJSONForm, type CodeHostJSONFormState } from './common'
import { GithubConnectView } from './github/GithubConnectView'

import styles from './CodeHostCreation.module.scss'

/**
 * Manually created type based on gql query schema, auto-generated tool can't infer
 * proper type for ExternalServices (because of problems with gql schema itself, node
 * type implementation problem that leads to have a massive union if when you use specific
 * type fragment)
 */
interface EditableCodeHost {
    __typename: 'ExternalService'
    id: string
    kind: ExternalServiceKind
    displayName: string
    config: string
}

interface CodeHostEditProps extends TelemetryProps, TelemetryV2Props {
    onCodeHostDelete: (codeHost: EditableCodeHost) => void
}

/**
 * Renders edit UI for any supported code host type. (Github, Gitlab, ...)
 * Also performs edit, delete actions over opened code host connection
 */
export const CodeHostEdit: FC<CodeHostEditProps> = props => {
    const { onCodeHostDelete, telemetryService, telemetryRecorder } = props
    const { codehostId } = useParams()

    useEffect(() => {
        telemetryRecorder.recordEvent('setup-wizard.code-host-edit', 'view')
    }, [telemetryRecorder])

    const { data, loading, error, refetch } = useQuery<GetExternalServiceByIdResult, GetExternalServiceByIdVariables>(
        GET_CODE_HOST_BY_ID,
        {
            fetchPolicy: 'cache-and-network',
            variables: { id: codehostId! },
        }
    )

    if (error && !loading) {
        return (
            <div>
                <ErrorAlert error={error} />
                <Button variant="secondary" outline={true} size="sm" onClick={() => refetch()}>
                    Try fetch again
                </Button>
            </div>
        )
    }

    if (!data || (!data && loading)) {
        return (
            <small className={styles.loadingState}>
                <LoadingSpinner /> Fetching connected code host...
            </small>
        )
    }

    if (data.node?.__typename !== 'ExternalService') {
        return (
            <Alert variant="warning">
                <H4>We either couldn't find code host</H4>
                Try to connect new code host instead <Link to="..">here</Link>
            </Alert>
        )
    }

    return (
        <CodeHostEditView
            key={codehostId}
            codeHostId={codehostId!}
            codeHostKind={data.node.kind}
            displayName={data.node.displayName}
            configuration={data.node.config}
            telemetryService={telemetryService}
            telemetryRecorder={telemetryRecorder}
        >
            {state => (
                <footer className={styles.footer}>
                    <LoaderButton
                        type="submit"
                        variant="primary"
                        size="sm"
                        label={state.submitting ? 'Updating' : 'Update'}
                        alwaysShowLabel={true}
                        loading={state.submitting}
                        disabled={state.submitting}
                    />

                    <Button as={Link} size="sm" to=".." variant="secondary">
                        Cancel
                    </Button>

                    <Button
                        variant="danger"
                        size="sm"
                        type="submit"
                        onClick={() => onCodeHostDelete(data.node as EditableCodeHost)}
                    >
                        Delete
                    </Button>
                </footer>
            )}
        </CodeHostEditView>
    )
}

interface CodeHostEditViewProps extends TelemetryProps, TelemetryV2Props {
    codeHostId: string
    codeHostKind: ExternalServiceKind
    displayName: string
    configuration: string
    children: (state: CodeHostJSONFormState) => ReactNode
}

const CodeHostEditView: FC<CodeHostEditViewProps> = props => {
    const { codeHostId, codeHostKind, displayName, configuration, telemetryService, telemetryRecorder, children } =
        props

    const navigate = useNavigate()
    const [updateRemoteCodeHost] = useMutation<UpdateRemoteCodeHostResult, UpdateRemoteCodeHostVariables>(
        UPDATE_CODE_HOST
    )

    const handleSubmit = async (values: CodeHostConnectFormFields): Promise<void> => {
        await updateRemoteCodeHost({
            variables: { input: { id: codeHostId, ...values } },
            refetchQueries: ['StatusAndRepoStats'],
        })

        navigate('..')
        // TODO show notification UI that code host has been added successfully
    }

    const initialValues: CodeHostConnectFormFields = {
        displayName,
        config: configuration,
    }

    if (codeHostKind === ExternalServiceKind.GITHUB) {
        return (
            <GithubConnectView
                initialValues={initialValues}
                externalServiceId={codeHostId}
                telemetryService={telemetryService}
                onSubmit={handleSubmit}
            >
                {children}
            </GithubConnectView>
        )
    }

    return (
        <CodeHostJSONForm
            initialValues={initialValues}
            externalServiceOptions={defaultExternalServices[codeHostKind]}
            onSubmit={handleSubmit}
            telemetryRecorder={telemetryRecorder}
        >
            {children}
        </CodeHostJSONForm>
    )
}
