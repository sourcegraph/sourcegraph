import React, { useCallback, useEffect, useMemo, useState } from 'react'

import { mdiMagnify } from '@mdi/js'
import classNames from 'classnames'
import { debounce } from 'lodash'
import { useParams } from 'react-router-dom'
import { catchError, startWith } from 'rxjs/operators'

import { SyntaxHighlightedSearchQuery } from '@sourcegraph/branded'
import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { asError, isErrorLike, renderMarkdown, pluralize } from '@sourcegraph/common'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { VirtualList } from '@sourcegraph/shared/src/components/VirtualList'
import type { SearchContextRepositoryRevisionsFields } from '@sourcegraph/shared/src/graphql-operations'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SearchContextProps } from '@sourcegraph/shared/src/search'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import {
    Badge,
    Container,
    PageHeader,
    LoadingSpinner,
    useObservable,
    Button,
    Link,
    Alert,
    H3,
    Input,
    Markdown,
} from '@sourcegraph/wildcard'

import { Page } from '../../components/Page'
import { PageTitle } from '../../components/PageTitle'
import { SearchPatternType } from '../../graphql-operations'

import { useDefaultContext } from './hooks/useDefaultContext'
import { useToggleSearchContextStar } from './hooks/useToggleSearchContextStar'
import { SearchContextStarButton } from './SearchContextStarButton'

import styles from './SearchContextPage.module.scss'

export interface SearchContextPageProps
    extends Pick<SearchContextProps, 'fetchSearchContextBySpec'>,
        PlatformContextProps<'requestGraphQL' | 'telemetryRecorder'> {
    authenticatedUser: Pick<AuthenticatedUser, 'id'> | null
}

const initialRepositoriesToShow = 15
const incrementalRepositoriesToShow = 10

const SearchContextRepositories: React.FunctionComponent<
    React.PropsWithChildren<{ repositories: SearchContextRepositoryRevisionsFields[] }>
> = ({ repositories }) => {
    const [filterQuery, setFilterQuery] = useState('')
    const debouncedSetFilterQuery = useMemo(() => debounce(value => setFilterQuery(value), 250), [setFilterQuery])
    const filteredRepositories = useMemo(
        () =>
            repositories.filter(repositoryRevisions => {
                const lowerCaseFilterQuery = filterQuery.toLowerCase()
                return (
                    !lowerCaseFilterQuery ||
                    repositoryRevisions.repository.name.toLowerCase().includes(lowerCaseFilterQuery) ||
                    repositoryRevisions.revisions.some(revision =>
                        revision.toLowerCase().includes(lowerCaseFilterQuery)
                    )
                )
            }),
        [repositories, filterQuery]
    )

    const [repositoriesToShow, setRepositoriesToShow] = useState(initialRepositoriesToShow)
    const onBottomHit = useCallback(
        () =>
            setRepositoriesToShow(repositoriesToShow =>
                Math.min(filteredRepositories.length, repositoriesToShow + incrementalRepositoriesToShow)
            ),
        [filteredRepositories]
    )

    const renderRepositoryRevisions = useCallback(
        (repositoryRevisions: SearchContextRepositoryRevisionsFields) => (
            <div
                key={repositoryRevisions.repository.name}
                className={classNames(styles.searchContextPageRepoRevsRow, 'd-flex')}
            >
                <div className={classNames(styles.searchContextPageRepoRevsRowRepo, 'w-50')}>
                    <Link to={`/${repositoryRevisions.repository.name}`}>{repositoryRevisions.repository.name}</Link>
                </div>
                <div className="w-50">
                    {repositoryRevisions.revisions.map(revision => (
                        <div
                            key={`${repositoryRevisions.repository.name}-${revision}`}
                            className={styles.searchContextPageRepoRevsRowRev}
                        >
                            <Link to={`/${repositoryRevisions.repository.name}@${revision}`}>{revision}</Link>
                        </div>
                    ))}
                </div>
            </div>
        ),
        []
    )

    return (
        <div>
            <div className="d-flex justify-content-between align-items-center">
                {filteredRepositories.length > 0 && (
                    <H3>
                        <span>
                            {filteredRepositories.length}{' '}
                            {pluralize('repository', filteredRepositories.length, 'repositories')}
                        </span>
                    </H3>
                )}
                {repositories.length > 0 && (
                    <Input
                        className="w-50"
                        placeholder="Search repositories and revisions"
                        onChange={event => debouncedSetFilterQuery(event.target.value)}
                    />
                )}
            </div>
            {repositories.length > 0 && (
                <>
                    <div className="d-flex mt-3">
                        <div className="w-50">Repositories</div>
                        <div className="w-50">Revisions</div>
                    </div>
                    <hr className="mt-2 mb-0" />
                    <VirtualList<SearchContextRepositoryRevisionsFields>
                        className="mt-2"
                        itemsToShow={repositoriesToShow}
                        onShowMoreItems={onBottomHit}
                        items={filteredRepositories}
                        itemProps={undefined}
                        itemKey={repositoryRevisions => repositoryRevisions.repository.name}
                        renderItem={renderRepositoryRevisions}
                    />
                </>
            )}
        </div>
    )
}

export const SearchContextPage: React.FunctionComponent<SearchContextPageProps> = ({
    fetchSearchContextBySpec,
    platformContext,
    authenticatedUser,
}) => {
    const params = useParams()
    const spec: string = params.spec ? `${params.specOrOrg}/${params.spec}` : params.specOrOrg!

    const LOADING = 'loading' as const

    useEffect(() => {
        platformContext.telemetryRecorder.recordEvent('searchContext', 'view')
    }, [platformContext.telemetryRecorder])

    const searchContextOrError = useObservable(
        React.useMemo(
            () =>
                fetchSearchContextBySpec(spec, platformContext).pipe(
                    startWith(LOADING),
                    catchError(error => [asError(error)])
                ),
            [spec, fetchSearchContextBySpec, platformContext]
        )
    )

    const searchContext =
        searchContextOrError === LOADING || isErrorLike(searchContextOrError) ? null : searchContextOrError

    const [alert, setAlert] = useState('')

    const { starred, toggleStar } = useToggleSearchContextStar(
        searchContext?.viewerHasStarred || false,
        searchContext?.id || undefined,
        authenticatedUser?.id || undefined
    )

    const toggleStarWithErrorHandling = useCallback(() => {
        setAlert('') // Clear previous alerts
        toggleStar().catch(error => {
            if (isErrorLike(error)) {
                setAlert(error.message)
            }
        })
    }, [setAlert, toggleStar])

    const { defaultContext, setAsDefault } = useDefaultContext(
        searchContext?.viewerHasAsDefault ? searchContext.id : undefined
    )
    const isDefault = defaultContext === searchContext?.id
    const setAsDefaultWithErrorHandling = useCallback(() => {
        setAlert('') // Clear previous alerts
        setAsDefault(searchContext?.id, authenticatedUser?.id).catch(error => {
            if (isErrorLike(error)) {
                setAlert(error.message)
            }
        })
    }, [authenticatedUser?.id, searchContext?.id, setAsDefault])

    const actions = searchContext && (
        <div className={styles.actions}>
            {searchContext && authenticatedUser && !searchContext.autoDefined && (
                <SearchContextStarButton starred={starred} onClick={toggleStarWithErrorHandling} />
            )}
            {searchContext.viewerCanManage && (
                <Button
                    to={`/contexts/${encodeURIComponent(searchContext.spec)}/edit`}
                    data-testid="edit-search-context-link"
                    variant="secondary"
                    as={Link}
                >
                    Edit
                </Button>
            )}
            {searchContext && authenticatedUser && !isDefault && (
                <Button variant="secondary" onClick={setAsDefaultWithErrorHandling}>
                    Use as default
                </Button>
            )}
        </div>
    )

    return (
        <div className="w-100">
            <Page>
                <div className="container col-sm-8">
                    {searchContextOrError === LOADING && (
                        <div className="d-flex justify-content-center">
                            <LoadingSpinner inline={false} />
                        </div>
                    )}
                    {searchContext && (
                        <>
                            <PageTitle title={searchContext.spec} />
                            <PageHeader className="mb-2" actions={actions}>
                                <PageHeader.Heading as="h2" styleAs="h1">
                                    <PageHeader.Breadcrumb icon={mdiMagnify} to="/search" aria-label="Code Search" />
                                    <PageHeader.Breadcrumb to="/contexts">Contexts</PageHeader.Breadcrumb>
                                    <PageHeader.Breadcrumb>
                                        <div>
                                            <span
                                                className={classNames(
                                                    !searchContext.public && 'mr-2',
                                                    styles.searchContextPageTitleSpec
                                                )}
                                            >
                                                {searchContext.spec}
                                            </span>
                                        </div>
                                    </PageHeader.Breadcrumb>
                                </PageHeader.Heading>
                            </PageHeader>
                            <div>
                                {isDefault ? (
                                    <>
                                        <Badge
                                            variant="secondary"
                                            className="text-uppercase"
                                            data-testid="search-context-default-badge"
                                        >
                                            Default
                                        </Badge>{' '}
                                    </>
                                ) : null}
                                {!searchContext.public ? (
                                    <>
                                        <Badge variant="secondary" pill={true}>
                                            Private
                                        </Badge>{' '}
                                    </>
                                ) : null}
                                {searchContext.autoDefined ? (
                                    <Badge variant="outlineSecondary" pill={true}>
                                        Auto
                                    </Badge>
                                ) : (
                                    <span className="ml-1 text-muted">
                                        Updated <Timestamp date={searchContext.updatedAt} noAbout={true} />
                                    </span>
                                )}
                            </div>
                            {alert && (
                                <Alert variant="danger" className="mt-3">
                                    {alert}
                                </Alert>
                            )}
                            <Container className={classNames('mt-4', styles.container)}>
                                {searchContext.description && (
                                    <Markdown dangerousInnerHTML={renderMarkdown(searchContext.description)} />
                                )}
                                {!searchContext.autoDefined && searchContext.query.length === 0 && (
                                    <SearchContextRepositories repositories={searchContext.repositories} />
                                )}
                                {searchContext.query.length > 0 && (
                                    <Link
                                        to={`/search?${buildSearchURLQuery(
                                            searchContext.query,
                                            SearchPatternType.regexp,
                                            false
                                        )}`}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                    >
                                        <SyntaxHighlightedSearchQuery query={searchContext.query} />
                                    </Link>
                                )}
                            </Container>
                        </>
                    )}
                    {isErrorLike(searchContextOrError) && (
                        <Alert variant="danger">
                            Error while loading the search context: <strong>{searchContextOrError.message}</strong>
                        </Alert>
                    )}
                </div>
            </Page>
        </div>
    )
}
