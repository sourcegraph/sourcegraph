import * as H from 'history'
import { isEqual } from 'lodash'
import * as React from 'react'
import { Subject, Subscription } from 'rxjs'
import { distinctUntilChanged, startWith } from 'rxjs/operators'
import { Key } from 'ts-key-enum'
import { ExtensionsControllerProps } from '../../../shared/src/extensions/controller'
import { ThemeProps } from '../../../shared/src/theme'
import { AbsoluteRepo } from '../../../shared/src/util/url'
import { dirname } from '../util/path'
import { TreeRoot } from './TreeRoot'
import { getDomElement } from './util'

interface Props extends AbsoluteRepo, ExtensionsControllerProps, ThemeProps {
    history: H.History
    location: H.Location
    scrollRootSelector?: string

    /** The tree entry that is currently active, or '' if none (which means the root). */
    activePath: string

    /** Whether the active path is a directory (including the root directory). False if it is a file. */
    activePathIsDir: boolean
    /** The localStorage key that stores the current size of the (resizable) RepoRevisionSidebar. */
    sizeKey: string
}

interface State {
    /**
     * The root of the tree to show, or undefined for the root.
     *
     * This is initially the directory containing the first file viewed, but it can be changed to be an ancestor of
     * itself if the user browses to higher levels in the tree.
     */
    parentPath?: string
    /** Directories (including descendents multiple levels below this dir) that are expanded. */
    resolveTo: string[]
    /** The tree node currently in focus */
    selectedNode: TreeNode
    /** The tree node of the file or directory currently being viewed */
    activeNode: TreeNode
}

export interface TreeNode {
    index: number
    parent: TreeNode | null
    childNodes: TreeNode[]
    path: string
    url: string
}

/**
 * Gets the next child in the file tree given a node and index.
 * index represents the number of children of node that we have already traversed.
 * If node does not have any children or any more children or to traverse, we call
 * nextChild recursively, passing in node's parent to get any siblings of the current node.
 */
const nextChild = (node: TreeNode, index: number): TreeNode => {
    const nextChildNode = node.childNodes[index]
    if (!nextChildNode) {
        if (node.parent === null) {
            return node.childNodes[0]
        }
        /** This case gets called whenever we are going _up_ the tree */
        return nextChild(node.parent, node.index + 1)
    }
    return nextChildNode
}

/**
 * Helper for prevChild, this gets the deepest available descendant of a given node.
 * For a given node, a sibling node can have an arbitrary number of expanded directories.
 * In order to get the previous item in the tree, we need the absolute last
 * available descendent of a the previous sibling node.
 */
const getDeepestDescendant = (node: TreeNode): TreeNode => {
    while (node && node.childNodes.length > 0) {
        node = node.childNodes[node.childNodes.length - 1]
    }
    return node
}

/**
 * Gets the previous child in the file tree given a node and index.
 * To get the previous child, we check node's parent's child nodes, and get the
 * child node at index - 1. If we are at index 0, return the parent.
 */
const previousChild = (node: TreeNode, index: number): TreeNode => {
    // Only occurs on initial load of Tree, when there is no selected or active node.
    if (!node.parent) {
        return node
    }

    const validChildNodes = node.parent.childNodes.slice(0, node.index)

    // If we are at the first child in a tree layer (index 0), return the parent node.
    // Check if the dom node exists so if we're at the top-most layer,
    // we don't return the top-level Tree component node, which doesn't exist in the DOM.
    if (validChildNodes.length === 0 && getDomElement(node.parent.path)) {
        return node.parent
    }

    const previous = validChildNodes[index - 1]
    if (previous) {
        if (previous.childNodes && previous.childNodes.length > 0) {
            return getDeepestDescendant(previous)
        }

        return previous
    }

    // At top of tree, circle back down.
    return getDeepestDescendant(node.parent)
}

export class Tree extends React.PureComponent<Props, State> {
    private componentUpdates = new Subject<Props>()
    // This fires whenever a directory is expanded or collapsed.
    private expandDirectoryChanges = new Subject<{ path: string; expanded: boolean; node: TreeNode }>()
    private subscriptions = new Subscription()

    public node: TreeNode
    private treeElement: HTMLElement | null

    private keyHandlers: Record<string, () => void> = {
        [Key.ArrowDown]: () => {
            // This case gets called whenever we are going _down_ the tree
            if (this.state.selectedNode) {
                this.selectNode(nextChild(this.state.selectedNode, 0))
            }
        },
        [Key.ArrowUp]: () => {
            if (this.state.selectedNode) {
                this.selectNode(previousChild(this.state.selectedNode, this.state.selectedNode.index))
            }
        },
        [Key.ArrowLeft]: () => {
            const selectedNodePath =
                this.state.selectedNode.path !== '' ? this.state.selectedNode.path : this.props.activePath
            const isOpenDirectory = this.isExpanded(selectedNodePath)
            if (isOpenDirectory) {
                this.expandDirectoryChanges.next({
                    path: selectedNodePath,
                    expanded: false,
                    node: this.state.selectedNode,
                })
                return
            }
            const parent = this.state.selectedNode.parent
            if (parent?.parent) {
                this.selectNode(parent)
                return
            }

            this.selectNode(previousChild(this.state.selectedNode, this.state.selectedNode.index))
        },
        [Key.ArrowRight]: () => {
            const selectedNodePath =
                this.state.selectedNode.path !== '' ? this.state.selectedNode.path : this.props.activePath
            const nodeDomElement = getDomElement(selectedNodePath)
            if (nodeDomElement) {
                const isDirectory = Boolean(nodeDomElement.getAttribute('data-tree-is-directory'))
                if (!this.isExpanded(selectedNodePath) && isDirectory) {
                    // First, show the group (but don't update selection)
                    this.expandDirectoryChanges.next({
                        path: selectedNodePath,
                        expanded: true,
                        node: this.state.selectedNode,
                    })
                } else {
                    this.selectNode(nextChild(this.state.selectedNode, 0))
                }
            }
        },
        [Key.Enter]: () => {
            const selectedNodePath = this.state.selectedNode.path
            const nodeDomElement = getDomElement(selectedNodePath)
            if (nodeDomElement) {
                const isDirectory = Boolean(nodeDomElement.getAttribute('data-tree-is-directory'))
                if (isDirectory) {
                    const isOpen = this.isExpanded(selectedNodePath)
                    if (isOpen) {
                        this.expandDirectoryChanges.next({
                            path: selectedNodePath,
                            expanded: false,
                            node: this.state.selectedNode,
                        })
                        this.selectNode(this.state.selectedNode)
                        return
                    }
                    this.expandDirectoryChanges.next({
                        path: selectedNodePath,
                        expanded: true,
                        node: this.state.selectedNode,
                    })
                }
                this.selectNode(this.state.selectedNode)
                this.setActiveNode(this.state.selectedNode)
                this.props.history.push(this.state.selectedNode.url)
            }
        },
    }

    constructor(props: Props) {
        super(props)

        const parentPath = dotPathAsUndefined(props.activePathIsDir ? props.activePath : dirname(props.activePath))
        this.node = {
            index: 0,
            parent: null,
            childNodes: [],
            path: '',
            url: '',
        }

        this.state = {
            parentPath,
            resolveTo: [],
            selectedNode: this.node,
            activeNode: this.node,
        }

        this.treeElement = null
    }

    public componentDidMount(): void {
        this.subscriptions.add(
            this.expandDirectoryChanges.subscribe(({ path, expanded, node }) => {
                this.setState(previousState => ({
                    resolveTo: expanded
                        ? [...previousState.resolveTo, path]
                        : previousState.resolveTo.filter(expandedPath => expandedPath !== path),
                }))
                if (!expanded) {
                    // For directory nodes that are collapsed, unset the childNodes so we don't traverse them.
                    if (this.treeElement) {
                        this.treeElement.focus()
                    }
                    node.childNodes = []
                }
            })
        )

        this.subscriptions.add(
            this.componentUpdates
                .pipe(startWith(this.props), distinctUntilChanged(isEqual))
                .subscribe((props: Props) => {
                    const newParentPath = props.activePathIsDir ? props.activePath : dirname(props.activePath)
                    const queryParameters = new URLSearchParams(this.props.history.location.search)
                    const queryParametersHasSubtree = queryParameters.get('subtree') === 'true'

                    // If we're updating due to a file/directory suggestion or code intel action,
                    // load the relevant partial tree and jump to the file.
                    // This case is only used when going from an ancestor to a child file/directory, or equal.
                    if (queryParametersHasSubtree && !queryParameters.has('tab') && dotPathAsUndefined(newParentPath)) {
                        this.setState({
                            parentPath: dotPathAsUndefined(newParentPath),
                            resolveTo: [newParentPath],
                        })
                    }

                    // Recompute with new paths and parent path. But if the new active path is below where we are now,
                    // preserve the current parent path, so that it's easy for the user to go back up. Also resets the selectedNode
                    // to the top-level Tree component and resets resolveTo so no directories are expanded.
                    if (!pathEqualToOrAncestor(this.state.parentPath || '', newParentPath)) {
                        this.setState({
                            parentPath: dotPathAsUndefined(
                                props.activePathIsDir ? props.activePath : dirname(props.activePath)
                            ),
                            selectedNode: this.node,
                            resolveTo: [],
                        })
                    }

                    // Strip the ?subtree query param. Handle both when going from ancestor -> child and child -> ancestor.
                    queryParameters.delete('subtree')
                    if (queryParametersHasSubtree && !queryParameters.has('tab')) {
                        this.props.history.replace({
                            search: queryParameters.toString(),
                            hash: this.props.history.location.hash,
                        })
                    }
                })
        )
    }

    public componentDidUpdate(): void {
        this.componentUpdates.next(this.props)
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <div tabIndex={0} onKeyDown={this.onKeyDown} ref={this.setTreeElement}>
                <TreeRoot
                    ref={reference => {
                        if (reference) {
                            this.node = reference.node
                        }
                    }}
                    activeNode={this.state.activeNode}
                    activePath={this.props.activePath}
                    depth={0}
                    location={this.props.location}
                    repoName={this.props.repoName}
                    revision={this.props.revision}
                    commitID={this.props.commitID}
                    index={0}
                    // The root is always expanded so it loads the top level
                    isExpanded={true}
                    // A node with parent null tells us we're at the root of the tree
                    parent={null}
                    parentPath={this.state.parentPath}
                    expandedTrees={this.state.resolveTo}
                    onSelect={this.selectNode}
                    onToggleExpand={this.toggleExpandDirectory}
                    selectedNode={this.state.selectedNode}
                    setChildNodes={this.setChildNode}
                    setActiveNode={this.setActiveNode}
                    sizeKey={this.props.sizeKey}
                    extensionsController={this.props.extensionsController}
                    isLightTheme={this.props.isLightTheme}
                />
            </div>
        )
    }

    private setChildNode = (node: TreeNode, index: number): void => {
        this.node.childNodes[index] = node
    }

    private isExpanded(path: string): boolean {
        return this.state.resolveTo.includes(path)
    }

    private selectNode = (node: TreeNode): void => {
        if (node) {
            const element = getDomElement(node.path)
            if (element) {
                element.scrollIntoView({ block: 'start', inline: 'start' })
            }
            this.setState({ selectedNode: node })
        }
    }

    /** Set active node sets the active node when a directory or file is selected. It also sets the selected node in this case. */
    private setActiveNode = (node: TreeNode): void => {
        this.setState({ activeNode: node })
        this.selectNode(node)
    }

    /** Called when a tree entry is expanded or collapsed. */
    private toggleExpandDirectory = (path: string, expanded: boolean, node: TreeNode): void => {
        this.expandDirectoryChanges.next({ path, expanded, node })
    }

    private onKeyDown = (event: React.KeyboardEvent<HTMLElement>): void => {
        const handler = this.keyHandlers[event.key]
        if (handler) {
            event.preventDefault()
            handler()
        }
    }

    private setTreeElement = (element: HTMLElement | null): void => {
        if (element) {
            this.treeElement = element
        }
    }
}

function dotPathAsUndefined(path: string | undefined): string | undefined {
    if (path === undefined || path === '.') {
        return undefined
    }
    return path
}

/** Returns whether path is an ancestor of (or equal to) candidate. */
function pathEqualToOrAncestor(path: string, candidate: string): boolean {
    return path === candidate || path === '' || candidate.startsWith(path + '/')
}
