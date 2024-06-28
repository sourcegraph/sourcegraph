import { useCallback, useEffect, useState, type FormEventHandler, type FunctionComponent } from 'react'

import { useNavigate, useParams } from 'react-router-dom'

import { logger } from '@sourcegraph/common'
import { useMutation, useQuery } from '@sourcegraph/http-client'
import type { TelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { Alert, Button, ErrorAlert, Form, H3, LoadingSpinner, Modal } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { NamespaceSelector } from '../enterprise/batches/create/NamespaceSelector'
import { useNamespaces } from '../enterprise/batches/create/useNamespaces'
import type {
    SavedSearchFields,
    SavedSearchResult,
    SavedSearchVariables,
    TransferSavedSearchOwnershipResult,
    TransferSavedSearchOwnershipVariables,
    UpdateSavedSearchResult,
    UpdateSavedSearchVariables,
} from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { namespaceTelemetryMetadata } from '../namespaces/telemetry'

import { SavedSearchForm, type SavedSearchFormValue } from './Form'
import {
    deleteSavedSearchMutation,
    savedSearchQuery,
    transferSavedSearchOwnershipMutation,
    updateSavedSearchMutation,
} from './graphql'

interface Props extends NamespaceProps {
    authenticatedUser: AuthenticatedUser
    isSourcegraphDotCom: boolean
    id: string
}

export const UpdateForm: FunctionComponent<Omit<Props, 'id'>> = props => {
    const { id } = useParams<{ id: string }>()

    return <InnerUpdateForm {...props} id={id!} />
}

const InnerUpdateForm: FunctionComponent<Props> = ({
    id,
    namespace,
    authenticatedUser,
    telemetryRecorder,
    isSourcegraphDotCom,
}) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('savedSearches.update', 'view', {
            metadata: namespaceTelemetryMetadata(namespace),
        })
    }, [telemetryRecorder, namespace])
    const navigate = useNavigate()

    const result = useQuery<SavedSearchResult, SavedSearchVariables>(savedSearchQuery, { variables: { id } })

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
                        id,
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
        [namespace, telemetryRecorder, updateSavedSearch, id]
    )

    const savedSearch = result.data?.node?.__typename === 'SavedSearch' ? result.data.node : null

    const [showTransferOwnershipModal, setShowTransferOwnershipModal] = useState(false)

    const [deleteSavedSearch, { loading: deleteLoading, error: deleteError }] = useMutation(deleteSavedSearchMutation)
    const onDeleteClick = useCallback(async (): Promise<void> => {
        if (!savedSearch) {
            return
        }
        if (!window.confirm(`Delete the saved search ${JSON.stringify(savedSearch.description)}?`)) {
            return
        }
        try {
            await deleteSavedSearch({ variables: { id: savedSearch.id } })
            telemetryRecorder.recordEvent('savedSearches', 'delete', {
                metadata: namespaceTelemetryMetadata(namespace),
            })
            navigate(`${namespace.url}/searches`)
        } catch (error) {
            logger.error(error)
        }
    }, [savedSearch, deleteSavedSearch, telemetryRecorder, namespace, navigate])

    // If we're viewing this in a given owner namespace, ensure that the saved search is in that
    // namespace to avoid misleading the user about the actual owner.
    useEffect(() => {
        if (savedSearch && namespace.id !== savedSearch.owner.id) {
            navigate(savedSearch.url)
        }
    }, [namespace.id, navigate, savedSearch])

    return result.loading ? (
        <LoadingSpinner />
    ) : !savedSearch ? (
        <Alert variant="danger" as="p">
            Saved search not found.
        </Alert>
    ) : (
        <>
            <SavedSearchForm
                namespace={namespace}
                telemetryRecorder={telemetryRecorder}
                submitLabel="Save"
                onSubmit={onSubmit}
                otherButtons={
                    <>
                        <div className="flex-grow-1" />
                        {savedSearch.viewerCanAdminister && (
                            <Button
                                onClick={() => {
                                    telemetryRecorder.recordEvent('savedSearches.transferOwnership', 'openModal', {
                                        metadata: namespaceTelemetryMetadata(namespace),
                                    })
                                    setShowTransferOwnershipModal(true)
                                }}
                                disabled={updateLoading || deleteLoading}
                                variant="secondary"
                            >
                                Transfer ownership
                            </Button>
                        )}
                        {savedSearch.viewerCanAdminister && (
                            <Button
                                onClick={onDeleteClick}
                                disabled={updateLoading || deleteLoading}
                                variant="danger"
                                outline={true}
                            >
                                Delete
                            </Button>
                        )}
                    </>
                }
                isSourcegraphDotCom={isSourcegraphDotCom}
                initialValue={savedSearch}
                loading={updateLoading || deleteLoading}
                error={updateError ?? deleteError}
                flash={flashUpdated && 'Saved!'}
            />
            {showTransferOwnershipModal && (
                <TransferOwnershipModal
                    authenticatedUser={authenticatedUser}
                    savedSearch={savedSearch}
                    onClose={() => {
                        setShowTransferOwnershipModal(false)
                        telemetryRecorder.recordEvent('savedSearches.transferOwnership', 'closeModal', {
                            metadata: namespaceTelemetryMetadata(namespace),
                        })
                    }}
                    telemetryRecorder={telemetryRecorder}
                />
            )}
        </>
    )
}

const TransferOwnershipModal: FunctionComponent<{
    authenticatedUser: AuthenticatedUser
    savedSearch: Pick<SavedSearchFields, 'id' | 'owner'>
    onClose: () => void
    telemetryRecorder: TelemetryRecorder
}> = ({ authenticatedUser, savedSearch, onClose, telemetryRecorder }) => {
    const navigate = useNavigate()

    const { namespaces } = useNamespaces(authenticatedUser)
    const validNamespaces = namespaces.filter(ns => ns.id !== savedSearch.owner.id)
    const [selectedNamespace, setSelectedNamespace] = useState<string | undefined>(validNamespaces.at(0)?.id)

    const [transferSavedSearchOwnership, { loading, error }] = useMutation<
        TransferSavedSearchOwnershipResult,
        TransferSavedSearchOwnershipVariables
    >(transferSavedSearchOwnershipMutation, {})
    const onSubmit = useCallback<FormEventHandler>(
        async (event): Promise<void> => {
            event.preventDefault()
            try {
                const data = await transferSavedSearchOwnership({
                    variables: { id: savedSearch.id, newOwner: selectedNamespace! },
                })
                const updated = data.data?.transferSavedSearchOwnership
                if (!updated) {
                    return
                }
                telemetryRecorder.recordEvent('savedSearches.transferOwnership', 'submit', {
                    metadata: {
                        [`fromNamespaceType${savedSearch.owner.__typename}`]: 1,
                        [`toNamespaceType${updated.owner.__typename}`]: 1,
                    },
                })
                navigate(updated.url)
            } catch (error) {
                logger.error(error)
            }
        },
        [
            transferSavedSearchOwnership,
            savedSearch.id,
            savedSearch.owner.__typename,
            selectedNamespace,
            telemetryRecorder,
            navigate,
        ]
    )

    const MODAL_LABEL_ID = 'transfer-ownership-modal-label'

    return (
        <Modal aria-labelledby={MODAL_LABEL_ID} onDismiss={onClose}>
            <Form onSubmit={onSubmit} className="d-flex flex-column flex-gap-4">
                <H3 id={MODAL_LABEL_ID}>Transfer ownership of saved search</H3>
                {validNamespaces.length > 0 && selectedNamespace ? (
                    <>
                        <NamespaceSelector
                            namespaces={validNamespaces}
                            selectedNamespace={selectedNamespace}
                            onSelect={namespace => setSelectedNamespace(namespace.id)}
                            disabled={loading}
                            label="New owner"
                        />
                        <div className="d-flex flex-gap-2">
                            <Button type="submit" disabled={loading} variant="primary">
                                Transfer ownership
                            </Button>
                            <Button onClick={onClose} disabled={loading} variant="secondary" outline={true}>
                                Cancel
                            </Button>
                        </div>
                        {error && !loading && <ErrorAlert className="mb-0" error={error} />}
                    </>
                ) : (
                    <Alert variant="warning">
                        You aren't in any organizations to which you can transfer this saved search.
                    </Alert>
                )}
            </Form>
        </Modal>
    )
}
