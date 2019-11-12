import { Hoverifier } from '@sourcegraph/codeintellify'
import * as H from 'history'
import ChevronDownIcon from 'mdi-react/ChevronDownIcon'
import ChevronUpIcon from 'mdi-react/ChevronUpIcon'
import * as React from 'react'
import { Link } from 'react-router-dom'
import { ActionItemAction } from '../../../shared/src/actions/ActionItem'
import { HoverMerged } from '../../../shared/src/api/client/types/hover'
import * as GQL from '../../../shared/src/graphql/schema'
import { FileSpec, RepoSpec, ResolvedRevSpec, RevSpec } from '../../../shared/src/util/url'
import { DiffStat } from './DiffStat'
import { FileDiffHunks } from './FileDiffHunks'
import { ThemeProps } from '../../../shared/src/theme'
import { ExtensionsControllerProps } from '../../../shared/src/extensions/controller'

export interface FileDiffNodeProps extends Partial<ExtensionsControllerProps>, ThemeProps {
    node: GQL.IFileDiff | GQL.IPreviewFileDiff
    lineNumbers: boolean
    className?: string
    location: H.Location
    history: H.History

    /** The base repository and revision. */
    base?: { repoName: string; repoID: GQL.ID; rev: string; commitID: string }

    /** The head repository and revision. */
    head?: { repoName: string; repoID: GQL.ID; rev: string; commitID: string }

    hoverifier?: Hoverifier<RepoSpec & RevSpec & FileSpec & ResolvedRevSpec, HoverMerged, ActionItemAction>

    /** Don't reflect selected line in url */
    disableLinePersistance?: boolean
}

interface State {
    expanded: boolean
}

/** A file diff. */
export class FileDiffNode extends React.PureComponent<FileDiffNodeProps, State> {
    public state: State = { expanded: true }

    public render(): JSX.Element | null {
        const node = this.props.node

        let path: React.ReactFragment
        if (node.newPath && (node.newPath === node.oldPath || !node.oldPath)) {
            path = <span title={node.newPath}>{node.newPath}</span>
        } else if (node.newPath && node.oldPath && node.newPath !== node.oldPath) {
            path = (
                <span title={`${node.oldPath} ⟶ ${node.newPath}`}>
                    {node.oldPath} ⟶ {node.newPath}
                </span>
            )
        } else {
            // By process of elimination (that TypeScript is unfortunately unable to infer, except
            // by reorganizing this code in a way that's much more complex to humans), node.oldPath
            // is non-null.
            path = <span title={node.oldPath!}>{node.oldPath}</span>
        }

        const renderAnchor = node.__typename !== 'PreviewFileDiff'
        const anchor = `diff-${node.internalID}`

        return (
            <>
                {renderAnchor && <a id={anchor} />}
                <div className={`file-diff-node card ${this.props.className || ''}`}>
                    <div className="card-header file-diff-node__header">
                        <div className="file-diff-node__header-path-stat">
                            <DiffStat
                                added={node.stat.added}
                                changed={node.stat.changed}
                                deleted={node.stat.deleted}
                                className="file-diff-node__header-stat"
                            />
                            {renderAnchor ? (
                                <Link
                                    to={{ ...this.props.location, hash: anchor }}
                                    className="file-diff-node__header-path"
                                >
                                    {path}
                                </Link>
                            ) : (
                                <span>{path}</span>
                            )}
                        </div>
                        <div className="file-diff-node__header-actions">
                            {node.__typename === 'FileDiff' && (
                                <Link
                                    to={node.mostRelevantFile.url}
                                    className="btn btn-sm"
                                    data-tooltip="View file at revision"
                                >
                                    View
                                </Link>
                            )}
                            <button type="button" className="btn btn-sm btn-icon ml-2" onClick={this.toggleExpand}>
                                {this.state.expanded ? (
                                    <ChevronDownIcon className="icon-inline" />
                                ) : (
                                    <ChevronUpIcon className="icon-inline" />
                                )}
                            </button>
                        </div>
                    </div>
                    {this.state.expanded && (
                        <FileDiffHunks
                            {...this.props}
                            className="file-diff-node__hunks"
                            fileDiffAnchor={anchor}
                            base={
                                this.props.base && {
                                    ...this.props.base,
                                    filePath: node.oldPath,
                                }
                            }
                            head={
                                this.props.head && {
                                    ...this.props.head,
                                    filePath: node.newPath,
                                }
                            }
                            hunks={node.hunks}
                            lineNumbers={this.props.lineNumbers}
                            hoverifier={this.props.hoverifier}
                        />
                    )}
                </div>
            </>
        )
    }

    private toggleExpand = (): void => this.setState(prevState => ({ expanded: !prevState.expanded }))
}
