/**
 * Provides convenience functions for interacting with the Sourcegraph API from tests.
 */

import { zip, timer, concat, throwError, type Observable, lastValueFrom } from 'rxjs'
import { map, tap, retry, mergeMap } from 'rxjs/operators'

import { isErrorLike, createAggregateError, logger } from '@sourcegraph/common'
import {
    dataOrThrowErrors,
    createInvalidGraphQLMutationResponseError,
    isErrorGraphQLResult,
    gql,
} from '@sourcegraph/http-client'
import {
    CloneInProgressError,
    isCloneInProgressErrorLike,
    isRepoNotFoundErrorLike,
} from '@sourcegraph/shared/src/backend/errors'
import { viewerSettingsQuery } from '@sourcegraph/shared/src/backend/settings'
import type { ViewerSettingsResult, ViewerSettingsVariables } from '@sourcegraph/shared/src/graphql-operations'
import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import type { Config } from '@sourcegraph/shared/src/testing/config'

import type {
    AddExternalServiceInput,
    ExternalServiceKind,
    UpdateExternalServiceInput,
    Scalars,
    DeleteOrganizationResult,
    SearchPatternType,
    UpdateSiteConfigurationResult,
    SiteResult,
    OrganizationsConnectionFields,
    OrganizationsResult,
    CreateOrganizationResult,
    CreateUserResult,
    UpdateExternalServiceResult,
    UpdateExternalServiceVariables,
    OrganizationsVariables,
    SearchVersion,
    DeleteOrganizationVariables,
    CreateOrganizationVariables,
    CreateUserVariables,
    SetUserIsSiteAdminResult,
    SetUserIsSiteAdminVariables,
    DeleteExternalServiceResult,
    DeleteExternalServiceVariables,
    UpdateSiteConfigurationVariables,
    ResolveRevResult,
    ResolveRevVariables,
    ExternalServicesRegressionResult,
    ExternalServicesRegressionVariables,
    ExternalServiceNodeFields,
    UserResult,
    DeleteUserResult,
    DeleteUserVariables,
    SetTosAcceptedResult,
    SetTosAcceptedVariables,
    SiteProductVersionResult,
    SiteProductVersionVariables,
    UserVariables,
    addExternalServiceResult,
    addExternalServiceVariables,
    SearchResult,
    SearchVariables,
} from '../../graphql-operations'

import type { GraphQLClient } from './GraphQlClient'
import type { ResourceDestructor } from './TestResourceManager'

type WaitForRepoOptions = Partial<
    Pick<Config, 'logStatusMessages'> & {
        /**
         * If true, wait for the repositories *not* to exist, rather than to exist.
         */
        shouldNotExist?: boolean

        /**
         * How frequently to retry the test for repository existence/non-existence.
         */
        retryPeriod?: number

        /**
         * The maximum time to wait for the repository to exist.
         */
        timeout?: number

        /**
         * Wait for repository to be indexed, not just cloned. If shouldNotExist is true, this has
         * no effect.
         */
        indexed?: boolean
    }
>

/**
 * Wait until all repositories in the list exist.
 */
export async function waitForRepos(
    gqlClient: GraphQLClient,
    ensureRepos: string[],
    options: WaitForRepoOptions = {}
): Promise<void> {
    await lastValueFrom(zip(...ensureRepos.map(repoName => waitForRepo(gqlClient, repoName, options))))
}

export function waitForRepo(
    gqlClient: GraphQLClient,
    repoName: string,
    {
        logStatusMessages,
        shouldNotExist = false,
        retryPeriod = 5000, // 5 seconds
        timeout = 300000, // 5 minutes
        indexed: mustBeIndexed = false,
    }: WaitForRepoOptions = {}
): Observable<void> {
    const request = gqlClient.queryGraphQL<ResolveRevResult, ResolveRevVariables>(
        gql`
            query ResolveRev($repoName: String!) {
                repository(name: $repoName) {
                    mirrorInfo {
                        cloned
                    }
                    textSearchIndex {
                        status {
                            updatedAt
                        }
                    }
                }
            }
        `,
        { repoName }
    )
    const numberRetries = Math.ceil(timeout / retryPeriod)
    return shouldNotExist
        ? request.pipe(
              map(result => {
                  // map to true if repo is not found, false if repo is found, throw other errors
                  if (isErrorGraphQLResult(result) && result.errors.some(isRepoNotFoundErrorLike)) {
                      return undefined
                  }
                  const { repository } = dataOrThrowErrors(result)
                  if (!repository) {
                      return undefined
                  }
                  throw new Error('Repo exists')
              }),
              retry({
                  delay: (error, retryCount) => {
                      if (retryCount <= numberRetries) {
                          if (isErrorLike(error) && error.message === 'Repo exists') {
                              // Delay retry by 2s.
                              if (logStatusMessages) {
                                  logger.log(
                                      `Waiting for ${repoName} to be removed (attempt ${
                                          retryCount + 1
                                      } of ${numberRetries})`
                                  )
                              }
                              return timer(retryPeriod)
                          }
                          // Throw all errors
                          return throwError(() => error)
                      }
                      return throwError(() => new Error(`Could not resolve repo ${repoName}: too many retries`))
                  },
              })
          )
        : request.pipe(
              map(dataOrThrowErrors),
              // Wait until the repository is cloned even if it doesn't yet exist.
              // waitForRepos might be called immediately after adding a new external service,
              // and we have no guarantee that all the repositories from that external service
              // will exist when the add-external-service endpoint returns.
              tap(({ repository }) => {
                  if (!repository?.mirrorInfo?.cloned) {
                      throw new CloneInProgressError(repoName)
                  }
                  if (mustBeIndexed && !repository?.textSearchIndex?.status?.updatedAt) {
                      throw new CloneInProgressError(repoName)
                  }
              }),
              retry({
                  delay: (error, retryCount) => {
                      if (retryCount <= numberRetries) {
                          if (isCloneInProgressErrorLike(error)) {
                              // Delay retry by 2s.
                              if (logStatusMessages) {
                                  logger.log(
                                      `Waiting for ${repoName} to finish cloning (attempt ${
                                          retryCount + 1
                                      } of ${numberRetries})`
                                  )
                              }
                              return timer(retryPeriod)
                          }
                          // Throw all errors other than ECLONEINPROGRESS
                          return throwError(() => error)
                      }
                      return throwError(() => new Error(`Could not resolve repo ${repoName}: too many retries`))
                  },
              }),
              map(() => undefined)
          )
}

export async function ensureNoTestExternalServices(
    gqlClient: GraphQLClient,
    options: {
        kind: ExternalServiceKind
        uniqueDisplayName: string
        deleteIfExist?: boolean
    }
): Promise<void> {
    if (!options.uniqueDisplayName.startsWith('[TEST]')) {
        throw new Error(
            `Test external service name ${JSON.stringify(options.uniqueDisplayName)} must start with "[TEST]".`
        )
    }

    const externalServices = await getExternalServices(gqlClient, options)
    if (externalServices.length === 0) {
        return
    }
    if (!options.deleteIfExist) {
        throw new Error('external services already exist, not deleting')
    }

    for (const externalService of externalServices) {
        await lastValueFrom(
            gqlClient.mutateGraphQL<DeleteExternalServiceResult, DeleteExternalServiceVariables>(
                gql`
                    mutation DeleteExternalService($externalService: ID!) {
                        deleteExternalService(externalService: $externalService) {
                            alwaysNil
                        }
                    }
                `,
                { externalService: externalService.id }
            )
        )
    }
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export function getExternalServices(
    gqlClient: GraphQLClient,
    options: {
        kind?: ExternalServiceKind
        uniqueDisplayName?: string
    } = {}
): Promise<ExternalServiceNodeFields[]> {
    return lastValueFrom(
        gqlClient
            .queryGraphQL<ExternalServicesRegressionResult, ExternalServicesRegressionVariables>(
                gql`
                    query ExternalServicesRegression($first: Int) {
                        externalServices(first: $first) {
                            nodes {
                                ...ExternalServiceNodeFields
                            }
                        }
                    }

                    fragment ExternalServiceNodeFields on ExternalService {
                        id
                        kind
                        displayName
                        config
                        createdAt
                        updatedAt
                        warning
                    }
                `,
                { first: 100 }
            )
            .pipe(
                map(dataOrThrowErrors),
                map(({ externalServices }) =>
                    externalServices.nodes.filter(
                        ({ displayName, kind }) =>
                            (options.uniqueDisplayName === undefined || options.uniqueDisplayName === displayName) &&
                            (options.kind === undefined || options.kind === kind)
                    )
                )
            )
    )
}

export async function updateExternalService(
    gqlClient: GraphQLClient,
    input: UpdateExternalServiceInput
): Promise<void> {
    await lastValueFrom(
        gqlClient
            .mutateGraphQL<UpdateExternalServiceResult, UpdateExternalServiceVariables>(
                gql`
                    mutation UpdateExternalServiceRegression($input: UpdateExternalServiceInput!) {
                        updateExternalService(input: $input) {
                            warning
                        }
                    }
                `,
                { input }
            )
            .pipe(
                map(dataOrThrowErrors),
                tap(({ updateExternalService: { warning } }) => {
                    if (warning) {
                        logger.warn('updateExternalService warning:', warning)
                    }
                })
            )
    )
}

export async function ensureTestExternalService(
    gqlClient: GraphQLClient,
    externalServiceOptions: {
        kind: ExternalServiceKind
        uniqueDisplayName: string
        config: Record<string, any>
        waitForRepos?: string[]
    },
    waitForReposOptions?: WaitForRepoOptions
): Promise<ResourceDestructor> {
    if (!externalServiceOptions.uniqueDisplayName.startsWith('[TEST]')) {
        throw new Error(
            `Test external service name ${JSON.stringify(
                externalServiceOptions.uniqueDisplayName
            )} must start with "[TEST]".`
        )
    }

    const destroy = (): Promise<void> =>
        ensureNoTestExternalServices(gqlClient, { ...externalServiceOptions, deleteIfExist: true })

    const externalServices = await getExternalServices(gqlClient, externalServiceOptions)
    if (externalServices.length > 0) {
        return destroy
    }

    // Add a new external service if one doesn't already exist.
    const input: AddExternalServiceInput = {
        kind: externalServiceOptions.kind,
        displayName: externalServiceOptions.uniqueDisplayName,
        config: JSON.stringify(externalServiceOptions.config),
    }
    await lastValueFrom(addExternalService(input, gqlClient))

    if (externalServiceOptions.waitForRepos && externalServiceOptions.waitForRepos.length > 0) {
        await waitForRepos(gqlClient, externalServiceOptions.waitForRepos, waitForReposOptions)
    }

    return destroy
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export async function deleteUser(
    { requestGraphQL }: Pick<PlatformContext, 'requestGraphQL'>,
    username: string,
    mustAlreadyExist: boolean = true
): Promise<void> {
    let user: UserResult['user'] | null
    try {
        user = await getUser({ requestGraphQL }, username)
    } catch (error) {
        if (mustAlreadyExist) {
            throw error
        } else {
            return
        }
    }

    if (!user) {
        if (mustAlreadyExist) {
            throw new Error(`Fetched user ${username} was null`)
        } else {
            return
        }
    }

    await lastValueFrom(
        requestGraphQL<DeleteUserResult, DeleteUserVariables>({
            request: gql`
                mutation DeleteUser($user: ID!, $hard: Boolean) {
                    deleteUser(user: $user, hard: $hard) {
                        alwaysNil
                    }
                }
            `,
            variables: { hard: false, user: user.id },
            mightContainPrivateInfo: false,
        })
    )
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export async function setUserSiteAdmin(
    gqlClient: GraphQLClient,
    userID: Scalars['ID'],
    siteAdmin: boolean
): Promise<void> {
    await lastValueFrom(
        gqlClient.mutateGraphQL<SetUserIsSiteAdminResult, SetUserIsSiteAdminVariables>(
            gql`
                mutation SetUserIsSiteAdmin($userID: ID!, $siteAdmin: Boolean!) {
                    setUserIsSiteAdmin(userID: $userID, siteAdmin: $siteAdmin) {
                        alwaysNil
                    }
                }
            `,
            { userID, siteAdmin }
        )
    )
}

export async function setTosAccepted(gqlClient: GraphQLClient, userID: Scalars['ID']): Promise<void> {
    await lastValueFrom(
        gqlClient.mutateGraphQL<SetTosAcceptedResult, SetTosAcceptedVariables>(
            gql`
                mutation SetTosAccepted($userID: ID!) {
                    setTosAccepted(userID: $userID) {
                        alwaysNil
                    }
                }
            `,
            { userID }
        )
    )
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export function currentProductVersion(gqlClient: GraphQLClient): Promise<string> {
    return lastValueFrom(
        gqlClient
            .queryGraphQL<SiteProductVersionResult, SiteProductVersionVariables>(
                gql`
                    query SiteProductVersion {
                        site {
                            productVersion
                        }
                    }
                `,
                {}
            )
            .pipe(
                map(dataOrThrowErrors),
                map(({ site }) => site.productVersion)
            )
    )
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export function getViewerSettings({
    requestGraphQL,
}: Pick<PlatformContext, 'requestGraphQL'>): Promise<ViewerSettingsResult['viewerSettings']> {
    return lastValueFrom(
        requestGraphQL<ViewerSettingsResult, ViewerSettingsVariables>({
            request: viewerSettingsQuery,
            variables: {},
            mightContainPrivateInfo: true,
        }).pipe(
            map(dataOrThrowErrors),
            map(data => data.viewerSettings)
        )
    )
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export function deleteOrganization(
    { requestGraphQL }: Pick<PlatformContext, 'requestGraphQL'>,
    organization: Scalars['ID']
): Promise<void> {
    return lastValueFrom(
        requestGraphQL<DeleteOrganizationResult, DeleteOrganizationVariables>({
            request: gql`
                mutation DeleteOrganization($organization: ID!) {
                    deleteOrganization(organization: $organization) {
                        alwaysNil
                    }
                }
            `,
            variables: { organization },
            mightContainPrivateInfo: true,
        }).pipe(
            map(dataOrThrowErrors),
            map(data => {
                if (!data.deleteOrganization) {
                    throw createInvalidGraphQLMutationResponseError('DeleteOrganization')
                }
            })
        )
    )
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export function fetchAllOrganizations(
    { requestGraphQL }: Pick<PlatformContext, 'requestGraphQL'>,
    args: { first?: number; query?: string }
): Observable<OrganizationsConnectionFields> {
    return requestGraphQL<OrganizationsResult, OrganizationsVariables>({
        request: gql`
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
        variables: {
            first: args.first ?? null,
            query: args.query ?? null,
        },
        mightContainPrivateInfo: true,
    }).pipe(
        map(dataOrThrowErrors),
        map(data => data.organizations)
    )
}

interface EventLogger {
    log: (eventLabel: string, eventProperties?: any) => void
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export function createOrganization(
    {
        requestGraphQL,
        eventLogger = { log: () => undefined },
    }: Pick<PlatformContext, 'requestGraphQL'> & {
        eventLogger?: EventLogger
    },
    variables: CreateOrganizationVariables
): Observable<CreateOrganizationResult['createOrganization']> {
    return requestGraphQL<CreateOrganizationResult, CreateOrganizationVariables>({
        request: gql`
            mutation CreateOrganization($name: String!, $displayName: String) {
                createOrganization(name: $name, displayName: $displayName) {
                    id
                    name
                    settingsURL
                }
            }
        `,
        variables,
        mightContainPrivateInfo: false,
    }).pipe(
        mergeMap(({ data, errors }) => {
            if (!data?.createOrganization) {
                eventLogger.log('NewOrgFailed')
                throw createAggregateError(errors)
            }
            eventLogger.log('NewOrgCreated')
            return concat([data.createOrganization])
        })
    )
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export function createUser(
    { requestGraphQL }: Pick<PlatformContext, 'requestGraphQL'>,
    username: string,
    email: string | null
): Promise<CreateUserResult['createUser']> {
    return lastValueFrom(
        requestGraphQL<CreateUserResult, CreateUserVariables>({
            request: gql`
                mutation CreateUser($username: String!, $email: String) {
                    createUser(username: $username, email: $email) {
                        resetPasswordURL
                    }
                }
            `,
            variables: { username, email },
            mightContainPrivateInfo: true,
        }).pipe(
            map(dataOrThrowErrors),
            map(data => data.createUser)
        )
    )
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export function getUser(
    { requestGraphQL }: Pick<PlatformContext, 'requestGraphQL'>,
    username: string
): Promise<UserResult['user']> {
    return lastValueFrom(
        requestGraphQL<UserResult, UserVariables>({
            request: gql`
                query User($username: String!) {
                    user(username: $username) {
                        __typename
                        id
                        username
                        displayName
                        url
                        settingsURL
                        avatarURL
                        viewerCanAdminister
                        siteAdmin
                        createdAt
                        emails {
                            email
                            verified
                        }
                        organizations {
                            nodes {
                                id
                                displayName
                                name
                            }
                        }
                        settingsCascade {
                            subjects {
                                latestSettings {
                                    id
                                    contents
                                }
                            }
                        }
                    }
                }
            `,
            variables: { username },
            mightContainPrivateInfo: true,
        }).pipe(
            map(dataOrThrowErrors),
            map(({ user }) => user)
        )
    )
}

/**
 * TODO(beyang): remove this after the corresponding API in the main code has been updated to use a
 * dependency-injected `requestGraphQL`.
 */
export function addExternalService(
    input: AddExternalServiceInput,
    {
        eventLogger = { log: () => undefined },
        requestGraphQL,
    }: Pick<PlatformContext, 'requestGraphQL'> & { eventLogger: EventLogger }
): Observable<addExternalServiceResult['addExternalService']> {
    return requestGraphQL<addExternalServiceResult, addExternalServiceVariables>({
        request: gql`
            mutation addExternalService($input: AddExternalServiceInput!) {
                addExternalService(input: $input) {
                    id
                    kind
                    displayName
                    warning
                }
            }
        `,
        variables: { input },
        mightContainPrivateInfo: true,
    }).pipe(
        map(({ data, errors }) => {
            if (!data?.addExternalService || (errors && errors.length > 0)) {
                eventLogger.log('AddExternalServiceFailed')
                throw createAggregateError(errors)
            }
            eventLogger.log('AddExternalServiceSucceeded')
            return data.addExternalService
        })
    )
}

const GenericSearchResultInterfaceFragment = gql`
    fragment GenericSearchResultFields on GenericSearchResultInterface {
        label {
            html
        }
        url
        detail {
            html
        }
        matches {
            url
            body {
                text
                html
            }
            highlights {
                line
                character
                length
            }
        }
    }
`

export function search(
    { requestGraphQL }: Pick<PlatformContext, 'requestGraphQL'>,
    query: string,
    version: SearchVersion,
    patternType: SearchPatternType
): Promise<SearchResult['search']> {
    return lastValueFrom(
        requestGraphQL<SearchResult, SearchVariables>({
            request: gql`
                query Search($query: String!, $version: SearchVersion!, $patternType: SearchPatternType!) {
                    search(query: $query, version: $version, patternType: $patternType) {
                        results {
                            __typename
                            limitHit
                            matchCount
                            approximateResultCount
                            missing {
                                name
                            }
                            cloning {
                                name
                            }
                            timedout {
                                name
                            }
                            indexUnavailable
                            dynamicFilters {
                                value
                                label
                                count
                                limitHit
                                kind
                            }
                            results {
                                __typename
                                ... on Repository {
                                    id
                                    name
                                    ...GenericSearchResultFields
                                }
                                ... on FileMatch {
                                    file {
                                        path
                                        url
                                        commit {
                                            oid
                                        }
                                    }
                                    repository {
                                        name
                                        url
                                    }
                                    limitHit
                                    symbols {
                                        name
                                        containerName
                                        url
                                        kind
                                    }
                                    lineMatches {
                                        preview
                                        lineNumber
                                        offsetAndLengths
                                    }
                                }
                                ... on CommitSearchResult {
                                    ...GenericSearchResultFields
                                }
                            }
                            alert {
                                title
                                description
                                proposedQueries {
                                    description
                                    query
                                }
                            }
                            elapsedMilliseconds
                        }
                    }
                }
                ${GenericSearchResultInterfaceFragment}
            `,
            variables: { query, version, patternType },
            mightContainPrivateInfo: false,
        }).pipe(
            map(dataOrThrowErrors),
            map(data => {
                if (!data.search) {
                    throw new Error('no results field in search response')
                }
                return data.search
            })
        )
    )
}

/**
 * Fetches the site and its configuration.
 *
 * @returns Observable that emits the site
 */
export function fetchSiteConfiguration({
    requestGraphQL,
}: Pick<PlatformContext, 'requestGraphQL'>): Observable<SiteResult['site']> {
    return requestGraphQL<SiteResult>({
        request: gql`
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
        `,
        variables: {},
        mightContainPrivateInfo: true,
    }).pipe(
        map(dataOrThrowErrors),
        map(data => data.site)
    )
}

/**
 * Updates the site's configuration.
 *
 * @returns An observable indicating whether or not a service restart is
 * required for the update to be applied.
 */
export function updateSiteConfiguration(
    { requestGraphQL }: Pick<PlatformContext, 'requestGraphQL'>,
    lastID: number,
    input: string
): Promise<boolean> {
    return lastValueFrom(
        requestGraphQL<UpdateSiteConfigurationResult, UpdateSiteConfigurationVariables>({
            request: gql`
                mutation UpdateSiteConfiguration($lastID: Int!, $input: String!) {
                    updateSiteConfiguration(lastID: $lastID, input: $input)
                }
            `,
            variables: { lastID, input },
            mightContainPrivateInfo: true,
        }).pipe(
            map(dataOrThrowErrors),
            map(data => data.updateSiteConfiguration)
        )
    )
}
