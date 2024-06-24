import { useCallback, useEffect, useMemo, useState, type FunctionComponent } from 'react'

import type { ApolloError } from '@apollo/client'
import { mdiDelete, mdiGraveStone } from '@mdi/js'
import classNames from 'classnames'
import { debounce } from 'lodash'
import { useLocation, useNavigate, useParams } from 'react-router-dom'

import { Toggle } from '@sourcegraph/branded/src/components/Toggle'
import { useLazyQuery } from '@sourcegraph/http-client'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { displayRepoName, RepoLink } from '@sourcegraph/shared/src/components/RepoLink'
import { GitObjectType } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Alert,
    Badge,
    Button,
    Checkbox,
    Code,
    Container,
    ErrorAlert,
    Icon,
    Input,
    Label,
    Link,
    LoadingSpinner,
    PageHeader,
    Select,
    Text,
    Tooltip,
} from '@sourcegraph/wildcard'

import { PageTitle } from '../../../../components/PageTitle'
import type {
    CodeIntelligenceConfigurationPolicyFields,
    PreviewGitObjectFilterResult,
    PreviewGitObjectFilterVariables,
} from '../../../../graphql-operations'
import { DurationSelect, maxDuration } from '../components/DurationSelect'
import { FlashMessage } from '../components/FlashMessage'
import { RepositoryPatternList } from '../components/RepositoryPatternList'
import { nullPolicy } from '../hooks/types'
import { useDeletePolicies } from '../hooks/useDeletePolicies'
import { usePolicyConfigurationByID } from '../hooks/usePolicyConfigurationById'
import {
    convertGitObjectFilterResult,
    PREVIEW_GIT_OBJECT_FILTER,
    type GitObjectPreviewResult,
} from '../hooks/usePreviewGitObjectFilter'
import { useSavePolicyConfiguration } from '../hooks/useSavePolicyConfiguration'
import { hasGlobalPolicyViolation } from '../shared'

import styles from './CodeIntelConfigurationPolicyPage.module.scss'

const DEBOUNCED_WAIT = 250

const MS_IN_HOURS = 60 * 60 * 1000

export interface CodeIntelConfigurationPolicyPageProps extends TelemetryProps, TelemetryV2Props {
    repo?: { id: string; name: string }
    authenticatedUser: AuthenticatedUser | null
    indexingEnabled?: boolean
    allowGlobalPolicies?: boolean
}

type PolicyUpdater = <K extends keyof CodeIntelligenceConfigurationPolicyFields>(updates: {
    [P in K]: CodeIntelligenceConfigurationPolicyFields[P]
}) => void

export const CodeIntelConfigurationPolicyPage: FunctionComponent<CodeIntelConfigurationPolicyPageProps> = ({
    repo,
    authenticatedUser,
    indexingEnabled = window.context?.codeIntelAutoIndexingEnabled,
    allowGlobalPolicies = window.context?.codeIntelAutoIndexingAllowGlobalPolicies,
    telemetryService,
    telemetryRecorder,
}) => {
    const navigate = useNavigate()
    const location = useLocation()
    const { id } = useParams<{ id: string }>()

    useEffect(() => {
        telemetryService.logViewEvent('CodeIntelConfigurationPolicy')
        telemetryRecorder.recordEvent(getViewEventFeatureName(!!repo), 'view')
    }, [telemetryService, telemetryRecorder, repo])

    // Handle local policy state
    const [policy, setPolicy] = useState<CodeIntelligenceConfigurationPolicyFields | undefined>()
    const updatePolicy: PolicyUpdater = updates => setPolicy(policy => ({ ...(policy || nullPolicy), ...updates }))

    // Handle remote policy state
    const { policyConfig, loadingPolicyConfig, policyConfigError } = usePolicyConfigurationByID(id!)
    const [saved, setSaved] = useState<CodeIntelligenceConfigurationPolicyFields>()
    const { savePolicyConfiguration, isSaving, savingError } = useSavePolicyConfiguration(policy?.id === '')
    const { handleDeleteConfig, isDeleting, deleteError } = useDeletePolicies()

    const savePolicyConfig = useCallback(async () => {
        if (!policy) {
            return
        }

        const variables = repo?.id ? { ...policy, repositoryId: repo.id ?? null } : { ...policy }
        variables.pattern = variables.type === GitObjectType.GIT_COMMIT ? 'HEAD' : variables.pattern

        return savePolicyConfiguration({ variables })
            .then(() =>
                navigate(
                    {
                        pathname: '..',
                    },
                    {
                        state: { modal: 'SUCCESS', message: `Configuration for policy ${policy.name} has been saved.` },
                        relative: 'path',
                    }
                )
            )
            .catch((error: ApolloError) =>
                navigate(
                    {},
                    {
                        state: {
                            modal: 'ERROR',
                            message: `There was an error while saving policy: ${policy.name}. See error: ${error.message}`,
                        },
                    }
                )
            )
    }, [policy, repo, savePolicyConfiguration, navigate])

    const handleDelete = useCallback(
        async (id: string, name: string) => {
            if (!policy || !window.confirm(`Delete policy ${name}?`)) {
                return
            }

            return handleDeleteConfig({
                variables: { id },
                update: cache => cache.modify({ fields: { node: () => {} } }),
            }).then(() =>
                navigate(
                    {
                        pathname: '..',
                    },
                    {
                        state: { modal: 'SUCCESS', message: `Configuration policy ${name} has been deleted.` },
                        relative: 'path',
                    }
                )
            )
        },
        [policy, handleDeleteConfig, navigate]
    )

    // Set initial policy state
    useEffect(() => {
        const urlType = new URLSearchParams(location.search).get('type')
        const defaultTypes =
            urlType === 'branch'
                ? { type: GitObjectType.GIT_TREE, pattern: '*' }
                : urlType === 'tag'
                ? { type: GitObjectType.GIT_TAG, pattern: '*' }
                : { type: GitObjectType.GIT_COMMIT, retentionEnabled: true }

        const repoDefaults = repo ? { repository: repo } : {}
        const typeDefaults = policyConfig?.type === GitObjectType.GIT_UNKNOWN ? defaultTypes : {}
        const configWithDefaults = policyConfig && { ...policyConfig, ...repoDefaults, ...typeDefaults }

        setPolicy(configWithDefaults)
        setSaved(configWithDefaults)
    }, [policyConfig, repo, location.search])

    if (loadingPolicyConfig) {
        return <LoadingSpinner />
    }

    if (policyConfigError || policy === undefined) {
        return <ErrorAlert prefix="Error fetching configuration policy" error={policyConfigError} />
    }

    return (
        <>
            <PageTitle
                title={
                    repo
                        ? 'Code graph configuration policy for repository'
                        : 'Global code graph data configuration policy'
                }
            />
            <PageHeader
                headingElement="h2"
                path={[
                    {
                        text: repo ? (
                            <>
                                {policy?.id === '' ? 'Create a new' : 'Update a'} code graph configuration policy for{' '}
                                <RepoLink repoName={repo.name} to={null} />
                            </>
                        ) : (
                            <>
                                {policy?.id === '' ? 'Create a new' : 'Update a'} global code graph configuration policy
                            </>
                        ),
                    },
                ]}
                description={
                    <>
                        Rules that control{indexingEnabled && <> auto-indexing and</>} data retention behavior of code
                        graph data.
                    </>
                }
                className="mb-3"
            />
            {!policy.id && authenticatedUser?.siteAdmin && <NavigationCTA repo={repo} />}={true}{' '}
            {savingError && <ErrorAlert prefix="Error saving configuration policy" error={savingError} />}
            {deleteError && <ErrorAlert prefix="Error deleting configuration policy" error={deleteError} />}
            {location.state && <FlashMessage state={location.state.modal} message={location.state.message} />}
            {policy.protected && (
                <Alert variant="info">
                    This configuration policy is protected. Protected configuration policies may not be deleted and only
                    the retention duration and indexing options are editable.
                </Alert>
            )}
            <Container className="container form">
                <NameSettingsSection policy={policy} updatePolicy={updatePolicy} repo={repo} />
                <GitConfiguration policy={policy} updatePolicy={updatePolicy} repo={repo} />
                {!policy.repository && <RepositorySettingsSection policy={policy} updatePolicy={updatePolicy} />}

                {indexingEnabled && <IndexSettingsSection policy={policy} updatePolicy={updatePolicy} repo={repo} />}
                <RetentionSettingsSection policy={policy} updatePolicy={updatePolicy} />

                <div className="mt-4">
                    <Button
                        type="submit"
                        variant="primary"
                        onClick={savePolicyConfig}
                        disabled={
                            isSaving ||
                            isDeleting ||
                            !validatePolicy(policy, allowGlobalPolicies) ||
                            comparePolicies(policy, saved)
                        }
                    >
                        {!isSaving && <>{policy.id === '' ? 'Create' : 'Update'} policy</>}
                        {isSaving && (
                            <>
                                <LoadingSpinner /> Saving...
                            </>
                        )}
                    </Button>

                    <Button
                        type="button"
                        className="ml-2"
                        variant="secondary"
                        onClick={() => navigate('..', { relative: 'path' })}
                        disabled={isSaving}
                    >
                        Cancel
                    </Button>

                    {!policy.protected && policy.id !== '' && (
                        <Tooltip
                            content={`Deleting this policy may immediately affect data retention${
                                indexingEnabled ? ' and auto-indexing' : ''
                            }.`}
                        >
                            <Button
                                type="button"
                                className="float-right"
                                variant="danger"
                                disabled={isSaving || isDeleting}
                                onClick={() => handleDelete(policy.id, policy.name)}
                            >
                                {!isDeleting && (
                                    <>
                                        <Icon aria-hidden={true} svgPath={mdiDelete} /> Delete policy
                                    </>
                                )}
                                {isDeleting && (
                                    <>
                                        <LoadingSpinner /> Deleting...
                                    </>
                                )}
                            </Button>
                        </Tooltip>
                    )}
                </div>
                {!allowGlobalPolicies && hasGlobalPolicyViolation(policy) && (
                    <Alert variant="warning" className="mt-2">
                        This Sourcegraph instance has disabled global policies for auto-indexing. Create a more
                        constrained policy targeting an explicit set of repositories to enable this policy.{' '}
                        <Link
                            to="/help/code_navigation/how-to/enable_auto_indexing#configure-auto-indexing-policies"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            See autoindexing docs.
                        </Link>
                    </Alert>
                )}
            </Container>
        </>
    )
}

interface NavigationCTAProps {
    repo?: { id: string; name: string }
}

const NavigationCTA: FunctionComponent<NavigationCTAProps> = ({ repo }) => (
    <Container className="mb-2">
        {repo ? (
            <>
                Alternatively,{' '}
                <Link to="/site-admin/code-graph/configuration/new">create global configuration policy</Link> that
                applies to more than this repository.
            </>
        ) : (
            <>
                To create a policy that applies to a particular repository, visit that repository's code graph settings.
            </>
        )}
    </Container>
)

interface NameSettingsSectionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    updatePolicy: PolicyUpdater
    repo?: { id: string; name: string }
}

const NameSettingsSection: FunctionComponent<NameSettingsSectionProps> = ({ repo, policy, updatePolicy }) => (
    <div className="form-group">
        <div className="input-group">
            <Input
                id="name"
                label="Policy name"
                className={styles.input}
                value={policy.name}
                onChange={({ target: { value: name } }) => updatePolicy({ name })}
                disabled={policy.protected}
                required={true}
                error={policy.name === '' ? 'Please supply a value' : undefined}
                placeholder={`Custom ${!repo ? 'global ' : ''}${
                    policy.indexingEnabled ? 'indexing ' : policy.retentionEnabled ? 'retention ' : ''
                }policy${repo ? ` for ${displayRepoName(repo.name)}` : ''}`}
            />
        </div>
    </div>
)

const DEFAULT_GIT_OBJECT_FETCH_LIMIT = 15

interface GitConfigurationProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    updatePolicy: PolicyUpdater
    repo?: { id: string; name: string }
}

const GitConfiguration: FunctionComponent<GitConfigurationProps> = ({ policy, updatePolicy, repo }) => {
    const [gitObjectFetchLimit, setGitObjectFetchLimit] = useState(DEFAULT_GIT_OBJECT_FETCH_LIMIT)

    // updateGitPreview is called only from the useEffect below, which guarantees that repo and policy
    // are both non-nil (so none of the details in the following variable ist should ever be exercised).
    const [updateGitPreview, { data: preview, loading: previewLoading, error: previewError }] = useLazyQuery<
        PreviewGitObjectFilterResult,
        PreviewGitObjectFilterVariables
    >(PREVIEW_GIT_OBJECT_FILTER, {
        variables: {
            id: repo?.id || '',
            type: policy?.type || GitObjectType.GIT_UNKNOWN,
            pattern: policy?.pattern || '',
            countObjectsYoungerThanHours: policy?.indexCommitMaxAgeHours || null,
            first: gitObjectFetchLimit,
        },
    })

    useEffect(() => {
        if (repo && policy?.type) {
            // Update git preview on policy detail changes
            updateGitPreview({}).catch(() => {})
        }
    }, [repo, updateGitPreview, policy?.type, policy?.pattern, policy?.indexCommitMaxAgeHours])

    return (
        <>
            <GitObjectSettingsSection
                policy={policy}
                updatePolicy={updatePolicy}
                repo={repo}
                previewLoading={previewLoading}
                previewError={previewError}
                preview={convertGitObjectFilterResult(preview)}
            />
            <GitObjectPreview
                policy={policy}
                preview={convertGitObjectFilterResult(preview)}
                updateCount={setGitObjectFetchLimit}
            />
        </>
    )
}

interface GitObjectSettingsSectionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    updatePolicy: PolicyUpdater
    repo?: { id: string; name: string }
    previewLoading: boolean
    previewError?: ApolloError
    preview?: GitObjectPreviewResult
}

const GitObjectSettingsSection: FunctionComponent<GitObjectSettingsSectionProps> = ({
    policy,
    updatePolicy,
    repo,
    previewError,
    previewLoading,
    preview,
}) => {
    const [localGitPattern, setLocalGitPattern] = useState('')
    useEffect(() => policy && setLocalGitPattern(policy.pattern), [policy])
    const debouncedSetGitPattern = useMemo(
        () => debounce(pattern => updatePolicy({ ...(policy || nullPolicy), pattern }), DEBOUNCED_WAIT),
        [policy, updatePolicy]
    )

    return (
        <div className="form-group">
            <Label className="d-inline" id="git-type-label">
                Which{' '}
                {policy.type === GitObjectType.GIT_COMMIT
                    ? 'commits'
                    : policy.type === GitObjectType.GIT_TREE
                    ? 'branches'
                    : policy.type === GitObjectType.GIT_TAG
                    ? 'tags'
                    : ''}{' '}
                match this policy?
            </Label>
            <Text size="small" className="text-muted mb-2">
                Configuration policies apply to code intelligence data for specific revisions of{' '}
                {repo ? 'this repository' : 'matching repositories'}. Specify branches or tags using a{' '}
                <Link to="https://github.com/gobwas/glob#example">glob pattern</Link>.
            </Text>

            <div className="input-group">
                <Select
                    id="git-type"
                    aria-labelledby="git-type-label"
                    className={styles.input}
                    value={policy.type}
                    disabled={policy.protected}
                    onChange={({ target: { value } }) => {
                        const type = value as GitObjectType

                        if (type === GitObjectType.GIT_COMMIT) {
                            updatePolicy({
                                type,
                                pattern: '',
                                retainIntermediateCommits: false,
                                indexIntermediateCommits: false,
                            })
                        } else {
                            updatePolicy({
                                type,
                                pattern: policy.type === GitObjectType.GIT_COMMIT ? '*' : policy.pattern,
                            })
                        }
                    }}
                >
                    <option value={GitObjectType.GIT_COMMIT}>HEAD (tip of default branch)</option>
                    <option value={GitObjectType.GIT_TREE}>Branches</option>
                    <option value={GitObjectType.GIT_TAG}>Tags</option>
                </Select>

                {(policy.type === GitObjectType.GIT_TAG || policy.type === GitObjectType.GIT_TREE) && (
                    <>
                        <div className="input-group-prepend ml-2">
                            <span className="input-group-text">matching</span>
                        </div>

                        <Input
                            id="pattern"
                            inputClassName="text-monospace"
                            value={localGitPattern}
                            onChange={({ target: { value } }) => {
                                setLocalGitPattern(value)
                                debouncedSetGitPattern(value)
                            }}
                            placeholder={policy.type === GitObjectType.GIT_TAG ? 'v*' : 'feat/*'}
                            disabled={policy.protected}
                            required={true}
                            status={previewLoading ? 'loading' : undefined}
                        />
                    </>
                )}
            </div>

            {(policy.type === GitObjectType.GIT_TAG || policy.type === GitObjectType.GIT_TREE) && (
                <>
                    <div className="text-right">
                        {policy.pattern === '' && <small className="text-danger">Please supply a value.</small>}
                    </div>

                    {policy.repository &&
                        policy.pattern !== '' &&
                        (previewError ? (
                            <ErrorAlert
                                prefix="Error fetching matching git objects"
                                error={previewError}
                                className="mt-2"
                            />
                        ) : (
                            <div className="text-right">
                                {preview && preview.preview.length === 0 && (
                                    <small className="text-warning">
                                        This pattern does not match any{' '}
                                        {policy.type === GitObjectType.GIT_TAG ? 'tags' : 'branches'}.
                                    </small>
                                )}
                            </div>
                        ))}
                </>
            )}
        </div>
    )
}

interface GitObjectPreviewProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    preview?: GitObjectPreviewResult
    updateCount: (count: number) => void
}

const getGitObjectWording = (totalCountYounger: number, totalCount: number): string => {
    if (totalCountYounger === 0) {
        return 'none of them qualify for auto-indexing'
    }

    if (totalCountYounger < totalCount) {
        return `only ${totalCountYounger} of them qualify for auto-indexing`
    }

    if (totalCount === 1) {
        // Avoid saying "all" if we only have 1 object.
        return 'it qualifies for auto-indexing'
    }

    return 'all of them qualify for auto-indexing'
}

const GitObjectPreview: FunctionComponent<GitObjectPreviewProps> = ({ policy, preview, updateCount }) => {
    if (policy.repository && policy.pattern !== '' && preview && preview.preview.length > 0) {
        // Limit fetching more than 1000 objects
        const nextFetchLimit = Math.min(preview.totalCount, 1000)

        return (
            <div className="form-group">
                <div className="d-flex justify-content-between">
                    <span>
                        {preview.totalCount === 1 ? (
                            <>
                                {preview.totalCount} {policy.type === GitObjectType.GIT_TAG ? 'tag' : 'branch'} matches
                            </>
                        ) : (
                            <>
                                {preview.totalCount} {policy.type === GitObjectType.GIT_TAG ? 'tags' : 'branches'} match
                            </>
                        )}{' '}
                        this policy
                        {preview.totalCountYoungerThanThreshold !== null && (
                            <strong>
                                , {getGitObjectWording(preview.totalCountYoungerThanThreshold, preview.totalCount)}
                            </strong>
                        )}
                        {preview.preview.length < preview.totalCount && <> (showing only {preview.preview.length})</>}:
                    </span>
                    {preview.preview.length < preview.totalCount && (
                        <Button variant="link" className="p-0" onClick={() => updateCount(preview.totalCount)}>
                            Show {nextFetchLimit === preview.totalCount && 'all '}
                            {nextFetchLimit} {policy.type === GitObjectType.GIT_TAG ? 'tags' : 'branches'}
                        </Button>
                    )}
                </div>

                <ul className={classNames('list-group', styles.list)}>
                    {preview.preview.map(tag => (
                        <li key={tag.name} className="list-group-item">
                            <span>
                                {policy.repository !== null && (
                                    <Link
                                        to={`/${policy.repository.name}/-/commit/${tag.rev}`}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                    >
                                        <Code>{tag.rev.slice(0, 7)}</Code>
                                    </Link>
                                )}
                            </span>
                            <Badge variant="info" className="ml-2">
                                {tag.name}
                            </Badge>

                            {policy.indexingEnabled &&
                                policy.indexCommitMaxAgeHours !== null &&
                                (new Date().getTime() - new Date(tag.committedAt).getTime()) / MS_IN_HOURS >
                                    policy.indexCommitMaxAgeHours && (
                                    <span className="float-right text-muted">
                                        <Tooltip content="This commit is too old to be auto-indexed by this policy.">
                                            <Icon
                                                aria-label="This commit is too old to be auto-indexed by this policy."
                                                svgPath={mdiGraveStone}
                                            />
                                        </Tooltip>
                                    </span>
                                )}
                        </li>
                    ))}
                </ul>
            </div>
        )
    }

    return null
}

interface RepositorySettingsSectionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    updatePolicy: PolicyUpdater
}

const RepositorySettingsSection: FunctionComponent<RepositorySettingsSectionProps> = ({ policy, updatePolicy }) => (
    <div className="form-group">
        <Label className="mb-0">Which repositories match this policy?</Label>
        <Text size="small" className="text-muted mb-2">
            Configuration policies can apply to one, a set, or to all repositories on a Sourcegraph instance. Specify a
            set of repositories using a <Link to="https://github.com/gobwas/glob#example">glob pattern</Link>.
        </Text>
        {!policy.repositoryPatterns || policy.repositoryPatterns.length === 0 ? (
            <Alert variant="info" className="d-flex justify-content-between align-items-center">
                <div>
                    <Text weight="medium" className="mb-0">
                        This policy applies to{' '}
                        <Text weight="bold" className="d-inline">
                            all repositories
                        </Text>{' '}
                        on this Sourcegraph instance
                    </Text>
                    {!policy.protected && (
                        <Text size="small" className="text-muted mb-0">
                            Add a repository pattern if you wish to limit the number of repositories with auto indexing.
                        </Text>
                    )}
                </div>
                {!policy.protected && (
                    <Button variant="primary" onClick={() => updatePolicy({ repositoryPatterns: ['*'] })}>
                        Add repository pattern
                    </Button>
                )}
            </Alert>
        ) : (
            <RepositoryPatternList
                repositoryPatterns={policy.repositoryPatterns}
                setRepositoryPatterns={updater =>
                    updatePolicy({
                        repositoryPatterns: updater((policy || nullPolicy).repositoryPatterns),
                    })
                }
            />
        )}
    </div>
)

interface IndexSettingsSectionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    updatePolicy: PolicyUpdater
    repo?: { id: string; name: string }
}

const IndexSettingsSection: FunctionComponent<IndexSettingsSectionProps> = ({ policy, updatePolicy, repo }) => (
    <div className="form-group">
        <Label className="mb-0">
            Auto-indexing
            <div className={styles.toggleContainer}>
                <Toggle
                    id="indexing-enabled"
                    value={policy.indexingEnabled}
                    className={styles.toggle}
                    onToggle={indexingEnabled => {
                        if (indexingEnabled) {
                            updatePolicy({ indexingEnabled })
                        } else {
                            updatePolicy({
                                indexingEnabled,
                                indexIntermediateCommits: false,
                                indexCommitMaxAgeHours: null,
                            })
                        }
                    }}
                />

                <Text size="small" className="text-muted mb-0">
                    Sourcegraph will automatically generate precise code intelligence data for matching
                    {repo ? '' : ' repositories and'} revisions. Indexing configuration will be inferred from the
                    content at matching revisions if not explicitly configured for{' '}
                    {repo ? 'this repository' : 'matching repositories'}.{' '}
                    {repo && (
                        <>
                            See this repository's <Link to="../index-configuration">index configuration</Link>.
                        </>
                    )}
                </Text>
            </div>
        </Label>

        <IndexSettings policy={policy} updatePolicy={updatePolicy} />
    </div>
)

interface IndexSettingsProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    updatePolicy: PolicyUpdater
}

const IndexSettings: FunctionComponent<IndexSettingsProps> = ({ policy, updatePolicy }) =>
    policy.indexingEnabled && policy.type !== GitObjectType.GIT_COMMIT ? (
        <div className="ml-3 mb-3">
            <div className="mt-2 mb-2">
                <Checkbox
                    id="indexing-max-age-enabled"
                    label="Ignore commits older than a given age"
                    checked={policy.indexCommitMaxAgeHours !== null}
                    onChange={event =>
                        updatePolicy({
                            // 1 year by default
                            indexCommitMaxAgeHours: event.target.checked ? 8760 : null,
                        })
                    }
                    message="By default, commit age does not factor into auto-indexing decisions. Enable this option to ignore commits older than a configurable age."
                />

                {policy.indexCommitMaxAgeHours !== null && (
                    <div className="mt-2 ml-4">
                        <DurationSelect
                            id="index-commit-max-age"
                            value={`${policy.indexCommitMaxAgeHours}`}
                            onChange={indexCommitMaxAgeHours => updatePolicy({ indexCommitMaxAgeHours })}
                        />
                    </div>
                )}
            </div>

            {policy.type === GitObjectType.GIT_TREE && (
                <div className="mb-2">
                    <Checkbox
                        id="index-intermediate-commits"
                        label="Apply to all commits on matching branches"
                        checked={policy.indexIntermediateCommits}
                        onChange={event => updatePolicy({ indexIntermediateCommits: event.target.checked })}
                        message="By default, only the tip of the branches are indexed. Enable this option to index all commits on the matching branches."
                    />
                </div>
            )}
        </div>
    ) : (
        <></>
    )

interface RetentionSettingsSectionProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    updatePolicy: PolicyUpdater
}

const RetentionSettingsSection: FunctionComponent<RetentionSettingsSectionProps> = ({ policy, updatePolicy }) => (
    <div className="form-group">
        <Label className="mb-0">
            Precise code intelligence index retention
            <div className={styles.toggleContainer}>
                <Toggle
                    id="retention-enabled"
                    value={policy.retentionEnabled}
                    className={styles.toggle}
                    onToggle={retentionEnabled => {
                        if (retentionEnabled) {
                            updatePolicy({ retentionEnabled })
                        } else {
                            updatePolicy({
                                retentionEnabled,
                                retainIntermediateCommits: false,
                                retentionDurationHours: null,
                            })
                        }
                    }}
                    disabled={policy.protected || policy.type === GitObjectType.GIT_COMMIT}
                />

                <Text size="small" className="text-muted mb-0">
                    Precise code intelligence indexes will expire once they no longer serve data for a revision matched
                    by a configuration policy. Expired indexes are removed once they are no longer referenced by any
                    unexpired index. Enabling retention keeps data for matching revisions longer than the default.
                </Text>
            </div>
        </Label>

        <RetentionSettings policy={policy} updatePolicy={updatePolicy} />
    </div>
)

interface RetentionSettingsProps {
    policy: CodeIntelligenceConfigurationPolicyFields
    updatePolicy: PolicyUpdater
}

const RetentionSettings: FunctionComponent<RetentionSettingsProps> = ({ policy, updatePolicy }) => (
    <>
        {policy.type === GitObjectType.GIT_COMMIT && (
            <Alert variant="info" className="mt-2">
                Precise code intelligence indexes serving data for the tip of the default branch are retained
                implicitly.
            </Alert>
        )}
        {policy.retentionEnabled ? (
            <div className="ml-3 mb-3">
                <div className="mt-2 mb-2">
                    <Checkbox
                        id="retention-max-age-enabled"
                        label="Expire matching indexes older than a given age"
                        checked={policy.retentionDurationHours !== null}
                        onChange={event =>
                            updatePolicy({
                                retentionDurationHours: event.target.checked ? 168 : null,
                            })
                        }
                        message="By default, matching indexes are protected indefinitely. Enable this option to expire index records once they have reached a configurable age (after upload)."
                    />

                    {policy.retentionDurationHours !== null && (
                        <div className="mt-2 ml-4">
                            <DurationSelect
                                id="retention-duration"
                                value={`${policy.retentionDurationHours}`}
                                onChange={retentionDurationHours => updatePolicy({ retentionDurationHours })}
                            />
                        </div>
                    )}
                </div>

                {policy.type === GitObjectType.GIT_TREE && (
                    <div className="mb-2">
                        <Checkbox
                            id="retain-intermediate-commits"
                            label="Apply to all commits on matching branches"
                            checked={policy.retainIntermediateCommits}
                            onChange={event => updatePolicy({ retainIntermediateCommits: event.target.checked })}
                            message="By default, only indexes providing data for the tip of the branches are protected. Enable this option to protect indexes providing data for any commit on the matching branches."
                        />
                    </div>
                )}
            </div>
        ) : (
            <></>
        )}
    </>
)

function validatePolicy(
    policy: CodeIntelligenceConfigurationPolicyFields,
    globalAutoIndexingEnabled: boolean
): boolean {
    const invalidConditions = [
        // Name is required
        policy.name === '',

        // Pattern is required if policy type is GIT_COMMIT
        policy.type !== GitObjectType.GIT_COMMIT && policy.pattern === '',

        // If repository patterns are supplied they must be non-empty
        policy.repositoryPatterns?.some(pattern => pattern === ''),

        // Policy type must be GIT_{COMMIT,TAG,TREE}
        ![GitObjectType.GIT_COMMIT, GitObjectType.GIT_TAG, GitObjectType.GIT_TREE].includes(policy.type),

        // If numeric values are supplied they must be between 1 and maxDuration (inclusive)
        policy.retentionDurationHours !== null &&
            (policy.retentionDurationHours < 0 || policy.retentionDurationHours > maxDuration),
        policy.indexCommitMaxAgeHours !== null &&
            (policy.indexCommitMaxAgeHours < 0 || policy.indexCommitMaxAgeHours > maxDuration),

        // If global indexing is disabled, the policy must be scoped to a repository
        !globalAutoIndexingEnabled && hasGlobalPolicyViolation(policy),
    ]

    return invalidConditions.every(isInvalid => !isInvalid)
}

function comparePolicies(
    a: CodeIntelligenceConfigurationPolicyFields,
    b?: CodeIntelligenceConfigurationPolicyFields
): boolean {
    if (b === undefined) {
        return false
    }

    const equalityConditions = [
        a.id === b.id,
        a.name === b.name,
        a.type === b.type,
        a.pattern === b.pattern,
        a.retentionEnabled === b.retentionEnabled,
        a.retentionDurationHours === b.retentionDurationHours,
        a.retainIntermediateCommits === b.retainIntermediateCommits,
        a.indexingEnabled === b.indexingEnabled,
        a.indexCommitMaxAgeHours === b.indexCommitMaxAgeHours,
        a.indexIntermediateCommits === b.indexIntermediateCommits,
        comparePatterns(a.repositoryPatterns, b.repositoryPatterns),
    ]

    return equalityConditions.every(isEqual => isEqual)
}

function comparePatterns(a: string[] | null, b: string[] | null): boolean {
    if (a === null && b === null) {
        // Neither supplied
        return true
    }

    if (!a || !b) {
        // Only one supplied
        return false
    }

    // Both supplied and their contents match
    return a.length === b.length && a.every((pattern, index) => b[index] === pattern)
}

type viewEventFeatureName = 'repo.codeIntel.configurationPolicy' | 'admin.codeIntel.configurationPolicy'

function getViewEventFeatureName(hasRepo: boolean): viewEventFeatureName {
    return hasRepo ? 'repo.codeIntel.configurationPolicy' : 'admin.codeIntel.configurationPolicy'
}
