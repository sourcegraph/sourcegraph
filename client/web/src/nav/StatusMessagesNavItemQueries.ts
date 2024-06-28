import { gql } from '@sourcegraph/http-client'

export const STATUS_AND_REPO_COUNT = gql`
    query StatusAndRepoCount {
        repositoryStats {
            __typename
            total
        }
        statusMessages {
            ... on GitUpdatesDisabled {
                __typename

                message
            }

            ... on NoRepositoriesDetected {
                __typename

                message
            }

            ... on CloningProgress {
                __typename

                message
            }

            ... on IndexingProgress {
                __typename

                notIndexed
                indexed
            }

            ... on SyncError {
                __typename

                message
            }

            ... on GitserverDiskThresholdReached {
                __typename

                message
            }

            ... on ExternalServiceSyncError {
                __typename

                externalService {
                    id
                    displayName
                }
            }
        }

        repositories {
            totalCount
        }
    }
`
