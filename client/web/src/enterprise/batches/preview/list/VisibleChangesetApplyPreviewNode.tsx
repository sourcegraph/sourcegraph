import * as H from 'history'
import React, { useCallback, useState } from 'react'
import {
    ChangesetState,
    VisibleChangesetApplyPreviewFields,
    VisibleChangesetSpecFields,
} from '../../../../graphql-operations'
import { ThemeProps } from '../../../../../../shared/src/theme'
import { queryChangesetSpecFileDiffs as _queryChangesetSpecFileDiffs } from './backend'
import { Link } from '../../../../../../shared/src/components/Link'
import { DiffStat } from '../../../../components/diff/DiffStat'
import { PreviewActions } from './PreviewActions'
import ChevronDownIcon from 'mdi-react/ChevronDownIcon'
import ChevronRightIcon from 'mdi-react/ChevronRightIcon'
import CardTextOutlineIcon from 'mdi-react/CardTextOutlineIcon'
import FileDocumentEditOutlineIcon from 'mdi-react/FileDocumentEditOutlineIcon'
import AccountEditIcon from 'mdi-react/AccountEditIcon'
import CheckboxBlankCircleIcon from 'mdi-react/CheckboxBlankCircleIcon'
import { FileDiffConnection } from '../../../../components/diff/FileDiffConnection'
import { FileDiffNode } from '../../../../components/diff/FileDiffNode'
import { FilteredConnectionQueryArguments } from '../../../../components/FilteredConnection'
import { ChangesetStatusCell } from '../../detail/changesets/ChangesetStatusCell'
import { PreviewNodeIndicator } from './PreviewNodeIndicator'
import classNames from 'classnames'
import { PersonLink } from '../../../../person/PersonLink'
import { PreviewPageAuthenticatedUser } from '../BatchChangePreviewPage'
import { Description } from '../../Description'
import { GitBranchChangesetDescriptionInfo } from './GitBranchChangesetDescriptionInfo'

export interface VisibleChangesetApplyPreviewNodeProps extends ThemeProps {
    node: VisibleChangesetApplyPreviewFields
    history: H.History
    location: H.Location
    authenticatedUser: PreviewPageAuthenticatedUser

    /** Used for testing. **/
    queryChangesetSpecFileDiffs?: typeof _queryChangesetSpecFileDiffs
    /** Expand changeset descriptions, for testing only. **/
    expandChangesetDescriptions?: boolean
}

export const VisibleChangesetApplyPreviewNode: React.FunctionComponent<VisibleChangesetApplyPreviewNodeProps> = ({
    node,
    isLightTheme,
    history,
    location,
    authenticatedUser,

    queryChangesetSpecFileDiffs,
    expandChangesetDescriptions = false,
}) => {
    const [isExpanded, setIsExpanded] = useState(expandChangesetDescriptions)
    const toggleIsExpanded = useCallback<React.MouseEventHandler<HTMLButtonElement>>(
        event => {
            event.preventDefault()
            setIsExpanded(!isExpanded)
        },
        [isExpanded]
    )

    return (
        <>
            <button
                type="button"
                className="btn btn-icon test-batches-expand-preview d-none d-sm-block mx-1"
                aria-label={isExpanded ? 'Collapse section' : 'Expand section'}
                onClick={toggleIsExpanded}
            >
                {isExpanded ? (
                    <ChevronDownIcon className="icon-inline" aria-label="Close section" />
                ) : (
                    <ChevronRightIcon className="icon-inline" aria-label="Expand section" />
                )}
            </button>
            <VisibleChangesetApplyPreviewNodeStatusCell
                node={node}
                className="visible-changeset-apply-preview-node__list-cell d-block d-sm-flex visible-changeset-apply-preview-node__current-state align-self-stretch visible-changeset-apply-preview-node__status-cell"
            />
            <PreviewNodeIndicator node={node} />
            <PreviewActions
                node={node}
                className="visible-changeset-apply-preview-node__list-cell visible-changeset-apply-preview-node__action align-self-stretch"
            />
            <div className="visible-changeset-apply-preview-node__list-cell visible-changeset-apply-preview-node__information align-self-stretch">
                <div className="d-flex flex-column">
                    <ChangesetSpecTitle spec={node} />
                    <div className="mr-2">
                        <RepoLink spec={node} /> <References spec={node} />
                    </div>
                </div>
            </div>
            <div className="d-flex justify-content-center align-content-center align-self-stretch">
                {node.delta.commitMessageChanged && (
                    <div className="d-flex justify-content-center align-items-center flex-column mx-1 visible-changeset-apply-preview-node__commit-change-entry">
                        <CardTextOutlineIcon data-tooltip="The commit message changed" className="icon-inline" />
                        <span className="text-nowrap">Message</span>
                    </div>
                )}
                {node.delta.diffChanged && (
                    <div className="d-flex justify-content-center align-items-center flex-column mx-1 visible-changeset-apply-preview-node__commit-change-entry">
                        <FileDocumentEditOutlineIcon data-tooltip="The diff changed" className="icon-inline" />
                        <span className="text-nowrap">Diff</span>
                    </div>
                )}
                {(node.delta.authorNameChanged || node.delta.authorEmailChanged) && (
                    <div className="d-flex justify-content-center align-items-center flex-column mx-1 visible-changeset-apply-preview-node__commit-change-entry">
                        <AccountEditIcon data-tooltip="The commit author details changed" className="icon-inline" />
                        <span className="text-nowrap">Author</span>
                    </div>
                )}
            </div>
            <div className="visible-changeset-apply-preview-node__list-cell d-flex justify-content-center align-items-center align-self-stretch">
                <ApplyDiffStat spec={node} />
            </div>
            {/* The button for expanding the information used on xs devices. */}
            <button
                type="button"
                aria-label={isExpanded ? 'Collapse section' : 'Expand section'}
                onClick={toggleIsExpanded}
                className="visible-changeset-apply-preview-node__show-details btn btn-outline-secondary d-block d-sm-none test-batches-expand-preview"
            >
                {isExpanded ? (
                    <ChevronDownIcon className="icon-inline" aria-label="Close section" />
                ) : (
                    <ChevronRightIcon className="icon-inline" aria-label="Expand section" />
                )}{' '}
                {isExpanded ? 'Hide' : 'Show'} details
            </button>
            {isExpanded && (
                <>
                    <div className="visible-changeset-apply-preview-node__expanded-section visible-changeset-apply-preview-node__bg-expanded pt-4">
                        <ExpandedSection
                            node={node}
                            history={history}
                            isLightTheme={isLightTheme}
                            location={location}
                            authenticatedUser={authenticatedUser}
                            queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
                        />
                    </div>
                </>
            )}
        </>
    )
}

type SelectedTab = 'diff' | 'description' | 'commits'

const ExpandedSection: React.FunctionComponent<
    {
        node: VisibleChangesetApplyPreviewFields
        history: H.History
        location: H.Location
        authenticatedUser: PreviewPageAuthenticatedUser

        /** Used for testing. **/
        queryChangesetSpecFileDiffs?: typeof _queryChangesetSpecFileDiffs
    } & ThemeProps
> = ({ node, history, isLightTheme, location, authenticatedUser, queryChangesetSpecFileDiffs }) => {
    const [selectedTab, setSelectedTab] = useState<SelectedTab>('diff')
    const onSelectDiff = useCallback<React.MouseEventHandler>(event => {
        event.preventDefault()
        setSelectedTab('diff')
    }, [])
    const onSelectDescription = useCallback<React.MouseEventHandler>(event => {
        event.preventDefault()
        setSelectedTab('description')
    }, [])
    const onSelectCommits = useCallback<React.MouseEventHandler>(event => {
        event.preventDefault()
        setSelectedTab('commits')
    }, [])
    if (node.targets.__typename === 'VisibleApplyPreviewTargetsDetach') {
        return (
            <div className="alert alert-info mb-0">
                When run, the changeset <strong>{node.targets.changeset.title}</strong> in repo{' '}
                <strong>{node.targets.changeset.repository.name}</strong> will be removed from this batch change.
            </div>
        )
    }
    if (node.targets.changesetSpec.description.__typename === 'ExistingChangesetReference') {
        return (
            <div className="alert alert-info mb-0">
                When run, the changeset with ID <strong>{node.targets.changesetSpec.description.externalID}</strong>{' '}
                will be imported from <strong>{node.targets.changesetSpec.description.baseRepository.name}</strong>.
            </div>
        )
    }
    return (
        <>
            <div className="overflow-auto mb-4">
                <ul className="nav nav-tabs d-inline-flex d-sm-flex flex-nowrap text-nowrap">
                    <li className="nav-item">
                        <a
                            href=""
                            onClick={onSelectDiff}
                            className={classNames(
                                'nav-link',
                                selectedTab === 'diff' &&
                                    'active visible-changeset-apply-preview-node__tab-link--active'
                            )}
                        >
                            Changed files
                            {node.delta.diffChanged && (
                                <small className="text-warning ml-2" data-tooltip="Changes in this tab">
                                    <CheckboxBlankCircleIcon className="icon-inline visible-changeset-apply-preview-node__change-indicator" />
                                </small>
                            )}
                        </a>
                    </li>
                    <li className="nav-item">
                        <a
                            href=""
                            onClick={onSelectDescription}
                            className={classNames(
                                'nav-link',
                                selectedTab === 'description' &&
                                    'active visible-changeset-apply-preview-node__tab-link--active'
                            )}
                        >
                            Description
                            {(node.delta.titleChanged || node.delta.bodyChanged) && (
                                <small className="text-warning ml-2" data-tooltip="Changes in this tab">
                                    <CheckboxBlankCircleIcon className="icon-inline visible-changeset-apply-preview-node__change-indicator" />
                                </small>
                            )}
                        </a>
                    </li>
                    <li className="nav-item">
                        <a
                            href=""
                            onClick={onSelectCommits}
                            className={classNames(
                                'nav-link',
                                selectedTab === 'commits' &&
                                    'active visible-changeset-apply-preview-node__tab-link--active'
                            )}
                        >
                            Commits
                            {(node.delta.authorEmailChanged ||
                                node.delta.authorNameChanged ||
                                node.delta.commitMessageChanged) && (
                                <small className="text-warning ml-2" data-tooltip="Changes in this tab">
                                    <CheckboxBlankCircleIcon className="icon-inline visible-changeset-apply-preview-node__change-indicator" />
                                </small>
                            )}
                        </a>
                    </li>
                </ul>
            </div>
            {selectedTab === 'diff' && (
                <>
                    {node.delta.diffChanged && (
                        <div className="alert alert-warning">
                            The files in this changeset have been altered from the previous version. These changes will
                            be pushed to the target branch.
                        </div>
                    )}
                    <ChangesetSpecFileDiffConnection
                        history={history}
                        isLightTheme={isLightTheme}
                        location={location}
                        spec={node.targets.changesetSpec}
                        queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
                    />
                </>
            )}
            {selectedTab === 'description' && (
                <>
                    {node.targets.__typename === 'VisibleApplyPreviewTargetsUpdate' &&
                        node.delta.bodyChanged &&
                        node.targets.changeset.currentSpec?.description.__typename ===
                            'GitBranchChangesetDescription' && (
                            <>
                                <h3 className="text-muted">
                                    <del>{node.targets.changeset.currentSpec.description.title}</del>
                                </h3>
                                <del className="text-muted">
                                    <Description
                                        history={history}
                                        description={node.targets.changeset.currentSpec.description.body}
                                    />
                                </del>
                            </>
                        )}
                    <h3>
                        {node.targets.changesetSpec.description.title}{' '}
                        <small>
                            by{' '}
                            <PersonLink
                                person={
                                    node.targets.__typename === 'VisibleApplyPreviewTargetsUpdate' &&
                                    node.targets.changeset.author
                                        ? node.targets.changeset.author
                                        : {
                                              email: authenticatedUser.email,
                                              displayName: authenticatedUser.displayName || authenticatedUser.username,
                                              user: authenticatedUser,
                                          }
                                }
                            />
                        </small>
                    </h3>
                    <Description history={history} description={node.targets.changesetSpec.description.body} />
                </>
            )}
            {selectedTab === 'commits' && <GitBranchChangesetDescriptionInfo node={node} />}
        </>
    )
}

const ChangesetSpecFileDiffConnection: React.FunctionComponent<
    {
        spec: VisibleChangesetSpecFields
        history: H.History
        location: H.Location

        /** Used for testing. **/
        queryChangesetSpecFileDiffs?: typeof _queryChangesetSpecFileDiffs
    } & ThemeProps
> = ({ spec, history, location, isLightTheme, queryChangesetSpecFileDiffs = _queryChangesetSpecFileDiffs }) => {
    /** Fetches the file diffs for the changeset */
    const queryFileDiffs = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryChangesetSpecFileDiffs({
                after: args.after ?? null,
                first: args.first ?? null,
                changesetSpec: spec.id,
                isLightTheme,
            }),
        [spec.id, isLightTheme, queryChangesetSpecFileDiffs]
    )
    return (
        <FileDiffConnection
            listClassName="list-group list-group-flush"
            noun="changed file"
            pluralNoun="changed files"
            queryConnection={queryFileDiffs}
            nodeComponent={FileDiffNode}
            nodeComponentProps={{
                history,
                location,
                isLightTheme,
                persistLines: true,
                lineNumbers: true,
            }}
            defaultFirst={15}
            hideSearch={true}
            noSummaryIfAllNodesVisible={true}
            history={history}
            location={location}
            useURLQuery={false}
            cursorPaging={true}
        />
    )
}

const ChangesetSpecTitle: React.FunctionComponent<{ spec: VisibleChangesetApplyPreviewFields }> = ({ spec }) => {
    if (spec.targets.__typename === 'VisibleApplyPreviewTargetsDetach') {
        return <h3>{spec.targets.changeset.title}</h3>
    }
    if (spec.targets.changesetSpec.description.__typename === 'ExistingChangesetReference') {
        return <h3>Import changeset #{spec.targets.changesetSpec.description.externalID}</h3>
    }
    if (
        spec.operations.length === 0 ||
        !spec.delta.titleChanged ||
        spec.targets.__typename === 'VisibleApplyPreviewTargetsAttach'
    ) {
        return <h3>{spec.targets.changesetSpec.description.title}</h3>
    }
    return (
        <h3>
            <del className="text-muted">{spec.targets.changeset.title}</del>{' '}
            {spec.targets.changesetSpec.description.title}
        </h3>
    )
}

const RepoLink: React.FunctionComponent<{ spec: VisibleChangesetApplyPreviewFields }> = ({ spec }) => {
    let to: string
    let name: string
    if (
        spec.targets.__typename === 'VisibleApplyPreviewTargetsAttach' ||
        spec.targets.__typename === 'VisibleApplyPreviewTargetsUpdate'
    ) {
        to = spec.targets.changesetSpec.description.baseRepository.url
        name = spec.targets.changesetSpec.description.baseRepository.name
    } else {
        to = spec.targets.changeset.repository.url
        name = spec.targets.changeset.repository.name
    }
    return (
        <Link to={to} target="_blank" rel="noopener noreferrer" className="d-block d-sm-inline">
            {name}
        </Link>
    )
}

const References: React.FunctionComponent<{ spec: VisibleChangesetApplyPreviewFields }> = ({ spec }) => {
    if (spec.targets.__typename === 'VisibleApplyPreviewTargetsDetach') {
        return null
    }
    if (spec.targets.changesetSpec.description.__typename !== 'GitBranchChangesetDescription') {
        return null
    }
    return (
        <div className="d-block d-sm-inline-block">
            {spec.delta.baseRefChanged &&
                spec.targets.__typename === 'VisibleApplyPreviewTargetsUpdate' &&
                spec.targets.changeset.currentSpec?.description.__typename === 'GitBranchChangesetDescription' && (
                    <del className="badge badge-danger mr-2">
                        {spec.targets.changeset.currentSpec?.description.baseRef}
                    </del>
                )}
            <span className="badge badge-primary">{spec.targets.changesetSpec.description.baseRef}</span> &larr;{' '}
            <span className="badge badge-primary">{spec.targets.changesetSpec.description.headRef}</span>
        </div>
    )
}

const ApplyDiffStat: React.FunctionComponent<{ spec: VisibleChangesetApplyPreviewFields }> = ({ spec }) => {
    let diffStat: { added: number; changed: number; deleted: number }
    if (spec.targets.__typename === 'VisibleApplyPreviewTargetsDetach') {
        if (!spec.targets.changeset.diffStat) {
            return null
        }
        diffStat = spec.targets.changeset.diffStat
    } else if (spec.targets.changesetSpec.description.__typename !== 'GitBranchChangesetDescription') {
        return null
    } else {
        diffStat = spec.targets.changesetSpec.description.diffStat
    }
    return <DiffStat {...diffStat} expandedCounts={true} separateLines={true} />
}

const VisibleChangesetApplyPreviewNodeStatusCell: React.FunctionComponent<
    Pick<VisibleChangesetApplyPreviewNodeProps, 'node'> & { className?: string }
> = ({ node, className }) => {
    if (node.targets.__typename === 'VisibleApplyPreviewTargetsAttach') {
        return <ChangesetStatusCell state={ChangesetState.UNPUBLISHED} className={className} />
    }
    return <ChangesetStatusCell state={node.targets.changeset.state} className={className} />
}
