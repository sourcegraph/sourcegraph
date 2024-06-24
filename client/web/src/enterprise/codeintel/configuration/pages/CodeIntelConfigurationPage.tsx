import React, { useCallback, useEffect, useMemo, type FunctionComponent } from 'react'

import { useApolloClient } from '@apollo/client'
import {
    mdiAlert,
    mdiCircleOffOutline,
    mdiDatabaseClock,
    mdiDelete,
    mdiDeleteClock,
    mdiEarth,
    mdiLock,
    mdiPencil,
    mdiSourceRepository,
} from '@mdi/js'
import classNames from 'classnames'
import { useLocation, useNavigate } from 'react-router-dom'
import { Subject } from 'rxjs'

import { RepoLink } from '@sourcegraph/shared/src/components/RepoLink'
import { GitObjectType } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps, TelemetryService } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Badge, Button, Container, ErrorAlert, H3, Icon, Link, PageHeader, Text, Tooltip } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../../auth'
import { FilteredConnection, type FilteredConnectionQueryArguments } from '../../../../components/FilteredConnection'
import { PageTitle } from '../../../../components/PageTitle'
import type { CodeIntelligenceConfigurationPolicyFields } from '../../../../graphql-operations'
import { CreatePolicyButtons } from '../components/CreatePolicyButtons'
import { Duration } from '../components/Duration'
import { EmptyPoliciesList } from '../components/EmptyPoliciesList'
import { FlashMessage } from '../components/FlashMessage'
import { queryPolicies as defaultQueryPolicies } from '../hooks/queryPolicies'
import { useDeletePolicies } from '../hooks/useDeletePolicies'
import { hasGlobalPolicyViolation } from '../shared'

import styles from './CodeIntelConfigurationPage.module.scss'

export interface CodeIntelConfigurationPageProps extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    queryPolicies?: typeof defaultQueryPolicies
    repo?: { id: string; name: string }
    indexingEnabled?: boolean
    telemetryService: TelemetryService
}

export const CodeIntelConfigurationPage: FunctionComponent<CodeIntelConfigurationPageProps> = ({
    authenticatedUser,
    queryPolicies = defaultQueryPolicies,
    repo,
    indexingEnabled = window.context?.codeIntelAutoIndexingEnabled,
    telemetryService,
    telemetryRecorder,
}) => {
    useEffect(() => {
        telemetryService.logViewEvent('CodeIntelConfiguration')
        if (repo) {
            telemetryRecorder.recordEvent('repo.codeIntel.configuration', 'view')
        } else {
            telemetryRecorder.recordEvent('admin.codeIntel.configuration', 'view')
        }
    }, [telemetryService, telemetryRecorder, repo])

    const navigate = useNavigate()
    const location = useLocation()
    const updates = useMemo(() => new Subject<void>(), [])

    const apolloClient = useApolloClient()
    const queryDefaultPoliciesCallback = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryPolicies({ ...args, repository: repo?.id, protected: true }, apolloClient),
        [queryPolicies, repo?.id, apolloClient]
    )
    const queryCustomPoliciesCallback = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryPolicies({ ...args, repository: repo?.id, protected: false }, apolloClient),
        [queryPolicies, repo?.id, apolloClient]
    )

    const { handleDeleteConfig, isDeleting, deleteError } = useDeletePolicies()

    const onDelete = useCallback(
        async (id: string, name: string) => {
            if (!window.confirm(`Delete policy ${name}?`)) {
                return
            }

            return handleDeleteConfig({
                variables: { id },
            }).then(() => {
                // Force update of filtered connection
                updates.next()

                navigate(
                    {
                        pathname: './',
                    },
                    {
                        relative: 'path',
                        state: { modal: 'SUCCESS', message: `Configuration policy ${name} has been deleted.` },
                    }
                )
            })
        },
        [handleDeleteConfig, updates, navigate]
    )

    return (
        <>
            <PageTitle
                title={
                    repo
                        ? 'Code graph data configuration policies for repository'
                        : 'Global code graph data configuration policies'
                }
            />
            <PageHeader
                headingElement="h2"
                path={[
                    {
                        text: repo ? (
                            <>
                                Code graph data configuration for <RepoLink repoName={repo.name} to={null} />
                            </>
                        ) : (
                            'Global code graph data configuration'
                        ),
                    },
                ]}
                description={
                    <>
                        Rules that control{indexingEnabled && <> auto-indexing and</>} data retention behavior of code
                        graph data.
                    </>
                }
                actions={authenticatedUser?.siteAdmin && <CreatePolicyButtons repo={repo} />}
                className="mb-3"
            />

            {deleteError && <ErrorAlert prefix="Error deleting configuration policy" error={deleteError} />}
            {location.state && <FlashMessage state={location.state.modal} message={location.state.message} />}

            {authenticatedUser?.siteAdmin && repo && (
                <Container className="mb-2">
                    View <Link to="/site-admin/code-graph/configuration">additional configuration policies</Link> that
                    do not affect this repository.
                </Container>
            )}

            <Container className="mb-3 pb-3">
                <H3>Custom policies</H3>
                <FilteredConnection<
                    CodeIntelligenceConfigurationPolicyFields,
                    Omit<UnprotectedPoliciesNodeProps, 'node'>
                >
                    listComponent="div"
                    listClassName={classNames(styles.grid, 'mb-3')}
                    showMoreClassName="mb-0"
                    noun="configuration policy"
                    pluralNoun="configuration policies"
                    nodeComponent={PoliciesNode}
                    nodeComponentProps={{ isDeleting, onDelete, indexingEnabled }}
                    queryConnection={queryCustomPoliciesCallback}
                    cursorPaging={true}
                    filters={[
                        {
                            id: 'filters',
                            label: 'Show',
                            type: 'select',
                            values: [
                                {
                                    label: 'All policies',
                                    value: 'all',
                                    args: {},
                                },
                                ...(indexingEnabled
                                    ? [
                                          {
                                              label: 'Policies affecting auto-indexing',
                                              value: 'indexing',
                                              args: { forIndexing: true },
                                          },
                                      ]
                                    : []),
                                ...[
                                    {
                                        label: 'Policies affecting data retention',
                                        value: 'data-retention',
                                        args: { forDataRetention: true },
                                    },
                                ],
                            ],
                        },
                    ]}
                    inputClassName="ml-2 flex-1"
                    emptyElement={<EmptyPoliciesList repo={repo} showCta={authenticatedUser?.siteAdmin} />}
                    updates={updates}
                />
            </Container>

            <Container className="mb-3">
                <H3>Default policies</H3>
                <FilteredConnection<CodeIntelligenceConfigurationPolicyFields, Omit<PoliciesNodeProps, 'node'>>
                    listComponent="div"
                    listClassName={classNames(styles.grid, 'mb-3')}
                    noun="configuration policy"
                    pluralNoun="configuration policies"
                    nodeComponent={PoliciesNode}
                    nodeComponentProps={{ indexingEnabled }}
                    queryConnection={queryDefaultPoliciesCallback}
                    emptyElement={<EmptyPoliciesList repo={repo} />}
                    hideSearch={true}
                    summaryClassName="d-none"
                    useURLQuery={false}
                />
            </Container>
        </>
    )
}

interface ProtectedPoliciesNodeProps {
    node: CodeIntelligenceConfigurationPolicyFields
    indexingEnabled?: boolean
}

export interface UnprotectedPoliciesNodeProps {
    node: CodeIntelligenceConfigurationPolicyFields
    isDeleting: boolean
    onDelete: (id: string, name: string) => Promise<void>
    indexingEnabled?: boolean
}

type PoliciesNodeProps = ProtectedPoliciesNodeProps | UnprotectedPoliciesNodeProps

export const PoliciesNode: FunctionComponent<React.PropsWithChildren<PoliciesNodeProps>> = ({
    node: policy,
    indexingEnabled = false,
    ...props
}) => (
    <>
        <span className={styles.separator} />

        <div className={classNames(styles.name, 'd-flex flex-column')}>
            <PolicyDescription policy={policy} indexingEnabled={indexingEnabled} />
            <RepositoryAndGitObjectDescription policy={policy} />
            {policy.indexingEnabled && indexingEnabled && <AutoIndexingDescription policy={policy} />}
            {policy.retentionEnabled && <RetentionDescription policy={policy} />}
        </div>

        <div className="h-100">
            <Link
                to={
                    policy.repository === null
                        ? `/site-admin/code-graph/configuration/${policy.id}`
                        : `/${policy.repository.name}/-/code-graph/configuration/${policy.id}`
                }
            >
                <Tooltip content="Edit this policy">
                    <Icon svgPath={mdiPencil} inline={true} aria-label="Edit" />
                </Tooltip>
            </Link>
        </div>

        <div className="h-100">
            {!policy.protected && 'onDelete' in props && 'isDeleting' in props && (
                <Button
                    aria-label="Delete the configuration policy"
                    variant="icon"
                    onClick={() => props.onDelete(policy.id, policy.name)}
                    disabled={props.isDeleting}
                >
                    <Tooltip content="Delete this policy">
                        <Icon className="text-danger" aria-label="Delete this policy" svgPath={mdiDelete} />
                    </Tooltip>
                </Button>
            )}
            {policy.protected && (
                <Tooltip content="This configuration policy is protected. Protected configuration policies may not be deleted and only the retention duration and indexing options are editable.">
                    <Icon
                        svgPath={mdiLock}
                        inline={true}
                        aria-label="This configuration policy is protected. Protected configuration policies may not be deleted and only the retention duration and indexing options are editable."
                        className="mr-2"
                    />
                </Tooltip>
            )}
        </div>
    </>
)

interface PolicyDescriptionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    indexingEnabled?: boolean
    allowGlobalPolicies?: boolean
}

const PolicyDescription: FunctionComponent<PolicyDescriptionProps> = ({
    policy,
    indexingEnabled = false,
    allowGlobalPolicies = window.context?.codeIntelAutoIndexingAllowGlobalPolicies,
}) => (
    <div className={styles.policyDescription}>
        <Link
            to={
                policy.repository === null
                    ? `/site-admin/code-graph/configuration/${policy.id}`
                    : `/${policy.repository.name}/-/code-graph/configuration/${policy.id}`
            }
        >
            <Text weight="bold" className="mb-0">
                {policy.name}
            </Text>
        </Link>

        {!policy.retentionEnabled && !(indexingEnabled && policy.indexingEnabled) && (
            <Tooltip content="This policy has no enabled behaviors.">
                <Icon
                    svgPath={mdiCircleOffOutline}
                    inline={true}
                    aria-label="This policy has no enabled behaviors."
                    className="ml-2"
                />
            </Tooltip>
        )}

        {indexingEnabled && !allowGlobalPolicies && hasGlobalPolicyViolation(policy) && (
            <Tooltip content="This Sourcegraph instance has disabled global policies for auto-indexing.">
                <Icon
                    svgPath={mdiAlert}
                    inline={true}
                    aria-label="This Sourcegraph instance has disabled global policies for auto-indexing."
                    className="text-warning ml-2"
                />
            </Tooltip>
        )}
    </div>
)

interface RepositoryAndGitObjectDescriptionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
}

const RepositoryAndGitObjectDescription: FunctionComponent<RepositoryAndGitObjectDescriptionProps> = ({ policy }) => (
    <div>
        {!policy.repository ? (
            <Tooltip content="This policy may apply to more than one repository.">
                <Icon
                    svgPath={mdiEarth}
                    inline={true}
                    aria-label="This policy may apply to more than one repository."
                    className="mr-2"
                />
            </Tooltip>
        ) : (
            <Tooltip content="This policy applies to a specific repository.">
                <Icon
                    svgPath={mdiSourceRepository}
                    inline={true}
                    aria-label="This policy applies to a specific repository."
                    className="mr-2"
                />
            </Tooltip>
        )}

        <span>
            Applies to <GitObjectDescription policy={policy} /> of <RepositoryDescription policy={policy} />.
        </span>
    </div>
)

interface GitObjectDescriptionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
}

const GitObjectDescription: FunctionComponent<GitObjectDescriptionProps> = ({ policy }) => {
    if (policy.type === GitObjectType.GIT_COMMIT) {
        if (policy.pattern === 'HEAD') {
            return (
                <>
                    <Badge variant="outlineSecondary">HEAD</Badge> (tip of default branch)
                </>
            )
        }

        return (
            <Badge variant="outlineSecondary">
                commit <span className="text-monospace">{policy.pattern}</span>
            </Badge>
        )
    }

    if (policy.type === GitObjectType.GIT_TREE) {
        if (policy.pattern !== '*') {
            return (
                <Badge variant="outlineSecondary">
                    branches matching <span className="text-monospace">{policy.pattern}</span>
                </Badge>
            )
        }

        return <Badge variant="outlineSecondary">all branches</Badge>
    }

    if (policy.type === GitObjectType.GIT_TAG) {
        if (policy.pattern !== '*') {
            return (
                <Badge variant="outlineSecondary">
                    tags matching <span className="text-monospace">{policy.pattern}</span>
                </Badge>
            )
        }

        return <Badge variant="outlineSecondary">all tags</Badge>
    }

    return <></>
}

interface RepositoryDescriptionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
}

const RepositoryDescription: FunctionComponent<RepositoryDescriptionProps> = ({ policy }) => {
    if (policy.repository) {
        return (
            <Badge variant="outlineSecondary">
                <span className="text-monospace">{policy.repository.name}</span>
            </Badge>
        )
    }

    if (policy.repositoryPatterns) {
        return (
            <Badge variant="outlineSecondary">
                repositories{' '}
                {policy.repositoryPatterns.map((pattern, index) => (
                    <React.Fragment key={pattern}>
                        {index !== 0 && (index === (policy.repositoryPatterns || []).length - 1 ? <>, or </> : <>, </>)}
                        <span key={pattern} className="text-monospace">
                            {pattern}
                        </span>
                    </React.Fragment>
                ))}
            </Badge>
        )
    }

    return <Badge variant="outlineSecondary">all repositories</Badge>
}

interface AutoIndexingDescriptionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
}

const AutoIndexingDescription: FunctionComponent<AutoIndexingDescriptionProps> = ({ policy }) => (
    <div>
        <Tooltip content="This policy affects auto-indexing.">
            <Icon
                svgPath={mdiDatabaseClock}
                inline={true}
                aria-label="This policy affects auto-indexing."
                className="mr-2"
            />
        </Tooltip>

        <span>
            Index{' '}
            {policy.type === GitObjectType.GIT_TREE ? (
                <>
                    <Badge variant="outlineSecondary">
                        {policy.indexIntermediateCommits ? 'all commits' : 'the tip'}
                    </Badge>{' '}
                    of matching branches
                </>
            ) : (
                'all matching commits'
            )}
            {policy.indexCommitMaxAgeHours && (
                <>
                    {' '}
                    younger than{' '}
                    <Badge variant="outlineSecondary">
                        <Duration hours={policy.indexCommitMaxAgeHours} />
                    </Badge>
                </>
            )}{' '}
            .
        </span>
    </div>
)

interface RetentionDescriptionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
}

const RetentionDescription: FunctionComponent<RetentionDescriptionProps> = ({ policy }) => (
    <div>
        <Tooltip content="This policy affects data retention.">
            <Icon
                svgPath={mdiDeleteClock}
                inline={true}
                aria-label="This policy affects data retention."
                className="mr-2"
            />
        </Tooltip>

        <span>
            Keep precise indexes providing intelligence for{' '}
            {policy.type === GitObjectType.GIT_TREE ? (
                <>
                    <Badge variant="outlineSecondary">
                        {policy.retainIntermediateCommits ? 'any commit' : 'the tip'}
                    </Badge>{' '}
                    of matching branches
                </>
            ) : (
                <>matching commits</>
            )}{' '}
            <Badge variant="outlineSecondary">
                {policy.retentionDurationHours ? (
                    <>
                        for <Duration hours={policy.retentionDurationHours} /> after upload
                    </>
                ) : (
                    'indefinitely'
                )}
            </Badge>
            .
        </span>
    </div>
)
