import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import * as H from 'history'
import * as React from 'react'
import { EMPTY, merge, of, Subject, Subscription } from 'rxjs'
import {
    catchError,
    debounceTime,
    delay,
    distinctUntilChanged,
    filter,
    mergeMap,
    share,
    switchMap,
    takeUntil,
} from 'rxjs/operators'
import { asError, ErrorLike, isErrorLike } from '../../../shared/src/util/errors'
import { AbsoluteRepo } from '../../../shared/src/util/url'
import { fetchTreeEntries } from '../repo/backend'
import { ChildTreeLayer } from './ChildTreeLayer'
import { TreeNode } from './Tree'
import { hasSingleChild, singleChildEntriesToGitTree, SingleChildGitTree } from './util'
import { ErrorAlert } from '../components/alerts'
import { TreeFields } from '../graphql-operations'
import { getFileDecorations } from '../backend/features'
import { ExtensionsControllerProps } from '../../../shared/src/extensions/controller'
import { ThemeProps } from '../../../shared/src/theme'
import { FileDecorationsByPath } from '../../../shared/src/api/extension/flatExtensionApi'

const maxEntries = 2500

const errorWidth = (width?: string): { width: string } => ({
    width: width ? `${width}px` : 'auto',
})

export interface TreeRootProps extends AbsoluteRepo, ExtensionsControllerProps, ThemeProps {
    location: H.Location
    activeNode: TreeNode
    activePath: string
    depth: number
    expandedTrees: string[]
    parent: TreeNode | null
    parentPath?: string
    index: number
    isExpanded: boolean
    selectedNode: TreeNode
    sizeKey: string
    onHover?: (filePath: string) => void
    onSelect: (node: TreeNode) => void
    onToggleExpand: (path: string, expanded: boolean, node: TreeNode) => void
    setChildNodes: (node: TreeNode, index: number) => void
    setActiveNode: (node: TreeNode) => void
}

const LOADING = 'loading' as const
interface TreeRootState {
    treeOrError?: typeof LOADING | TreeFields | ErrorLike
    fileDecorationsByPath: FileDecorationsByPath
}

export class TreeRoot extends React.Component<TreeRootProps, TreeRootState> {
    public node: TreeNode
    private subscriptions = new Subscription()
    private componentUpdates = new Subject<TreeRootProps>()
    private rowHovers = new Subject<string>()

    constructor(props: TreeRootProps) {
        super(props)
        this.node = {
            index: this.props.index,
            parent: this.props.parent,
            childNodes: [],
            path: '',
            url: '',
        }
        this.state = {
            fileDecorationsByPath: {},
        }
    }

    public componentDidMount(): void {
        // Set this row as a childNode of its TreeLayer parent
        this.props.setChildNodes(this.node, this.node.index)

        const treeOrErrors = this.componentUpdates.pipe(
            distinctUntilChanged(
                (a, b) =>
                    a.repoName === b.repoName &&
                    a.revision === b.revision &&
                    a.commitID === b.commitID &&
                    a.parentPath === b.parentPath &&
                    a.isExpanded === b.isExpanded &&
                    a.location === b.location
            ),
            filter(props => props.isExpanded),
            switchMap(props => {
                const treeFetch = fetchTreeEntries({
                    repoName: props.repoName,
                    revision: props.revision,
                    commitID: props.commitID,
                    filePath: props.parentPath || '',
                    first: maxEntries,
                }).pipe(
                    catchError(error => [asError(error)]),
                    share()
                )
                return merge(treeFetch, of(LOADING).pipe(delay(300), takeUntil(treeFetch)))
            })
        )

        this.subscriptions.add(
            treeOrErrors.subscribe(
                treeOrError => {
                    // clear file decorations before latest file decorations come
                    this.setState({ treeOrError, fileDecorationsByPath: {} })
                },
                error => console.error(error)
            )
        )

        this.subscriptions.add(
            treeOrErrors
                .pipe(
                    switchMap(treeOrError =>
                        treeOrError !== 'loading' && !isErrorLike(treeOrError)
                            ? getFileDecorations({
                                  files: treeOrError.entries,
                                  repoName: this.props.repoName,
                                  commitID: this.props.commitID,
                                  extensionsController: this.props.extensionsController,
                                  parentNodeUri: treeOrError.url,
                              })
                            : EMPTY
                    )
                )
                .subscribe(fileDecorationsByPath => {
                    this.setState({ fileDecorationsByPath })
                })
        )

        // This handles pre-fetching when a user
        // hovers over a directory. The `subscribe` is empty because
        // we simply want to cache the network request.
        this.subscriptions.add(
            this.rowHovers
                .pipe(
                    debounceTime(100),
                    mergeMap(path =>
                        fetchTreeEntries({
                            repoName: this.props.repoName,
                            revision: this.props.revision,
                            commitID: this.props.commitID,
                            filePath: path,
                            first: maxEntries,
                        }).pipe(catchError(error => [asError(error)]))
                    )
                )
                .subscribe()
        )

        // When we're at the root tree layer, fetch the tree contents on mount.
        this.componentUpdates.next(this.props)
    }

    public componentDidUpdate(): void {
        this.componentUpdates.next(this.props)
    }
    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        const { treeOrError } = this.state

        let singleChildTreeEntry = {} as SingleChildGitTree
        if (
            treeOrError &&
            treeOrError !== LOADING &&
            !isErrorLike(treeOrError) &&
            hasSingleChild(treeOrError.entries)
        ) {
            singleChildTreeEntry = singleChildEntriesToGitTree(treeOrError.entries)
        }

        return (
            <>
                {isErrorLike(treeOrError) ? (
                    <ErrorAlert
                        // needed because of dynamic styling
                        style={errorWidth(localStorage.getItem(this.props.sizeKey) ? this.props.sizeKey : undefined)}
                        className="tree__row tree__row-alert"
                        prefix="Error loading tree"
                        error={treeOrError}
                    />
                ) : (
                    <table className="tree-layer" tabIndex={0}>
                        <tbody>
                            <tr>
                                <td className="tree__cell">
                                    {treeOrError === LOADING ? (
                                        <div className="tree__row-loader">
                                            <LoadingSpinner className="icon-inline tree-page__entries-loader" />
                                            Loading tree
                                        </div>
                                    ) : (
                                        treeOrError && (
                                            <ChildTreeLayer
                                                {...this.props}
                                                parent={this.node}
                                                depth={-1 as number}
                                                entries={treeOrError.entries}
                                                singleChildTreeEntry={singleChildTreeEntry}
                                                childrenEntries={singleChildTreeEntry.children}
                                                onHover={this.fetchChildContents}
                                                setChildNodes={this.setChildNode}
                                                fileDecorationsByPath={this.state.fileDecorationsByPath}
                                            />
                                        )
                                    )}
                                </td>
                            </tr>
                        </tbody>
                    </table>
                )}
            </>
        )
    }
    /**
     * Prefetches the children of hovered tree rows. Gets passed from the root tree layer to child tree layers
     * through the onHover prop. This method only gets called on the root tree layer component so we can debounce
     * the hover prefetch requests.
     */
    private fetchChildContents = (path: string): void => {
        this.rowHovers.next(path)
    }
    private setChildNode = (node: TreeNode, index: number): void => {
        this.node.childNodes[index] = node
    }
}
