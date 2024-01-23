import type { QueryResult } from '@apollo/client'
import { parse as parseJSONC } from 'jsonc-parser'
import type { Observable } from 'rxjs'
import { map, mapTo, tap } from 'rxjs/operators'

import { resetAllMemoizationCaches } from '@sourcegraph/common'
import { createInvalidGraphQLMutationResponseError, dataOrThrowErrors, gql, useQuery } from '@sourcegraph/http-client'
import type { Settings } from '@sourcegraph/shared/src/settings/settings'

import { mutateGraphQL, queryGraphQL, requestGraphQL } from '../backend/graphql'
import {
    useShowMorePagination,
    type UseShowMorePaginationResult,
} from '../components/FilteredConnection/hooks/useShowMorePagination'
import type {
    AllConfigResult,
    CreateUserResult,
    DeleteOrganizationResult,
    DeleteOrganizationVariables,
    ExternalServiceKind,
    FeatureFlagFields,
    FeatureFlagsResult,
    FeatureFlagsVariables,
    OrganizationsConnectionFields,
    OrganizationsResult,
    OrganizationsVariables,
    OutOfBandMigrationFields,
    OutOfBandMigrationsResult,
    OutOfBandMigrationsVariables,
    RandomizeUserPasswordResult,
    ReloadSiteResult,
    ReloadSiteVariables,
    Scalars,
    ScheduleRepositoryPermissionsSyncResult,
    ScheduleRepositoryPermissionsSyncVariables,
    SetUserIsSiteAdminResult,
    SetUserIsSiteAdminVariables,
    SiteAdminAccessTokenConnectionFields,
    SiteAdminAccessTokensResult,
    SiteAdminAccessTokensVariables,
    SiteAdminSettingsCascadeFields,
    SiteResult,
    SiteUpdateCheckResult,
    SiteUpdateCheckVariables,
    UpdateSiteConfigurationResult,
    UpdateSiteConfigurationVariables,
    WebhookByIdResult,
    WebhookByIdVariables,
    WebhookFields,
    WebhookLogFields,
    WebhookLogsByWebhookIDResult,
    WebhookLogsByWebhookIDVariables,
    WebhookPageHeaderResult,
    WebhookPageHeaderVariables,
    WebhooksListResult,
    WebhooksListVariables,
    GitserversVariables,
    GitserversResult,
    GitserverFields,
    RepositoryJobFields,
    GlobalRepositoryJobsResult,
    GlobalRepositoryJobsVariables,
} from '../graphql-operations'
import { accessTokenFragment } from '../settings/tokens/AccessTokenNode'

import { WEBHOOK_LOGS_BY_ID } from './webhooks/backend'

/**
 * Fetches all organizations.
 */
export function fetchAllOrganizations(args: {
    first?: number
    query?: string
}): Observable<OrganizationsConnectionFields> {
    return requestGraphQL<OrganizationsResult, OrganizationsVariables>(
        gql`
            query Organizations($first: Int, $query: String) {
                organizations(first: $first, query: $query) {
                    ...OrganizationsConnectionFields
                }
            }

            fragment OrganizationsConnectionFields on OrgConnection {
                nodes {
                    ...OrganizationFields
                }
                totalCount
            }

            fragment OrganizationFields on Org {
                id
                name
                displayName
                createdAt
                latestSettings {
                    createdAt
                    contents
                }
                members {
                    totalCount
                }
            }
        `,
        {
            first: args.first ?? null,
            query: args.query ?? null,
        }
    ).pipe(
        map(dataOrThrowErrors),
        map(data => data.organizations)
    )
}

const mirrorRepositoryInfoFieldsFragment = gql`
    fragment MirrorRepositoryInfoFields on MirrorRepositoryInfo {
        cloned
        cloneInProgress
        cloneProgress @include(if: $displayCloneProgress)
        updatedAt
        nextSyncAt
        isCorrupted
        corruptionLogs {
            timestamp
        }
        lastError
        byteSize
        shard
    }
`

const externalRepositoryFieldsFragment = gql`
    fragment ExternalRepositoryFields on ExternalRepository {
        serviceType
        serviceID
    }
`

const siteAdminRepositoryFieldsFragment = gql`
    ${mirrorRepositoryInfoFieldsFragment}
    ${externalRepositoryFieldsFragment}

    fragment SiteAdminRepositoryFields on Repository {
        __typename
        id
        name
        createdAt
        viewerCanAdminister
        url
        isPrivate
        mirrorInfo {
            ...MirrorRepositoryInfoFields
        }
        externalRepository {
            ...ExternalRepositoryFields
        }
        embeddingExists
    }
`
export const REPOSITORIES_QUERY = gql`
    query Repositories(
        $first: Int
        $last: Int
        $after: String
        $before: String
        $query: String
        $indexed: Boolean
        $notIndexed: Boolean
        $embedded: Boolean
        $notEmbedded: Boolean
        $failedFetch: Boolean
        $corrupted: Boolean
        $cloneStatus: CloneStatus
        $orderBy: RepositoryOrderBy
        $descending: Boolean
        $externalService: ID
        $displayCloneProgress: Boolean = false
    ) {
        repositories(
            first: $first
            last: $last
            after: $after
            before: $before
            query: $query
            indexed: $indexed
            notIndexed: $notIndexed
            embedded: $embedded
            notEmbedded: $notEmbedded
            failedFetch: $failedFetch
            corrupted: $corrupted
            cloneStatus: $cloneStatus
            orderBy: $orderBy
            descending: $descending
            externalService: $externalService
        ) {
            nodes {
                ...SiteAdminRepositoryFields
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

    ${siteAdminRepositoryFieldsFragment}
`

export const REPO_PAGE_POLL_INTERVAL = 5000

export const SLOW_REQUESTS = gql`
    query SlowRequests($after: String) {
        slowRequests(after: $after) {
            nodes {
                index
                user {
                    username
                }
                start
                duration
                name
                source
                repository {
                    name
                }
                variables
                errors
                query
                filepath
            }
            totalCount
            pageInfo {
                endCursor
                hasNextPage
            }
        }
    }
`

export const OUTBOUND_REQUESTS = gql`
    query OutboundRequests($after: String) {
        outboundRequests(after: $after) {
            nodes {
                id
                startedAt
                method
                url
                requestHeaders {
                    name
                    values
                }
                requestBody
                statusCode
                responseHeaders {
                    name
                    values
                }
                durationMs
                errorMessage
                creationStackFrame
                callStack
            }
        }
    }
`
export const BACKGROUND_JOBS = gql`
    query BackgroundJobs($recentRunCount: Int) {
        backgroundJobs(recentRunCount: $recentRunCount) {
            nodes {
                name

                routines {
                    name
                    type
                    description
                    intervalMs
                    instances {
                        hostName
                        lastStartedAt
                        lastStoppedAt
                    }
                    recentRuns {
                        at
                        hostName
                        durationMs
                        errorMessage
                    }
                    stats {
                        since
                        runCount
                        errorCount
                        minDurationMs
                        avgDurationMs
                        maxDurationMs
                    }
                }
            }
        }
    }
`

export const OUTBOUND_REQUESTS_PAGE_POLL_INTERVAL_MS = 5000
export const BACKGROUND_JOBS_PAGE_POLL_INTERVAL_MS = 5000

export const UPDATE_MIRROR_REPOSITORY = gql`
    mutation UpdateMirrorRepository($repository: ID!) {
        updateMirrorRepository(repository: $repository) {
            alwaysNil
        }
    }
`

export const CHECK_MIRROR_REPOSITORY_CONNECTION = gql`
    mutation CheckMirrorRepositoryConnection($repository: ID!) {
        checkMirrorRepositoryConnection(repository: $repository) {
            error
        }
    }
`

export function scheduleRepositoryPermissionsSync(args: { repository: Scalars['ID'] }): Observable<void> {
    return requestGraphQL<ScheduleRepositoryPermissionsSyncResult, ScheduleRepositoryPermissionsSyncVariables>(
        gql`
            mutation ScheduleRepositoryPermissionsSync($repository: ID!) {
                scheduleRepositoryPermissionsSync(repository: $repository) {
                    alwaysNil
                }
            }
        `,
        args
    ).pipe(
        map(dataOrThrowErrors),
        tap(() => resetAllMemoizationCaches()),
        mapTo(undefined)
    )
}

export const RECLONE_REPOSITORY_MUTATION = gql`
    mutation RecloneRepository($repo: ID!) {
        recloneRepository(repo: $repo) {
            alwaysNil
        }
    }
`

/**
 * Fetches the site and its configuration.
 *
 * @returns Observable that emits the site
 */
export function fetchSite(): Observable<SiteResult['site']> {
    return queryGraphQL<SiteResult>(gql`
        query Site {
            site {
                __typename
                id
                canReloadSite
                configuration {
                    id
                    effectiveContents
                    validationMessages
                }
            }
        }
    `).pipe(
        map(dataOrThrowErrors),
        map(data => data.site)
    )
}

/**
 * Placeholder for the type of the external service config (to avoid explicit 'any' type)
 */
interface ExternalServiceConfig {}

type SettingsSubject = Pick<SiteAdminSettingsCascadeFields['subjects'][number], 'settingsURL' | '__typename'> & {
    contents: Settings
}

/**
 * All configuration and settings in one place.
 */
interface AllConfig {
    site: AllConfigResult['site']
    externalServices: Partial<Record<ExternalServiceKind, ExternalServiceConfig>>
    settings: {
        subjects: SettingsSubject[]
        final: Settings | null
    }
}

/**
 * Fetches all the configuration and settings (requires site admin privileges).
 */
export function fetchAllConfigAndSettings(): Observable<AllConfig> {
    return queryGraphQL<AllConfigResult>(
        gql`
            query AllConfig($first: Int) {
                site {
                    id
                    configuration {
                        id
                        effectiveContents
                    }
                    latestSettings {
                        contents
                    }
                    settingsCascade {
                        final
                    }
                }

                externalServices(first: $first) {
                    nodes {
                        id
                        kind
                        displayName
                        config
                        createdAt
                        updatedAt
                        warning
                    }
                }

                viewerSettings {
                    ...SiteAdminSettingsCascadeFields
                }
            }

            fragment SiteAdminSettingsCascadeFields on SettingsCascade {
                subjects {
                    __typename
                    latestSettings {
                        id
                        contents
                    }
                    settingsURL
                }
                final
            }
        `,
        { first: 100 } // assume no more than 100 external services added
    ).pipe(
        map(dataOrThrowErrors),
        map(data => {
            const externalServices: Partial<Record<ExternalServiceKind, ExternalServiceConfig[]>> =
                data.externalServices.nodes
                    .filter(svc => svc.config)
                    .map(svc => [svc.kind, parseJSONC(svc.config) as ExternalServiceConfig] as const)
                    .reduce<Partial<{ [k in ExternalServiceKind]: ExternalServiceConfig[] }>>(
                        (externalServicesByKind, [kind, config]) => {
                            let services = externalServicesByKind[kind]
                            if (!services) {
                                services = []
                                externalServicesByKind[kind] = services
                            }
                            services.push(config)
                            return externalServicesByKind
                        },
                        {}
                    )
            const settingsSubjects = data.viewerSettings.subjects.map(settings => ({
                __typename: settings.__typename,
                settingsURL: settings.settingsURL,
                contents: settings.latestSettings ? parseJSONC(settings.latestSettings.contents) : null,
            }))
            const finalSettings = parseJSONC(data.viewerSettings.final)
            return {
                site:
                    data.site?.configuration?.effectiveContents &&
                    parseJSONC(data.site.configuration.effectiveContents),
                externalServices,
                settings: {
                    subjects: settingsSubjects,
                    final: finalSettings,
                },
            }
        })
    )
}

/**
 * Updates the site's configuration.
 *
 * @returns An observable indicating whether or not a service restart is
 * required for the update to be applied.
 */
export function updateSiteConfiguration(lastID: number, input: string): Observable<boolean> {
    return requestGraphQL<UpdateSiteConfigurationResult, UpdateSiteConfigurationVariables>(
        gql`
            mutation UpdateSiteConfiguration($lastID: Int!, $input: String!) {
                updateSiteConfiguration(lastID: $lastID, input: $input)
            }
        `,
        { lastID, input }
    ).pipe(
        map(dataOrThrowErrors),
        map(data => data.updateSiteConfiguration)
    )
}

/**
 * Reloads the site.
 */
export function reloadSite(): Observable<void> {
    return requestGraphQL<ReloadSiteResult, ReloadSiteVariables>(
        gql`
            mutation ReloadSite {
                reloadSite {
                    alwaysNil
                }
            }
        `
    ).pipe(
        map(dataOrThrowErrors),
        map(data => {
            if (!data.reloadSite) {
                throw createInvalidGraphQLMutationResponseError('ReloadSite')
            }
        })
    )
}

export function setUserIsSiteAdmin(userID: Scalars['ID'], siteAdmin: boolean): Observable<void> {
    return requestGraphQL<SetUserIsSiteAdminResult, SetUserIsSiteAdminVariables>(
        gql`
            mutation SetUserIsSiteAdmin($userID: ID!, $siteAdmin: Boolean!) {
                setUserIsSiteAdmin(userID: $userID, siteAdmin: $siteAdmin) {
                    alwaysNil
                }
            }
        `,
        { userID, siteAdmin }
    ).pipe(
        map(dataOrThrowErrors),
        map(() => undefined)
    )
}

export function randomizeUserPassword(
    user: Scalars['ID']
): Observable<RandomizeUserPasswordResult['randomizeUserPassword']> {
    return mutateGraphQL<RandomizeUserPasswordResult>(
        gql`
            mutation RandomizeUserPassword($user: ID!) {
                randomizeUserPassword(user: $user) {
                    resetPasswordURL
                    emailSent
                }
            }
        `,
        { user }
    ).pipe(
        map(dataOrThrowErrors),
        map(data => data.randomizeUserPassword)
    )
}

export function createUser(username: string, email: string | undefined): Observable<CreateUserResult['createUser']> {
    return mutateGraphQL<CreateUserResult>(
        gql`
            mutation SiteAdminCreateUser($username: String!, $email: String) {
                createUser(username: $username, email: $email, verifiedEmail: false) {
                    resetPasswordURL
                }
            }
        `,
        { username, email }
    ).pipe(
        map(dataOrThrowErrors),
        map(data => data.createUser)
    )
}

export function deleteOrganization(organization: Scalars['ID']): Promise<void> {
    return requestGraphQL<DeleteOrganizationResult, DeleteOrganizationVariables>(
        gql`
            mutation DeleteOrganization($organization: ID!) {
                deleteOrganization(organization: $organization) {
                    alwaysNil
                }
            }
        `,
        { organization }
    )
        .pipe(
            map(dataOrThrowErrors),
            map(data => {
                if (!data.deleteOrganization) {
                    throw createInvalidGraphQLMutationResponseError('DeleteOrganization')
                }
            })
        )
        .toPromise()
}

export const SITE_UPDATE_CHECK = gql`
    query SiteUpdateCheck {
        site {
            buildVersion
            productVersion
            updateCheck {
                pending
                checkedAt
                errorMessage
                updateVersionAvailable
            }
        }
    }
`

export function fetchSiteUpdateCheck(): Observable<SiteUpdateCheckResult['site']> {
    return requestGraphQL<SiteUpdateCheckResult, SiteUpdateCheckVariables>(SITE_UPDATE_CHECK).pipe(
        map(dataOrThrowErrors),
        map(data => data.site)
    )
}

export const SITE_UPGRADE_READINESS = gql`
    query SiteUpgradeReadiness {
        site {
            upgradeReadiness {
                schemaDrift {
                    name
                    problem
                    solution
                    diff
                    statements
                    urlHint
                }
                requiredOutOfBandMigrations {
                    id
                    description
                }
            }
            autoUpgradeEnabled
        }
    }
`
export const GET_AUTO_UPGRADE = gql`
    query AutoUpgradeEnabled {
        site {
            autoUpgradeEnabled
        }
    }
`

export const SET_AUTO_UPGRADE = gql`
    mutation SetAutoUpgrade($enable: Boolean!) {
        setAutoUpgrade(enable: $enable) {
            alwaysNil
        }
    }
`

/**
 * Fetches all out-of-band migrations.
 */
export function fetchAllOutOfBandMigrations(): Observable<OutOfBandMigrationFields[]> {
    return requestGraphQL<OutOfBandMigrationsResult, OutOfBandMigrationsVariables>(
        gql`
            query OutOfBandMigrations {
                outOfBandMigrations {
                    ...OutOfBandMigrationFields
                }
            }

            fragment OutOfBandMigrationFields on OutOfBandMigration {
                id
                team
                component
                description
                introduced
                deprecated
                progress
                created
                lastUpdated
                nonDestructive
                applyReverse
                errors {
                    message
                    created
                }
            }
        `
    ).pipe(
        map(dataOrThrowErrors),
        map(data => data.outOfBandMigrations)
    )
}

/**
 * Fetches all feature flags.
 */
export function fetchFeatureFlags(): Observable<FeatureFlagFields[]> {
    return requestGraphQL<FeatureFlagsResult, FeatureFlagsVariables>(
        gql`
            query FeatureFlags {
                featureFlags {
                    ...FeatureFlagFields
                }
            }

            fragment FeatureFlagFields on FeatureFlag {
                __typename
                ... on FeatureFlagBoolean {
                    name
                    value
                    overrides {
                        ...OverrideFields
                    }
                    createdAt
                    updatedAt
                }
                ... on FeatureFlagRollout {
                    name
                    rolloutBasisPoints
                    overrides {
                        ...OverrideFields
                    }
                    createdAt
                    updatedAt
                }
            }

            fragment OverrideFields on FeatureFlagOverride {
                id
                value
                # Querying on namespace seems bugged, so we just get id and value for now.
            }
        `
    ).pipe(
        map(dataOrThrowErrors),
        map(data => data.featureFlags)
    )
}

export const STATUS_AND_REPO_STATS = gql`
    query StatusAndRepoStats {
        repositoryStats {
            __typename
            total
            notCloned
            cloned
            cloning
            failedFetch
            corrupted
            indexed
            embedded
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

            ... on ExternalServiceSyncError {
                __typename

                externalService {
                    id
                    displayName
                }
            }
        }
    }
`

export function queryAccessTokens(args: { first?: number }): Observable<SiteAdminAccessTokenConnectionFields> {
    return requestGraphQL<SiteAdminAccessTokensResult, SiteAdminAccessTokensVariables>(
        gql`
            query SiteAdminAccessTokens($first: Int) {
                site {
                    accessTokens(first: $first) {
                        ...SiteAdminAccessTokenConnectionFields
                    }
                }
            }
            fragment SiteAdminAccessTokenConnectionFields on AccessTokenConnection {
                nodes {
                    ...AccessTokenFields
                }
                totalCount
                pageInfo {
                    hasNextPage
                }
            }
            ${accessTokenFragment}
        `,
        { first: args.first ?? null }
    ).pipe(
        map(dataOrThrowErrors),
        map(data => data.site.accessTokens)
    )
}

export const SITE_EXTERNAL_SERVICE_CONFIG = gql`
    query SiteExternalServiceConfig {
        site {
            externalServicesFromFile
            allowEditExternalServicesWithFile
        }
    }
`

const WEBHOOK_FIELDS_FRAGMENT = gql`
    fragment WebhookFields on Webhook {
        id
        uuid
        url
        name
        codeHostKind
        codeHostURN
        secret
        updatedAt
        createdAt
        createdBy {
            username
            url
        }
        updatedBy {
            username
            url
        }
    }
`

export const WEBHOOKS = gql`
    ${WEBHOOK_FIELDS_FRAGMENT}

    query WebhooksList {
        webhooks {
            nodes {
                ...WebhookFields
            }
            totalCount
            pageInfo {
                hasNextPage
            }
        }
    }
`

export const WEBHOOK_BY_ID = gql`
    ${WEBHOOK_FIELDS_FRAGMENT}

    query WebhookById($id: ID!) {
        node(id: $id) {
            __typename
            ...WebhookFields
        }
    }
`

export const DELETE_WEBHOOK = gql`
    mutation DeleteWebhook($hookID: ID!) {
        deleteWebhook(id: $hookID) {
            alwaysNil
        }
    }
`

export const WEBHOOK_PAGE_HEADER = gql`
    query WebhookPageHeader {
        webhooks {
            nodes {
                webhookLogs {
                    totalCount
                }
            }
        }

        errorsOnly: webhooks {
            nodes {
                webhookLogs(onlyErrors: true) {
                    totalCount
                }
            }
        }
    }
`

export const useWebhookPageHeader = (): { loading: boolean; totalErrors: number; totalNoEvents: number } => {
    const { data, loading } = useQuery<WebhookPageHeaderResult, WebhookPageHeaderVariables>(WEBHOOK_PAGE_HEADER, {})
    const totalNoEvents = data?.webhooks.nodes.filter(webhook => webhook.webhookLogs?.totalCount === 0).length || 0
    const totalErrors =
        data?.errorsOnly.nodes.reduce((sum, webhook) => sum + (webhook.webhookLogs?.totalCount || 0), 0) || 0
    return { loading, totalErrors, totalNoEvents }
}

export const useWebhooksConnection = (): UseShowMorePaginationResult<WebhooksListResult, WebhookFields> =>
    useShowMorePagination<WebhooksListResult, WebhooksListVariables, WebhookFields>({
        query: WEBHOOKS,
        variables: {},
        getConnection: result => {
            const { webhooks } = dataOrThrowErrors(result)
            return webhooks
        },
    })

export const useWebhookQuery = (id: string): QueryResult<WebhookByIdResult, WebhookByIdVariables> =>
    useQuery<WebhookByIdResult, WebhookByIdVariables>(WEBHOOK_BY_ID, {
        variables: { id },
    })

export const useWebhookLogsConnection = (
    webhookID: string,
    first: number,
    onlyErrors: boolean
): UseShowMorePaginationResult<WebhookLogsByWebhookIDResult, WebhookLogFields> =>
    useShowMorePagination<WebhookLogsByWebhookIDResult, WebhookLogsByWebhookIDVariables, WebhookLogFields>({
        query: WEBHOOK_LOGS_BY_ID,
        variables: {
            first: first ?? 20,
            after: null,
            onlyErrors,
            onlyUnmatched: false,
            webhookID,
        },
        getConnection: result => {
            const { webhookLogs } = dataOrThrowErrors(result)
            return webhookLogs
        },
        options: {
            fetchPolicy: 'cache-first',
        },
    })

export const CREATE_WEBHOOK_QUERY = gql`
    mutation CreateWebhook($name: String!, $codeHostKind: String!, $codeHostURN: String!, $secret: String) {
        createWebhook(name: $name, codeHostKind: $codeHostKind, codeHostURN: $codeHostURN, secret: $secret) {
            id
        }
    }
`

export const UPDATE_WEBHOOK_QUERY = gql`
    mutation UpdateWebhook($id: ID!, $name: String!, $codeHostKind: String!, $codeHostURN: String!, $secret: String) {
        updateWebhook(id: $id, name: $name, codeHostKind: $codeHostKind, codeHostURN: $codeHostURN, secret: $secret) {
            id
        }
    }
`

export const EXTERNAL_SERVICE_KINDS = gql`
    query ExternalServiceKinds {
        externalServices {
            nodes {
                kind
            }
        }
    }
`

const siteAdminPackageFieldsFragment = gql`
    ${mirrorRepositoryInfoFieldsFragment}
    ${externalRepositoryFieldsFragment}

    fragment SiteAdminPackageFields on PackageRepoReference {
        id
        name
        kind
        blocked
        repository {
            id
            name
            url
            mirrorInfo {
                ...MirrorRepositoryInfoFields
            }
            externalRepository {
                ...ExternalRepositoryFields
            }
        }
    }
`

export const PACKAGES_QUERY = gql`
    query Packages(
        $kind: PackageRepoReferenceKind
        $name: String
        $first: Int!
        $after: String
        $displayCloneProgress: Boolean = false
    ) {
        packageRepoReferences(kind: $kind, name: $name, first: $first, after: $after) {
            nodes {
                ...SiteAdminPackageFields
            }
            totalCount
            pageInfo {
                hasNextPage
                endCursor
            }
        }
    }

    ${siteAdminPackageFieldsFragment}
`

export const SITE_CONFIGURATION_CHANGE_CONNECTION_QUERY = gql`
    query SiteConfigurationHistory($first: Int, $last: Int, $after: String, $before: String) {
        site {
            __typename
            configuration {
                history(first: $first, last: $last, after: $after, before: $before) {
                    __typename
                    totalCount
                    nodes {
                        __typename
                        ...SiteConfigurationChangeNode
                    }
                    pageInfo {
                        hasNextPage
                        hasPreviousPage
                        endCursor
                        startCursor
                    }
                }
            }
        }
    }

    fragment SiteConfigurationChangeNode on SiteConfigurationChange {
        id
        author {
            id
            username
            displayName
            avatarURL
        }
        diff
        createdAt
    }
`

const gitserverFieldsFragment = gql`
    fragment GitserverFields on GitserverInstance {
        id
        address
        freeDiskSpaceBytes
        totalDiskSpaceBytes
        repositoryJobs(first: 0) {
            stats {
                queued
                processing
                longestQueuedTime
            }
        }
    }
`

export const GITSERVERS = gql`
    query Gitservers {
        gitservers {
            nodes {
                ...GitserverFields
            }
        }
        site {
            gitserverDiskUsageWarningThreshold
        }
    }

    ${gitserverFieldsFragment}
`

export const useGitserversConnection = (): UseShowMorePaginationResult<GitserversResult, GitserverFields> =>
    useShowMorePagination<GitserversResult, GitserversVariables, GitserverFields>({
        query: GITSERVERS,
        variables: {},
        getConnection: result => {
            const { gitservers } = dataOrThrowErrors(result)
            return gitservers
        },
    })

const repositoryJobFieldsFragment = gql`
    fragment RepositoryJobFields on RepositoryJob {
        id
        backend
        type
        scheduleReason
        placeInQueue
        state
        failureMessage
        queuedAt
        startedAt
        finishedAt
        executionDeadlineSeconds
        repository {
            id
            name
            url
        }
        executionLogs {
            ...RepositoryJobExecutionLogEntryFields
        }
    }

    fragment RepositoryJobExecutionLogEntryFields on ExecutionLogEntry {
        key
        command
        startTime
        exitCode
        out
        durationMilliseconds
    }
`

export const GLOBAL_REPOSITORY_JOBS = gql`
    query GlobalRepositoryJobs($first: Int!, $after: String) {
        repositoryJobs(first: $first, after: $after) {
            totalCount
            pageInfo {
                hasNextPage
                endCursor
            }
            nodes {
                ...RepositoryJobFields
            }
        }
    }

    ${repositoryJobFieldsFragment}
`

export const REPOSITORY_JOBS_PER_PAGE_COUNT = 15

export const useGlobalRepositoryJobsConnection = (): UseShowMorePaginationResult<
    GlobalRepositoryJobsResult,
    RepositoryJobFields
> =>
    useShowMorePagination<GlobalRepositoryJobsResult, GlobalRepositoryJobsVariables, RepositoryJobFields>({
        query: GLOBAL_REPOSITORY_JOBS,
        variables: {
            first: REPOSITORY_JOBS_PER_PAGE_COUNT,
            after: null,
        },
        options: {
            useURL: true,
        },
        getConnection: result => {
            const { repositoryJobs } = dataOrThrowErrors(result)
            return repositoryJobs
        },
    })
