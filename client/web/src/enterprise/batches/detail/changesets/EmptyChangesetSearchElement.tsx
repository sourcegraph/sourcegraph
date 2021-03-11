import React from 'react'
import MagnifyIcon from 'mdi-react/MagnifyIcon'

export const EmptyChangesetSearchElement: React.FunctionComponent<{}> = () => (
    <div className="text-muted mt-4 pt-4 mb-4 row">
        <div className="col-12 text-center">
            <MagnifyIcon className="icon" />
            <div className="pt-2">No changesets matched the search and/or filters selected.</div>
        </div>
    </div>
)
