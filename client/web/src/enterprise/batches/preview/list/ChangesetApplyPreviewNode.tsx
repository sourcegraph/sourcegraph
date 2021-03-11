import * as H from 'history'
import React from 'react'
import { ChangesetApplyPreviewFields } from '../../../../graphql-operations'
import { HiddenChangesetApplyPreviewNode } from './HiddenChangesetApplyPreviewNode'
import { VisibleChangesetApplyPreviewNode } from './VisibleChangesetApplyPreviewNode'
import { ThemeProps } from '../../../../../../shared/src/theme'
import { queryChangesetSpecFileDiffs } from './backend'
import { PreviewPageAuthenticatedUser } from '../BatchChangePreviewPage'

export interface ChangesetApplyPreviewNodeProps extends ThemeProps {
    node: ChangesetApplyPreviewFields
    history: H.History
    location: H.Location
    authenticatedUser: PreviewPageAuthenticatedUser

    /** Used for testing. */
    queryChangesetSpecFileDiffs?: typeof queryChangesetSpecFileDiffs
    /** Expand changeset descriptions, for testing only. */
    expandChangesetDescriptions?: boolean
}

export const ChangesetApplyPreviewNode: React.FunctionComponent<ChangesetApplyPreviewNodeProps> = ({
    node,
    history,
    location,
    authenticatedUser,
    isLightTheme,
    queryChangesetSpecFileDiffs,
    expandChangesetDescriptions,
}) => {
    if (node.__typename === 'HiddenChangesetApplyPreview') {
        return (
            <>
                <span className="changeset-apply-preview-node__separator" />
                <HiddenChangesetApplyPreviewNode node={node} />
            </>
        )
    }
    return (
        <>
            <span className="changeset-apply-preview-node__separator" />
            <VisibleChangesetApplyPreviewNode
                node={node}
                history={history}
                location={location}
                isLightTheme={isLightTheme}
                authenticatedUser={authenticatedUser}
                queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
                expandChangesetDescriptions={expandChangesetDescriptions}
            />
        </>
    )
}
