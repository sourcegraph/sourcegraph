import * as React from 'react'

import classNames from 'classnames'
import { isEqual } from 'lodash'
import { RouteComponentProps } from 'react-router'
import { merge, Observable, of, Subject, Subscription } from 'rxjs'
import { catchError, distinctUntilChanged, filter, map, switchMap, tap, withLatestFrom } from 'rxjs/operators'

import { ErrorAlert } from '@sourcegraph/branded/src/components/alerts'
import { HoverMerged } from '@sourcegraph/client-api'
import { HoveredToken, createHoverifier, Hoverifier, HoverState } from '@sourcegraph/codeintellify'
import {
    asError,
    createAggregateError,
    ErrorLike,
    isErrorLike,
    isDefined,
    memoizeObservable,
    property,
} from '@sourcegraph/common'
import { gql } from '@sourcegraph/http-client'
import { ActionItemAction } from '@sourcegraph/shared/src/actions/ActionItem'
import { ExtensionsControllerProps } from '@sourcegraph/shared/src/extensions/controller'
import { getHoverActions } from '@sourcegraph/shared/src/hover/actions'
import { HoverContext } from '@sourcegraph/shared/src/hover/HoverOverlay'
import { getModeFromPath } from '@sourcegraph/shared/src/languages'
import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import * as GQL from '@sourcegraph/shared/src/schema'
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'
import {
    FileSpec,
    ModeSpec,
    RepoSpec,
    ResolvedRevisionSpec,
    RevisionSpec,
    UIPositionSpec,
} from '@sourcegraph/shared/src/util/url'
import { LoadingSpinner } from '@sourcegraph/wildcard'

import { getHover, getDocumentHighlights } from '../../backend/features'
import { requestGraphQL } from '../../backend/graphql'
import { FileDiffConnection } from '../../components/diff/FileDiffConnection'
import { FileDiffNode } from '../../components/diff/FileDiffNode'
import { FilteredConnectionQueryArguments } from '../../components/FilteredConnection'
import { PageTitle } from '../../components/PageTitle'
import { WebHoverOverlay } from '../../components/shared'
import {
    ExternalLinkFields,
    GitCommitFields,
    RepositoryCommitResult,
    RepositoryCommitVariables,
    RepositoryFields,
    Scalars,
} from '../../graphql-operations'
import { GitCommitNode } from '../commits/GitCommitNode'
import { gitCommitFragment } from '../commits/RepositoryCommitsPage'
import { queryRepositoryComparisonFileDiffs } from '../compare/RepositoryCompareDiffPage'

import styles from './RepositoryCommitPage.module.scss'

const queryCommit = memoizeObservable(
    (args: { repo: Scalars['ID']; revspec: string }): Observable<GitCommitFields> =>
        requestGraphQL<RepositoryCommitResult, RepositoryCommitVariables>(
            gql`
                query RepositoryCommit($repo: ID!, $revspec: String!) {
                    node(id: $repo) {
                        __typename
                        ... on Repository {
                            commit(rev: $revspec) {
                                __typename # necessary so that isErrorLike(x) is false when x: GitCommitFields
                                ...GitCommitFields
                            }
                        }
                    }
                }
                ${gitCommitFragment}
            `,
            args
        ).pipe(
            map(({ data, errors }) => {
                if (!data || !data.node) {
                    throw createAggregateError(errors)
                }
                if (data.node.__typename !== 'Repository') {
                    throw new Error(`Node is a ${data.node.__typename}, not a Repository`)
                }
                if (!data.node.commit) {
                    // Filter out any revision not found errors, they usually come in multiples when searching for a commit, we want to replace all of them with 1 "Commit not found" error
                    const errorsWithoutRevisionError = errors?.filter(
                        error => !error.message.includes('revision not found')
                    )

                    const revisionErrorsFiltered =
                        errors && errorsWithoutRevisionError && errorsWithoutRevisionError.length < errors?.length

                    // If there are no other errors left (or there wasn't any errors to begin with throw out a Commit not found error
                    if (!errorsWithoutRevisionError || errorsWithoutRevisionError.length === 0) {
                        throw new Error('Commit not found')
                    }

                    // if we found at least 1 "revision nor found error" add "Commit not found" to the errors
                    if (revisionErrorsFiltered) {
                        throw createAggregateError([new Error('Commit not found'), ...errorsWithoutRevisionError])
                    }

                    // no "revision not found" errors, throw the other errors
                    throw createAggregateError(errorsWithoutRevisionError)
                }
                return data.node.commit
            })
        ),
    args => `${args.repo}:${args.revspec}`
)

interface Props
    extends RouteComponentProps<{ revspec: string }>,
        TelemetryProps,
        PlatformContextProps,
        ExtensionsControllerProps,
        ThemeProps,
        SettingsCascadeProps {
    repo: RepositoryFields
    onDidUpdateExternalLinks: (externalLinks: ExternalLinkFields[] | undefined) => void
}

export type DiffMode = 'split' | 'unified' | 'difft'

interface State extends HoverState<HoverContext, HoverMerged, ActionItemAction> {
    /** The commit, undefined while loading, or an error. */
    commitOrError?: GitCommitFields | ErrorLike
    /** The visualization mode for file diff */
    diffMode: DiffMode
    rawDiff?: string
}

const DIFF_MODE_VISUALIZER = 'diff-mode-visualizer'

/** Displays a commit. */
export class RepositoryCommitPage extends React.Component<Props, State> {
    private componentUpdates = new Subject<Props>()

    /** Emits whenever the ref callback for the hover element is called */
    private hoverOverlayElements = new Subject<HTMLElement | null>()
    private nextOverlayElement = (element: HTMLElement | null): void => this.hoverOverlayElements.next(element)

    /** Emits whenever the ref callback for the hover element is called */
    private repositoryCommitPageElements = new Subject<HTMLElement | null>()
    private nextRepositoryCommitPageElement = (element: HTMLElement | null): void =>
        this.repositoryCommitPageElements.next(element)
    private subscriptions = new Subscription()
    private hoverifier: Hoverifier<
        RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec,
        HoverMerged,
        ActionItemAction
    >

    constructor(props: Props) {
        super(props)
        this.hoverifier = createHoverifier<
            RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec,
            HoverMerged,
            ActionItemAction
        >({
            hoverOverlayElements: this.hoverOverlayElements,
            hoverOverlayRerenders: this.componentUpdates.pipe(
                withLatestFrom(this.hoverOverlayElements, this.repositoryCommitPageElements),
                map(([, hoverOverlayElement, repositoryCommitPageElement]) => ({
                    hoverOverlayElement,
                    // The root component element is guaranteed to be rendered after a componentDidUpdate
                    relativeElement: repositoryCommitPageElement!,
                })),
                // Can't reposition HoverOverlay if it wasn't rendered
                filter(property('hoverOverlayElement', isDefined))
            ),
            getHover: hoveredToken => getHover(this.getLSPTextDocumentPositionParams(hoveredToken), this.props),
            getDocumentHighlights: hoveredToken =>
                getDocumentHighlights(this.getLSPTextDocumentPositionParams(hoveredToken), this.props),
            getActions: context => getHoverActions(this.props, context),
        })
        this.subscriptions.add(this.hoverifier)
        this.onHandleDiffMode = this.onHandleDiffMode.bind(this)
        this.state = {
            ...this.hoverifier.hoverState,
            diffMode: (localStorage.getItem(DIFF_MODE_VISUALIZER) as DiffMode | null) || 'unified',
        }

        this.subscriptions.add(
            this.hoverifier.hoverStateUpdates.subscribe(update => {
                this.setState(update)
            })
        )
    }

    private getLSPTextDocumentPositionParams(
        hoveredToken: HoveredToken & RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec
    ): RepoSpec & RevisionSpec & ResolvedRevisionSpec & FileSpec & UIPositionSpec & ModeSpec {
        return {
            repoName: hoveredToken.repoName,
            revision: hoveredToken.revision,
            filePath: hoveredToken.filePath,
            commitID: hoveredToken.commitID,
            position: hoveredToken,
            mode: getModeFromPath(hoveredToken.filePath || ''),
        }
    }

    private onHandleDiffMode = (mode: DiffMode): void => {
        localStorage.setItem(DIFF_MODE_VISUALIZER, mode)
        this.setState({ diffMode: mode })
    }

    public componentDidMount(): void {
        this.props.telemetryService.logViewEvent('RepositoryCommit')
        this.subscriptions.add(
            this.componentUpdates
                .pipe(
                    distinctUntilChanged(
                        (a, b) => a.repo.id === b.repo.id && a.match.params.revspec === b.match.params.revspec
                    ),
                    switchMap(({ repo, match }) =>
                        merge(
                            of({ commitOrError: undefined }),
                            queryCommit({ repo: repo.id, revspec: match.params.revspec }).pipe(
                                catchError(error => [asError(error)]),
                                map(commitOrError => ({ commitOrError })),
                                tap(({ commitOrError }) => {
                                    if (isErrorLike(commitOrError)) {
                                        this.props.onDidUpdateExternalLinks(undefined)
                                    } else {
                                        this.props.onDidUpdateExternalLinks(commitOrError.externalURLs)
                                    }
                                })
                            )
                        )
                    )
                )
                .subscribe(
                    stateUpdate => this.setState(stateUpdate),
                    error => console.error(error)
                )
        )
        this.componentUpdates.next(this.props)
    }

    public shouldComponentUpdate(nextProps: Readonly<Props>, nextState: Readonly<State>): boolean {
        return !isEqual(this.props, nextProps) || !isEqual(this.state, nextState)
    }

    public componentDidUpdate(): void {
        this.componentUpdates.next(this.props)
    }

    public componentWillUnmount(): void {
        this.props.onDidUpdateExternalLinks(undefined)
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <div className={classNames('p-3', styles.repositoryCommitPage)} ref={this.nextRepositoryCommitPageElement}>
                <PageTitle
                    title={
                        this.state.commitOrError && !isErrorLike(this.state.commitOrError)
                            ? this.state.commitOrError.subject
                            : `Commit ${this.props.match.params.revspec}`
                    }
                />
                {this.state.commitOrError === undefined ? (
                    <LoadingSpinner className="mt-2" />
                ) : isErrorLike(this.state.commitOrError) ? (
                    <ErrorAlert className="mt-2" error={this.state.commitOrError} />
                ) : (
                    <>
                        <div className="border-bottom pb-2">
                            <div>
                                <GitCommitNode
                                    node={this.state.commitOrError}
                                    expandCommitMessageBody={true}
                                    showSHAAndParentsRow={true}
                                    diffMode={this.state.diffMode}
                                    onHandleDiffMode={this.onHandleDiffMode}
                                    className={styles.gitCommitNode}
                                />
                            </div>
                        </div>
                        {this.state.diffMode === 'difft' ? (
                            <div
                                // eslint-disable-next-line react/forbid-dom-props
                                style={{
                                    whiteSpace: 'pre-wrap',
                                    fontFamily: 'monospace',
                                }}
                                dangerouslySetInnerHTML={{ __html: this.state.rawDiff || '' }}
                            />
                        ) : null}
                        <FileDiffConnection
                            listClassName="list-group list-group-flush"
                            noun="changed file"
                            pluralNoun="changed files"
                            queryConnection={this.queryDiffs}
                            nodeComponent={FileDiffNode}
                            nodeComponentProps={{
                                ...this.props,
                                extensionInfo: {
                                    base: {
                                        repoName: this.props.repo.name,
                                        repoID: this.props.repo.id,
                                        revision: commitParentOrEmpty(this.state.commitOrError),
                                        commitID: commitParentOrEmpty(this.state.commitOrError),
                                    },
                                    head: {
                                        repoName: this.props.repo.name,
                                        repoID: this.props.repo.id,
                                        revision: this.state.commitOrError.oid,
                                        commitID: this.state.commitOrError.oid,
                                    },
                                    hoverifier: this.hoverifier,
                                    extensionsController: this.props.extensionsController,
                                },
                                lineNumbers: true,
                                diffMode: this.state.diffMode,
                            }}
                            updateOnChange={`${this.props.repo.id}:${this.state.commitOrError.oid}`}
                            defaultFirst={15}
                            hideSearch={true}
                            noSummaryIfAllNodesVisible={true}
                            history={this.props.history}
                            location={this.props.location}
                            cursorPaging={true}
                        />
                        )
                    </>
                )}
                {this.state.hoverOverlayProps && (
                    <WebHoverOverlay
                        {...this.props}
                        {...this.state.hoverOverlayProps}
                        nav={url => this.props.history.push(url)}
                        hoveredTokenElement={this.state.hoveredTokenElement}
                        telemetryService={this.props.telemetryService}
                        hoverRef={this.nextOverlayElement}
                    />
                )}
            </div>
        )
    }

    private queryDiffs = (args: FilteredConnectionQueryArguments): Observable<GQL.IFileDiffConnection> =>
        queryRepositoryComparisonFileDiffs({
            ...args,
            repo: this.props.repo.id,
            base: commitParentOrEmpty(this.state.commitOrError as GitCommitFields),
            head: (this.state.commitOrError as GitCommitFields).oid,
        }).pipe(
            tap(value => {
                this.setState({ rawDiff: value.rawDiff })
            })
        )
}

function commitParentOrEmpty(commit: GitCommitFields): string {
    // 4b825dc642cb6eb9a060e54bf8d69288fbee4904 is `git hash-object -t tree /dev/null`, which is used as the base
    // when computing the `git diff` of the root commit.
    return commit.parents.length > 0 ? commit.parents[0].oid : '4b825dc642cb6eb9a060e54bf8d69288fbee4904'
}
