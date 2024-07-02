import React, { useCallback, useEffect, useMemo } from 'react'

import { mdiChevronRight } from '@mdi/js'
import classNames from 'classnames'
import { forkJoin, of, type Observable } from 'rxjs'
import { catchError, map, mergeMap } from 'rxjs/operators'

import { asError, isErrorLike, pluralize, type ErrorLike } from '@sourcegraph/common'
import { LATEST_VERSION, aggregateStreamingSearch, type ContentMatch } from '@sourcegraph/shared/src/search/stream'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Alert, ButtonLink, Code, Container, H3, Icon, Link, PageHeader, Text, Tooltip } from '@sourcegraph/wildcard'

import { FilteredConnection, type Filter } from '../components/FilteredConnection'
import { PageTitle } from '../components/PageTitle'
import { useFeatureFlag } from '../featureFlags/useFeatureFlag'
import { SearchPatternType, type FeatureFlagFields } from '../graphql-operations'

import { fetchFeatureFlags as defaultFetchFeatureFlags } from './backend'

import styles from './SiteAdminFeatureFlagsPage.module.scss'

interface SiteAdminFeatureFlagsPageProps extends TelemetryProps, TelemetryV2Props {
    fetchFeatureFlags?: typeof defaultFetchFeatureFlags
    productVersion?: string
}

/**
 * Reference indicates a potential usage of a feature flag.
 */
export interface Reference {
    /**
     * File where the reference occurred in.
     */
    file: string
    /**
     * Partial URL to the results, starting with '/search'
     */
    searchURL: string
}

/**
 * Denotes the feature flag itself, along with potential references to the feature flag.
 */
export type FeatureFlagAndReferences = FeatureFlagFields & {
    references: Reference[]
}

/**
 * Tries to parse a commit or tag out of the product version. Falls back to 'main' if
 * nothing useful is inferred.
 *
 * @param productVersion e.g. from window.context
 * @returns git ref
 */
export function parseProductReference(productVersion: string): string {
    // Look for format 214157_2023-04-19_5.0-89aa613e7e1e, where last segment is
    // $VERSION-$COMMIT
    const parts = productVersion.split('_')
    if (parts.length === 3) {
        // We need to split $VERSION-$COMMIT - if any of these step fails, we
        // fall back to main.
        const versionAndCommit = parts.pop()?.split('-')
        if (!versionAndCommit || versionAndCommit.length !== 2) {
            return 'main'
        }
        return versionAndCommit[1]
    }
    // Unknown format, fall back to main
    if (parts.length === 2) {
        return 'main'
    }
    // Special case for dev tag
    if (productVersion.startsWith('0.0.0')) {
        return 'main'
    }
    // Otherwise assume product version is probably a tag
    return productVersion
}

/**
 * Finds references to this flag in `github.com/sourcegraph/sourcegraph@productGitVersion`.
 * Will only work if this Sourcegraph instance has the Sourcegraph repository - if not,
 * returns an empty reference set.
 */
export function getFeatureFlagReferences(flagName: string, productGitVersion: string): Observable<Reference[]> {
    const repoQuery = `repo:^github.com/sourcegraph/sourcegraph$@${productGitVersion}`
    const flagQuery = `('${flagName}' OR "${flagName}")`
    const referencesQuery = `${repoQuery} (${flagQuery} AND (lang:TypeScript OR lang:Go)) count:25`
    return aggregateStreamingSearch(of(referencesQuery), {
        caseSensitive: true,
        patternType: SearchPatternType.standard,
        version: LATEST_VERSION,
        trace: undefined,
    }).pipe(
        map(({ results }) =>
            results
                .filter((match): match is ContentMatch => match.type === 'content')
                .map(content => ({
                    file: content.path,
                    searchURL: `/search?q=${encodeURIComponent(`${repoQuery} ${flagQuery} file:${content.path}`)}`,
                }))
        )
    )
}

const filters: Filter[] = [
    {
        id: 'filters',
        label: 'Type',
        type: 'select',
        options: [
            {
                label: 'All',
                value: 'all',
                tooltip: 'Show all feature flags',
                args: {},
            },
            {
                label: 'Boolean',
                value: 'boolean',
                tooltip: 'Show boolean feature flags',
                args: { type: 'FeatureFlagBoolean' },
            },
            {
                label: 'Rollout',
                value: 'rollout',
                tooltip: 'Show rollout feature flags',
                args: { type: 'FeatureFlagRollout' },
            },
        ],
    },
]

export const SiteAdminFeatureFlagsPage: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminFeatureFlagsPageProps>
> = ({ fetchFeatureFlags = defaultFetchFeatureFlags, productVersion = window.context.version, telemetryRecorder }) => {
    // Try to parse out a git rev based on the product version, otherwise just fall back
    // to main.
    const productGitVersion = parseProductReference(productVersion)

    // Fetch feature flags
    const featureFlagsOrErrorsObservable = useMemo(
        () =>
            fetchFeatureFlags().pipe(
                // T => Observable<T[]>
                map(flags =>
                    // Observable<T>[] => Observable<T[]>
                    flags.length > 0
                        ? forkJoin(
                              flags.map(flag =>
                                  getFeatureFlagReferences(flag.name, productGitVersion).pipe(
                                      map(
                                          (references): FeatureFlagAndReferences => ({
                                              ...flag,
                                              references,
                                          })
                                      )
                                  )
                              )
                          )
                        : of([])
                ),
                // Observable<T[]> => T[]
                mergeMap(flags => flags),
                // T[] => (T | ErrorLike)[]
                catchError((error): [ErrorLike] => [asError(error)])
            ),
        [fetchFeatureFlags, productGitVersion]
    )

    const queryFeatureFlags = useCallback(
        (args: { query?: string; type?: string }) =>
            featureFlagsOrErrorsObservable.pipe(
                map(featureFlagsOrErrors => {
                    if (isErrorLike(featureFlagsOrErrors)) {
                        return { nodes: [] }
                    }
                    return {
                        nodes: featureFlagsOrErrors.filter(
                            node =>
                                (args.type === undefined || node.__typename === args.type) &&
                                (!args.query || node.name.toLowerCase().includes(args.query.toLowerCase()))
                        ),
                        totalCount: featureFlagsOrErrors.length,
                        pageInfo: { hasNextPage: false },
                    }
                })
            ),
        [featureFlagsOrErrorsObservable]
    )

    const [isSourcegraphCloudManagedFeatureFlagsWarningShown] = useFeatureFlag(
        'sourcegraph-cloud-managed-feature-flags-warning-shown'
    )

    useEffect(() => telemetryRecorder.recordEvent('admin.featureFlags', 'view'), [telemetryRecorder])

    return (
        <>
            <PageTitle title="Feature flags - Admin" />

            <PageHeader
                headingElement="h2"
                path={[{ text: 'Feature flags' }]}
                description={
                    <>
                        Feature flags, as opposed to experimental features, are intended to be strictly short-lived.
                        They are designed to be useful for A/B testing, and the values of all active feature flags are
                        added to every event log for the purpose of analytics. To learn more, refer to{' '}
                        <Link target="_blank" rel="noopener noreferrer" to="/help/dev/how-to/use_feature_flags">
                            How to use feature flags
                        </Link>
                        .
                    </>
                }
                className={classNames(styles.pageHeader, 'mb-3')}
                actions={
                    <ButtonLink variant="primary" to="./configuration/new">
                        Create feature flag
                    </ButtonLink>
                }
            />

            {isSourcegraphCloudManagedFeatureFlagsWarningShown && (
                <Alert variant="info">
                    Feature flag settings are managed by Sourcegraph and will be overridden by updates. Contact support
                    for help.
                </Alert>
            )}

            <Container>
                <FilteredConnection<FeatureFlagAndReferences, {}>
                    listComponent="div"
                    listClassName={classNames('mb-3', styles.flagsGrid)}
                    noun="feature flag"
                    pluralNoun="feature flags"
                    queryConnection={queryFeatureFlags}
                    nodeComponent={FeatureFlagNode}
                    filters={filters}
                    withCenteredSummary={true}
                />
            </Container>
        </>
    )
}

interface FeatureFlagNodeProps {
    node: FeatureFlagAndReferences
}

const FeatureFlagNode: React.FunctionComponent<React.PropsWithChildren<FeatureFlagNodeProps>> = ({ node }) => {
    const { name, overrides, references } = node
    const hasOverridesOrReferences = overrides.length > 0 || references.length > 0
    return (
        <React.Fragment key={name}>
            <div className={classNames('d-flex flex-column', styles.information)}>
                <div>
                    <H3 className={classNames(!hasOverridesOrReferences && 'm-0')}>{name}</H3>

                    {hasOverridesOrReferences && (
                        <Text className="m-0">
                            <span className="text-muted">
                                {overrides.length > 0 &&
                                    `${overrides.length} ${overrides.length !== 1 ? 'overrides' : 'override'}`}
                                {overrides.length > 0 && references.length > 0 && ', '}
                                {references.length > 0 &&
                                    `${references.length} ${pluralize('reference', references.length)}`}
                            </span>
                        </Text>
                    )}
                </div>
            </div>

            <span className={classNames('d-none d-md-inline', styles.progress)}>
                <div className="m-0 text-nowrap d-flex flex-column align-items-center justify-content-center">
                    <div>
                        {node.__typename === 'FeatureFlagBoolean' && <Code>{JSON.stringify(node.value)}</Code>}
                        {node.__typename === 'FeatureFlagRollout' && node.rolloutBasisPoints}
                    </div>

                    {node.__typename === 'FeatureFlagRollout' && (
                        <Tooltip content={`${Math.floor(node.rolloutBasisPoints / 100) || 0}%`} placement="bottom">
                            <div>
                                <meter
                                    min={0}
                                    max={1}
                                    optimum={1}
                                    value={node.rolloutBasisPoints / (100 * 100)}
                                    aria-label="rollout progress"
                                />
                            </div>
                        </Tooltip>
                    )}
                </div>
            </span>

            <span className={classNames(styles.button, 'd-none d-md-inline')}>
                <Link to={`./configuration/${node.name}`} className="p-0">
                    <Icon svgPath={mdiChevronRight} inline={false} aria-label="Configure" />
                </Link>
            </span>

            <span className={styles.separator} />
        </React.Fragment>
    )
}
