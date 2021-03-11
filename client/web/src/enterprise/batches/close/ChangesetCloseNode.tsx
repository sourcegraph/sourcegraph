import * as H from 'history'
import React from 'react'
import { Hoverifier } from '@sourcegraph/codeintellify'
import { ThemeProps } from '../../../../../shared/src/theme'
import { ChangesetFields } from '../../../graphql-operations'
import { RepoSpec, RevisionSpec, FileSpec, ResolvedRevisionSpec } from '../../../../../shared/src/util/url'
import { HoverMerged } from '../../../../../shared/src/api/client/types/hover'
import { ActionItemAction } from '../../../../../shared/src/actions/ActionItem'
import { ExtensionsControllerProps } from '../../../../../shared/src/extensions/controller'
import { queryExternalChangesetWithFileDiffs } from '../detail/backend'
import { ExternalChangesetCloseNode } from './ExternalChangesetCloseNode'
import { HiddenExternalChangesetCloseNode } from './HiddenExternalChangesetCloseNode'

export interface ChangesetCloseNodeProps extends ThemeProps {
    node: ChangesetFields
    viewerCanAdminister: boolean
    history: H.History
    location: H.Location
    extensionInfo?: {
        hoverifier: Hoverifier<RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec, HoverMerged, ActionItemAction>
    } & ExtensionsControllerProps
    queryExternalChangesetWithFileDiffs?: typeof queryExternalChangesetWithFileDiffs
    willClose: boolean
}

export const ChangesetCloseNode: React.FunctionComponent<ChangesetCloseNodeProps> = ({ node, ...props }) => {
    if (node.__typename === 'ExternalChangeset') {
        return (
            <>
                <span className="changeset-close-node__separator" />
                <ExternalChangesetCloseNode node={node} {...props} />
            </>
        )
    }
    return (
        <>
            <span className="changeset-close-node__separator" />
            <HiddenExternalChangesetCloseNode node={node} {...props} />
        </>
    )
}
