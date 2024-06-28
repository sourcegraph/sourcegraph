import { useCallback, useEffect, useState, type FunctionComponent } from 'react'

import { useParams } from 'react-router-dom'

import { useMutation, useQuery } from '@sourcegraph/http-client'
import { Alert, Container, Link, LoadingSpinner } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import type {
    SavedSearchResult,
    SavedSearchVariables,
    UpdateSavedSearchResult,
    UpdateSavedSearchVariables,
} from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { namespaceTelemetryMetadata } from '../namespaces/telemetry'

import { savedSearchQuery, updateSavedSearchMutation } from './backend'
import { SavedSearchForm, type SavedSearchFormValue } from './SavedSearchForm'

interface Props extends NamespaceProps {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    id: string
}

export const SavedSearchUpdateForm: FunctionComponent<Omit<Props, 'id'>> = props => {
    const { id } = useParams<{ id: string }>()

    return <InnerSavedSearchUpdateForm {...props} id={id!} />
}

const InnerSavedSearchUpdateForm: FunctionComponent<Props> = ({
    id: searchId,
    namespace,
    telemetryRecorder,
    authenticatedUser,
    isSourcegraphDotCom,
}) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('savedSearches.update', 'view', {
            metadata: namespaceTelemetryMetadata(namespace),
        })
    }, [telemetryRecorder, namespace])

    const result = useQuery<SavedSearchResult, SavedSearchVariables>(savedSearchQuery, { variables: { id: searchId } })

    const [updateSavedSearch, { loading: updateLoading, error: updateError }] = useMutation<
        UpdateSavedSearchResult,
        UpdateSavedSearchVariables
    >(updateSavedSearchMutation, {})
    const [flashUpdated, setFlashUpdated] = useState(false)

    const onSubmit = useCallback(
        async (fields: SavedSearchFormValue): Promise<void> => {
            try {
                await updateSavedSearch({
                    variables: {
                        id: fields.id,
                        input: {
                            description: fields.description,
                            query: fields.query,
                        },
                    },
                })
                telemetryRecorder.recordEvent('savedSearches', 'update', {
                    metadata: namespaceTelemetryMetadata(namespace),
                })
                setFlashUpdated(true)
                setTimeout(() => {
                    setFlashUpdated(false)
                }, 1000)
            } catch {
                // Mutation error is read in useMutation call.
            }
        },
        [namespace, telemetryRecorder, updateSavedSearch]
    )

    const savedSearch = result.data?.node?.__typename === 'SavedSearch' ? result.data.node : null

    return (
        <div>
            {result.loading ? (
                <LoadingSpinner />
            ) : !savedSearch ? (
                <Alert variant="danger" as="p">
                    Saved search not found.
                </Alert>
            ) : (
                <SavedSearchForm
                    namespace={namespace}
                    telemetryRecorder={telemetryRecorder}
                    authenticatedUser={authenticatedUser}
                    submitLabel="Update saved search"
                    title="Edit saved search"
                    isSourcegraphDotCom={isSourcegraphDotCom}
                    defaultValues={savedSearch}
                    loading={updateLoading}
                    error={updateError}
                    onSubmit={(fields: Pick<SavedSearchFormValue, Exclude<keyof SavedSearchFormValue, 'id'>>): void =>
                        void onSubmit({ id: savedSearch.id, ...fields })
                    }
                />
            )}
            {flashUpdated && (
                <Alert variant="success" as="p">
                    Updated!
                </Alert>
            )}
            {savedSearch && (
                <Container className="p-3 mt-3">
                    To get notified when there are new results for this query, create a{' '}
                    <Link to="/code-monitoring">code monitor</Link>.
                </Container>
            )}
        </div>
    )
}
