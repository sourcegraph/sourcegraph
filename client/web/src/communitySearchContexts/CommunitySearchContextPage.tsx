import React, { useCallback, useEffect, useMemo, useState } from 'react'

import { mdiSourceRepositoryMultiple, mdiGithub, mdiGitlab, mdiBitbucket } from '@mdi/js'
import classNames from 'classnames'
import { useNavigate, useLocation } from 'react-router-dom'
import { catchError, startWith } from 'rxjs/operators'

import { SyntaxHighlightedSearchQuery } from '@sourcegraph/branded'
import { asError, isErrorLike } from '@sourcegraph/common'
import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { QueryState, SearchContextInputProps, SearchContextProps } from '@sourcegraph/shared/src/search'
import type { SettingsCascadeProps, Settings } from '@sourcegraph/shared/src/settings/settings'
import { Button, useObservable, Link, Card, Icon, Code, H2, H3, Text } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { PageTitle } from '../components/PageTitle'
import type { SearchPatternType } from '../graphql-operations'
import { submitSearch } from '../search/helpers'
import { useNavbarQueryState } from '../stores'
import { SearchPageInput } from '../storm/pages/SearchPage/SearchPageInput'

import type { CommunitySearchContextMetadata, CommunitySearchContextSpecs } from './types'

import styles from './CommunitySearchContextPage.module.scss'

// Unique number identifier for telemetry
const specTypes: { [k in CommunitySearchContextSpecs]: number } = {
    backstage: 0,
    chakraui: 1,
    cncf: 2,
    temporalio: 3,
    o3de: 4,
    stackstorm: 5,
    kubernetes: 6,
    stanford: 7,
    julia: 8,
}

export interface CommunitySearchContextPageProps
    extends SettingsCascadeProps<Settings>,
        PlatformContextProps<'settings' | 'sourcegraphURL' | 'requestGraphQL' | 'telemetryRecorder'>,
        SearchContextInputProps,
        Pick<SearchContextProps, 'fetchSearchContextBySpec'> {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean

    // CommunitySearchContext page metadata
    communitySearchContextMetadata: CommunitySearchContextMetadata
}

export const CommunitySearchContextPage: React.FunctionComponent<
    React.PropsWithChildren<CommunitySearchContextPageProps>
> = (props: CommunitySearchContextPageProps) => {
    const location = useLocation()
    const navigate = useNavigate()
    const LOADING = 'loading' as const

    const [queryState, setQueryState] = useState<QueryState>({
        query: '',
    })

    const telemetryRecorder = props.platformContext.telemetryRecorder

    useEffect(() => {
        telemetryRecorder.recordEvent('communitySearchContext', 'view', {
            metadata: { spec: specTypes[props.communitySearchContextMetadata.spec] },
        })
    }, [props.communitySearchContextMetadata.spec, telemetryRecorder])
    const caseSensitive = useNavbarQueryState(state => state.searchCaseSensitivity)

    const contextQuery = `context:${props.communitySearchContextMetadata.spec}`

    const { fetchSearchContextBySpec } = props
    const searchContextOrError = useObservable(
        useMemo(
            () =>
                fetchSearchContextBySpec(props.communitySearchContextMetadata.spec, props.platformContext).pipe(
                    startWith(LOADING),
                    catchError(error => [asError(error)])
                ),
            [props.communitySearchContextMetadata.spec, fetchSearchContextBySpec, props.platformContext]
        )
    )

    const { selectedSearchContextSpec } = props
    const onSubmitExample = useCallback(
        (query: string, patternType: SearchPatternType) =>
            (event?: React.MouseEvent<HTMLButtonElement>): void => {
                telemetryRecorder.recordEvent('communitySearchContext.suggestion', 'click')
                event?.preventDefault()
                submitSearch({
                    historyOrNavigate: navigate,
                    location,
                    query,
                    caseSensitive,
                    patternType,
                    selectedSearchContextSpec,
                    source: 'communitySearchContextPage',
                })
            },
        [telemetryRecorder, caseSensitive, location, navigate, selectedSearchContextSpec]
    )

    const onRepoLinkClicked = useCallback(() => {
        telemetryRecorder.recordEvent('communitySearchContext.repoLink', 'click')
    }, [telemetryRecorder])

    const onExternalRepoLinkClicked = useCallback(() => {
        telemetryRecorder.recordEvent('communitySearchContext.repoLink.external', 'click')
    }, [telemetryRecorder])

    return (
        <div className={styles.communitySearchContextsPage}>
            <PageTitle title={props.communitySearchContextMetadata.title} />
            <CommunitySearchContextPageLogo
                className={styles.logo}
                icon={props.communitySearchContextMetadata.homepageIcon}
                text={props.communitySearchContextMetadata.title}
            />
            <div className={styles.subheading}>
                {props.communitySearchContextMetadata.lowProfile ? (
                    <>{props.communitySearchContextMetadata.description}</>
                ) : (
                    <span className="text-monospace">
                        {/*
                           a11y-ignore
                           Rule: "color-contrast" (Elements must have sufficient color contrast)
                           GitHub issue: https://github.com/sourcegraph/sourcegraph/issues/33343
                          */}
                        <span className="search-filter-keyword a11y-ignore">context:</span>
                        {props.communitySearchContextMetadata.spec}
                    </span>
                )}
            </div>
            <div className={styles.container}>
                <SearchPageInput
                    {...props}
                    queryState={queryState}
                    setQueryState={setQueryState}
                    hardCodedSearchContextSpec={props.communitySearchContextMetadata.spec}
                    simpleSearch={false}
                />
            </div>
            {!props.communitySearchContextMetadata.lowProfile && (
                <div className="row">
                    <div className={classNames('col-xs-12 col-lg-7', styles.column)}>
                        <Text weight="regular" className={classNames('mb-4', styles.contentDescription)}>
                            {props.communitySearchContextMetadata.description}
                        </Text>

                        <H2>Search examples</H2>
                        {props.communitySearchContextMetadata.examples.map(example => (
                            <div className="mt-3" key={example.title}>
                                <H3 className="mb-3">{example.title}</H3>
                                <Text>{example.description}</Text>
                                <div className="d-flex mb-4">
                                    <small className={classNames('form-control text-monospace ', styles.exampleBar)}>
                                        <SyntaxHighlightedSearchQuery query={`${contextQuery} ${example.query}`} />
                                    </small>
                                    <div className="d-flex">
                                        <Button
                                            className={styles.searchButton}
                                            aria-label="Search"
                                            onClick={onSubmitExample(
                                                `${contextQuery} ${example.query}`,
                                                example.patternType
                                            )}
                                            variant="secondary"
                                            size="sm"
                                        >
                                            Search
                                        </Button>
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                    <div className={classNames('col-xs-12 col-lg-5', styles.column)}>
                        <div className="order-2-lg order-1-xs">
                            <Card className={styles.repoCard}>
                                <H2>
                                    <Icon className="mr-2" aria-hidden={true} svgPath={mdiSourceRepositoryMultiple} />
                                    Repositories
                                </H2>
                                <Text>
                                    Using the syntax{' '}
                                    <Code>
                                        {/*
                                            a11y-ignore
                                            Rule: "color-contrast" (Elements must have sufficient color contrast)
                                            GitHub issue: https://github.com/sourcegraph/sourcegraph/issues/33343
                                          */}
                                        <span className="search-filter-keyword a11y-ignore">context:</span>
                                        {props.communitySearchContextMetadata.spec}
                                    </Code>{' '}
                                    in a query will search these repositories:
                                </Text>
                                {searchContextOrError &&
                                    !isErrorLike(searchContextOrError) &&
                                    searchContextOrError !== LOADING && (
                                        <div className="community-search-contexts-page__repo-list row">
                                            <div className="col-lg-6">
                                                {searchContextOrError.repositories
                                                    .slice(0, Math.ceil(searchContextOrError.repositories.length / 2))
                                                    .map(repo => (
                                                        <RepoLink
                                                            key={repo.repository.name}
                                                            repo={repo.repository.name}
                                                            onRepoLinkClicked={onRepoLinkClicked}
                                                            onExternalRepoLinkClicked={onExternalRepoLinkClicked}
                                                        />
                                                    ))}
                                            </div>
                                            <div className="col-lg-6">
                                                {searchContextOrError.repositories
                                                    .slice(Math.ceil(searchContextOrError.repositories.length / 2))
                                                    .map(repo => (
                                                        <RepoLink
                                                            key={repo.repository.name}
                                                            repo={repo.repository.name}
                                                            onRepoLinkClicked={onRepoLinkClicked}
                                                            onExternalRepoLinkClicked={onExternalRepoLinkClicked}
                                                        />
                                                    ))}
                                            </div>
                                        </div>
                                    )}
                            </Card>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

const RepoLink: React.FunctionComponent<
    React.PropsWithChildren<{
        repo: string
        onRepoLinkClicked: () => void
        onExternalRepoLinkClicked: () => void
    }>
> = ({ repo, onRepoLinkClicked, onExternalRepoLinkClicked }) => (
    <li className={classNames('list-unstyled mb-3', styles.repoItem)} key={repo}>
        {repo.startsWith('github.com') && (
            <>
                <Link
                    to={`https://${repo}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={onExternalRepoLinkClicked}
                >
                    <Icon className={styles.repoListIcon} aria-hidden={true} svgPath={mdiGithub} />
                </Link>
                <Link to={`/${repo}`} className="text-monospace search-filter-keyword" onClick={onRepoLinkClicked}>
                    {displayRepoName(repo)}
                </Link>
            </>
        )}
        {repo.startsWith('gitlab.com') && (
            <>
                <Link
                    to={`https://${repo}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={onExternalRepoLinkClicked}
                >
                    <Icon className={styles.repoListIcon} aria-hidden={true} svgPath={mdiGitlab} />
                </Link>
                <Link to={`/${repo}`} className="text-monospace search-filter-keyword" onClick={onRepoLinkClicked}>
                    {displayRepoName(repo)}
                </Link>
            </>
        )}
        {repo.startsWith('bitbucket.org') && (
            <>
                <Link
                    to={`https://${repo}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={onExternalRepoLinkClicked}
                >
                    <Icon className={styles.repoListIcon} aria-hidden={true} svgPath={mdiBitbucket} />
                </Link>
                <Link to={`/${repo}`} className="text-monospace search-filter-keyword" onClick={onRepoLinkClicked}>
                    {displayRepoName(repo)}
                </Link>
            </>
        )}
    </li>
)

interface CommunitySearchContextPageLogoProps extends Exclude<React.ImgHTMLAttributes<HTMLImageElement>, 'src'> {
    icon: string
    text: string
}

/**
 * The community search context logo image.
 */
const CommunitySearchContextPageLogo: React.FunctionComponent<
    React.PropsWithChildren<CommunitySearchContextPageLogoProps>
> = props => (
    <div className={classNames('d-flex align-items-center', styles.logoContainer)}>
        <img {...props} src={props.icon} alt="" />
        <H3 as="span" className="font-weight-normal mb-0 ml-1">
            {props.text}
        </H3>
    </div>
)
