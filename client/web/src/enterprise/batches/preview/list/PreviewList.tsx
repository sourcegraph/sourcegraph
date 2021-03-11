import React, { useCallback, useState } from 'react'
import * as H from 'history'
import { ThemeProps } from '../../../../../../shared/src/theme'
import { FilteredConnection, FilteredConnectionQueryArguments } from '../../../../components/FilteredConnection'
import { ChangesetApplyPreviewFields, Scalars } from '../../../../graphql-operations'
import { queryChangesetApplyPreview as _queryChangesetApplyPreview, queryChangesetSpecFileDiffs } from './backend'
import { ChangesetApplyPreviewNode, ChangesetApplyPreviewNodeProps } from './ChangesetApplyPreviewNode'
import { PreviewListHeader } from './PreviewListHeader'
import { EmptyPreviewListElement } from './EmptyPreviewListElement'
import { PreviewFilterRow, PreviewFilters } from './PreviewFilterRow'
import MagnifyIcon from 'mdi-react/MagnifyIcon'
import { PreviewPageAuthenticatedUser } from '../BatchChangePreviewPage'

interface Props extends ThemeProps {
    batchSpecID: Scalars['ID']
    history: H.History
    location: H.Location
    authenticatedUser: PreviewPageAuthenticatedUser

    /** For testing only. */
    queryChangesetApplyPreview?: typeof _queryChangesetApplyPreview
    /** For testing only. */
    queryChangesetSpecFileDiffs?: typeof queryChangesetSpecFileDiffs
    /** Expand changeset descriptions, for testing only. */
    expandChangesetDescriptions?: boolean
}

/**
 * A list of a batch spec's preview nodes.
 */
export const PreviewList: React.FunctionComponent<Props> = ({
    batchSpecID,
    history,
    location,
    authenticatedUser,
    isLightTheme,

    queryChangesetApplyPreview = _queryChangesetApplyPreview,
    queryChangesetSpecFileDiffs,
    expandChangesetDescriptions,
}) => {
    const [filters, setFilters] = useState<PreviewFilters>({
        search: null,
        currentState: null,
        action: null,
    })

    const queryChangesetApplyPreviewConnection = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryChangesetApplyPreview({
                first: args.first ?? null,
                after: args.after ?? null,
                batchSpec: batchSpecID,
                search: filters.search,
                currentState: filters.currentState,
                action: filters.action,
            }),
        [batchSpecID, filters.search, filters.currentState, filters.action, queryChangesetApplyPreview]
    )

    return (
        <>
            <h3>Preview</h3>
            <hr className="mb-3" />
            <PreviewFilterRow history={history} location={location} onFiltersChange={setFilters} />
            <FilteredConnection<ChangesetApplyPreviewFields, Omit<ChangesetApplyPreviewNodeProps, 'node'>>
                className="mt-2"
                nodeComponent={ChangesetApplyPreviewNode}
                nodeComponentProps={{
                    isLightTheme,
                    history,
                    location,
                    authenticatedUser,
                    queryChangesetSpecFileDiffs,
                    expandChangesetDescriptions,
                }}
                queryConnection={queryChangesetApplyPreviewConnection}
                hideSearch={true}
                defaultFirst={15}
                noun="changeset"
                pluralNoun="changesets"
                history={history}
                location={location}
                useURLQuery={true}
                listComponent="div"
                listClassName="preview-list__grid mb-3"
                headComponent={PreviewListHeader}
                cursorPaging={true}
                noSummaryIfAllNodesVisible={true}
                emptyElement={
                    filters.search || filters.currentState || filters.action ? (
                        <EmptyPreviewSearchElement />
                    ) : (
                        <EmptyPreviewListElement />
                    )
                }
            />
        </>
    )
}

const EmptyPreviewSearchElement: React.FunctionComponent<{}> = () => (
    <div className="text-muted mt-4 pt-4 mb-4 row">
        <div className="col-12 text-center">
            <MagnifyIcon className="icon" />
            <div className="pt-2">No changesets matched the search.</div>
        </div>
    </div>
)
