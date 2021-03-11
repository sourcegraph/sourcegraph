import React from 'react'

export const CloseChangesetsListEmptyElement: React.FunctionComponent<{}> = () => (
    <div className="col-md-8 offset-md-2 col-sm-12 card mt-5">
        <div className="card-body close-changesets-list-empty-element__body p-5">
            <h2 className="text-center font-weight-normal">
                Closing this batch change will not alter changesets and no changesets will remain open.
            </h2>
        </div>
    </div>
)
