import * as H from 'history'
import React from 'react'
import { ThemeProps } from '../../../../../../shared/src/theme'
import { ExtensionsControllerProps } from '../../../../../../shared/src/extensions/controller'
import { Hoverifier } from '@sourcegraph/codeintellify'
import { RepoSpec, RevisionSpec, FileSpec, ResolvedRevisionSpec } from '../../../../../../shared/src/util/url'
import { HoverMerged } from '../../../../../../shared/src/api/client/types/hover'
import { ActionItemAction } from '../../../../../../shared/src/actions/ActionItem'
import { ExternalChangesetNode } from './ExternalChangesetNode'
import { HiddenExternalChangesetNode } from './HiddenExternalChangesetNode'
import { ChangesetFields } from '../../../../graphql-operations'
import { queryExternalChangesetWithFileDiffs } from '../backend'

export interface ChangesetNodeProps extends ThemeProps {
    node: ChangesetFields
    viewerCanAdminister: boolean
    history: H.History
    location: H.Location
    extensionInfo?: {
        hoverifier: Hoverifier<RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec, HoverMerged, ActionItemAction>
    } & ExtensionsControllerProps
    /** For testing purposes. */
    queryExternalChangesetWithFileDiffs?: typeof queryExternalChangesetWithFileDiffs
    /** For testing purposes. */
    expandByDefault?: boolean
}

export const ChangesetNode: React.FunctionComponent<ChangesetNodeProps> = ({ node, ...props }) => {
    if (node.__typename === 'ExternalChangeset') {
        return (
            <>
                <span className="changeset-node__separator" />
                <ExternalChangesetNode node={node} {...props} />
            </>
        )
    }
    return (
        <>
            <span className="changeset-node__separator" />
            <HiddenExternalChangesetNode node={node} {...props} />
        </>
    )
}
