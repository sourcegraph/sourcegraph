import { useCallback, useEffect, useState } from 'react'

import { useApolloClient, useLazyQuery } from '@apollo/client'
import { isEqual } from 'lodash'

import { ErrorLike } from '@sourcegraph/common'
import { useMutation, useQuery } from '@sourcegraph/http-client'

import {
    AddRemoteCodeHostResult,
    AddRemoteCodeHostVariables,
    DeleteRemoteCodeHostResult,
    DeleteRemoteCodeHostVariables,
    DiscoverLocalRepositoriesResult,
    DiscoverLocalRepositoriesVariables,
    ExternalServiceKind,
    GetLocalCodeHostsResult,
    GetLocalDirectoryPathResult,
    LocalRepository,
} from '../../../graphql-operations'
import { ADD_CODE_HOST, DELETE_CODE_HOST } from '../../queries'

import { createDefaultLocalServiceConfig, getLocalServicePaths, getLocalServices } from './helpers'
import { DISCOVER_LOCAL_REPOSITORIES, GET_LOCAL_CODE_HOSTS, GET_LOCAL_DIRECTORY_PATH } from './queries'

type Path = string

interface useNewLocalRepositoriesPathsAPI {
    loading: boolean
    loaded: boolean
    error: ErrorLike | undefined
    paths: Path[]
    addNewPaths: (paths: Path[]) => Promise<void>
    deletePath: (path: Path) => Promise<void>
}

export function useNewLocalRepositoriesPaths(): useNewLocalRepositoriesPathsAPI {
    const { data, previousData, loading, error } = useQuery<GetLocalCodeHostsResult>(GET_LOCAL_CODE_HOSTS, {
        fetchPolicy: 'cache-and-network',
    })

    const apolloClient = useApolloClient()
    const [addLocalCodeHost] = useMutation<AddRemoteCodeHostResult, AddRemoteCodeHostVariables>(ADD_CODE_HOST)
    const [deleteLocalCodeHost] = useMutation<DeleteRemoteCodeHostResult, DeleteRemoteCodeHostVariables>(
        DELETE_CODE_HOST
    )

    const addNewPaths = async (paths: Path[]): Promise<void> => {
        for (const path of paths) {
            // Create a new local external service for this path
            await addLocalCodeHost({
                variables: {
                    input: {
                        displayName: `Local repositories service (${path})`,
                        config: createDefaultLocalServiceConfig(path),
                        kind: ExternalServiceKind.OTHER,
                    },
                },
            })
        }

        await apolloClient.refetchQueries({ include: ['GetLocalCodeHosts'] })
    }

    const deletePath = async (path: Path): Promise<void> => {
        const localServices = getLocalServices(data, false)
        const localServiceToDelete = localServices.find(localService => localService.path === path)

        if (!localServiceToDelete) {
            return
        }

        await deleteLocalCodeHost({ variables: { id: localServiceToDelete.id } })
        await apolloClient.refetchQueries({ include: ['GetLocalCodeHosts'] })
    }

    return {
        error,
        loading,
        addNewPaths,
        deletePath,
        loaded: !!data || !!previousData,
        paths: getLocalServicePaths(data),
    }
}

interface LocalRepositoriesPathAPI {
    loading: boolean
    loaded: boolean
    error: ErrorLike | undefined
    paths: Path[]
    autogeneratedPaths: Path[]
    setPaths: (newPaths: Path[]) => void
}

/**
 * Returns a list of local paths that we use to gather local repositories
 * from the user's machine. Internally, it stores paths with special type of
 * external service, if service doesn't exist it returns empty list.
 */
export function useLocalRepositoriesPaths(): LocalRepositoriesPathAPI {
    const apolloClient = useApolloClient()

    const [error, setError] = useState<ErrorLike | undefined>()
    const [paths, setPaths] = useState<string[]>([])

    const [addLocalCodeHost] = useMutation<AddRemoteCodeHostResult, AddRemoteCodeHostVariables>(ADD_CODE_HOST)

    const [deleteLocalCodeHost] = useMutation<DeleteRemoteCodeHostResult, DeleteRemoteCodeHostVariables>(
        DELETE_CODE_HOST
    )

    const { data, previousData, loading } = useQuery<GetLocalCodeHostsResult>(GET_LOCAL_CODE_HOSTS, {
        fetchPolicy: 'network-only',
        // Sync local external service paths on first load
        onCompleted: data => {
            setPaths(getLocalServicePaths(data))
        },
        onError: setError,
    })

    // Automatically creates or deletes local external service to
    // match user chosen paths for local repositories.
    useEffect(() => {
        if (loading) {
            return
        }

        const localServices = getLocalServices(data)
        const localServicePaths = getLocalServicePaths(data)
        const havePathsChanged = !isEqual(paths, localServicePaths)

        // Do nothing if paths haven't changed
        if (!havePathsChanged) {
            return
        }

        setError(undefined)

        async function syncExternalServices(): Promise<void> {
            // Create/update local external services
            for (const path of paths) {
                // If we already have a local external service for this path, skip it
                if (localServicePaths.includes(path)) {
                    continue
                }

                // Create a new local external service for this path
                await addLocalCodeHost({
                    variables: {
                        input: {
                            displayName: `Local repositories service (${path})`,
                            config: createDefaultLocalServiceConfig(path),
                            kind: ExternalServiceKind.OTHER,
                        },
                    },
                })
            }

            // Delete local external services that are no longer in the list
            for (const localService of localServices || []) {
                // If we still have a local external service for this path, skip it
                if (paths.includes(localService.path)) {
                    continue
                }

                // Delete local external service for this path
                await deleteLocalCodeHost({
                    variables: {
                        id: localService.id,
                    },
                })
            }

            // Refetch local external services and status after all mutations have been completed.
            await apolloClient.refetchQueries({ include: ['GetLocalCodeHosts', 'StatusAndRepoStats'] })
        }

        syncExternalServices().catch(setError)
    }, [data, paths, loading, apolloClient, addLocalCodeHost, deleteLocalCodeHost])

    return {
        error,
        loading,
        loaded: !!data || !!previousData,
        paths,
        autogeneratedPaths: getLocalServices(data, true).map(item => item.path),
        setPaths,
    }
}

interface LocalRepositoriesInput {
    paths: Path[]
    skip: boolean
}

interface LocalRepositoriesResult {
    loading: boolean
    error: ErrorLike | undefined
    loaded: boolean
    repositories: LocalRepository[]
}

/** Returns list of local repositories by a given list of local paths. */
export function useLocalRepositories({ paths, skip }: LocalRepositoriesInput): LocalRepositoriesResult {
    const { data, previousData, loading, error } = useQuery<
        DiscoverLocalRepositoriesResult,
        DiscoverLocalRepositoriesVariables
    >(DISCOVER_LOCAL_REPOSITORIES, {
        skip,
        variables: { paths },
        fetchPolicy: 'network-only',
    })

    return {
        loading,
        error,
        loaded: skip || !!data || !!previousData,
        repositories: data?.localDirectories?.repositories ?? previousData?.localDirectories?.repositories ?? [],
    }
}

interface LocalPathPickerAPI {
    callPathPicker: () => Promise<Path[]>
}

export function useLocalPathsPicker(): LocalPathPickerAPI {
    const [queryPath] = useLazyQuery<GetLocalDirectoryPathResult>(GET_LOCAL_DIRECTORY_PATH, {
        fetchPolicy: 'network-only',
    })

    const callPathPicker = useCallback(
        () =>
            queryPath().then(({ data, error }) => {
                if (error) {
                    throw new Error(error.message)
                }

                return data?.localDirectoriesPicker?.paths ?? []
            }),
        [queryPath]
    )

    return { callPathPicker }
}
