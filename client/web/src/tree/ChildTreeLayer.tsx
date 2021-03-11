import React from 'react'
import { FileDecorationsByPath } from '../../../shared/src/api/extension/flatExtensionApi'
import { SingleChildTreeLayer } from './SingleChildTreeLayer'
import { TreeLayer } from './TreeLayer'
import { TreeRootProps } from './TreeRoot'
import { hasSingleChild, SingleChildGitTree, TreeEntryInfo } from './util'

interface ChildTreeLayerProps extends Pick<TreeRootProps, Exclude<keyof TreeRootProps, 'sizeKey'>> {
    fileDecorationsByPath: FileDecorationsByPath

    entries: TreeEntryInfo[]
    /** The entry information for a SingleChildTreeLayer. Will be a SingleChildGitTree with fields populated, or be an empty object if there is no SingleChildTreeLayer to render. */
    singleChildTreeEntry: SingleChildGitTree
    /** The children entries of a SingleChildTreeLayer. Will be undefined if there is no SingleChildTreeLayer to render. */
    childrenEntries?: SingleChildGitTree[]
    onHover: (filePath: string) => void
}

/**
 * Either a SingleChildTreeLayer or TreeLayer.
 */
export const ChildTreeLayer: React.FunctionComponent<ChildTreeLayerProps> = (props: ChildTreeLayerProps) => {
    const sharedProps = {
        location: props.location,
        activePath: props.activePath,
        activeNode: props.activeNode,
        depth: props.depth + 1,
        expandedTrees: props.expandedTrees,
        parent: props.parent,
        repoName: props.repoName,
        revision: props.revision,
        onToggleExpand: props.onToggleExpand,
        onHover: props.onHover,
        selectedNode: props.selectedNode,
        setChildNodes: props.setChildNodes,
        setActiveNode: props.setActiveNode,
        onSelect: props.onSelect,
        commitID: props.commitID,
        extensionsController: props.extensionsController,
        isLightTheme: props.isLightTheme,
    }

    return (
        <div>
            <table className="tree-layer">
                <tbody>
                    <tr>
                        <td>
                            {hasSingleChild(props.entries) ? (
                                <SingleChildTreeLayer
                                    {...sharedProps}
                                    key={props.singleChildTreeEntry.path}
                                    index={0}
                                    isExpanded={props.expandedTrees.includes(props.singleChildTreeEntry.path)}
                                    parentPath={props.singleChildTreeEntry.path}
                                    entryInfo={props.singleChildTreeEntry}
                                    childrenEntries={props.singleChildTreeEntry.children}
                                    fileDecorationsByPath={props.fileDecorationsByPath}
                                    fileDecorations={props.fileDecorationsByPath[props.singleChildTreeEntry.path]}
                                />
                            ) : (
                                props.entries.map((item, index) => (
                                    <TreeLayer
                                        {...sharedProps}
                                        key={item.path}
                                        index={index}
                                        isExpanded={props.expandedTrees.includes(item.path)}
                                        parentPath={item.path}
                                        entryInfo={item}
                                        fileDecorations={props.fileDecorationsByPath[item.path]}
                                    />
                                ))
                            )}
                        </td>
                    </tr>
                </tbody>
            </table>
        </div>
    )
}
