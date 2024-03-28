import { lastValueFrom } from 'rxjs'
import { map, tap } from 'rxjs/operators'

import { resetAllMemoizationCaches } from '@sourcegraph/common'
import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'
import type { Scalars } from '@sourcegraph/shared/src/graphql-operations'

import { requestGraphQL } from '../../../../backend/graphql'
import type {
    ScheduleUserPermissionsSyncResult,
    ScheduleUserPermissionsSyncVariables,
} from '../../../../graphql-operations'

export function scheduleUserPermissionsSync(args: { user: Scalars['ID'] }): Promise<void> {
    return lastValueFrom(
        requestGraphQL<ScheduleUserPermissionsSyncResult, ScheduleUserPermissionsSyncVariables>(
            gql`
                mutation ScheduleUserPermissionsSync($user: ID!) {
                    scheduleUserPermissionsSync(user: $user) {
                        alwaysNil
                    }
                }
            `,
            args
        ).pipe(
            map(dataOrThrowErrors),
            tap(() => resetAllMemoizationCaches()),
            map(() => undefined)
        )
    )
}

export const UserPermissionsInfoQuery = gql`
    query UserPermissionsInfo($userID: ID!, $first: Int, $last: Int, $after: String, $before: String, $query: String) {
        node(id: $userID) {
            __typename
            ... on User {
                ...UserPermissionsInfoUserNode
            }
        }
    }

    fragment UserPermissionsInfoUserNode on User {
        permissionsInfo {
            updatedAt
            source
            repositories(first: $first, last: $last, after: $after, before: $before, query: $query) {
                nodes {
                    ...PermissionsInfoRepositoryFields
                }
                totalCount
                pageInfo {
                    hasNextPage
                    hasPreviousPage
                    startCursor
                    endCursor
                }
            }
        }
    }

    fragment PermissionsInfoRepositoryFields on PermissionsInfoRepositoryNode {
        id
        reason
        updatedAt
        repository {
            id
            name
            url
            externalRepository {
                serviceType
            }
        }
    }
`
