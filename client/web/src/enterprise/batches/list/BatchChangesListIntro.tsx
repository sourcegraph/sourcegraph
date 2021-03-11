import React from 'react'
import { DismissibleAlert } from '../../../components/DismissibleAlert'
import { SourcegraphIcon } from '../../../auth/icons'

export interface BatchChangesListIntroProps {
    licensed: boolean | undefined
}

export const BatchChangesListIntro: React.FunctionComponent<BatchChangesListIntroProps> = ({ licensed }) => (
    <div className="row mb-2">
        {licensed === true ? (
            <div className="col-12">
                <BatchChangesChangelogAlert />
            </div>
        ) : (
            <>
                {licensed === false && (
                    <>
                        <div className="col-12 col-md-6">
                            <BatchChangesUnlicensedAlert />
                        </div>
                        <div className="col-12 col-md-6">
                            <BatchChangesChangelogAlert />
                        </div>
                    </>
                )}
            </>
        )}
    </div>
)

const BatchChangesChangelogAlert: React.FunctionComponent = () => (
    <DismissibleAlert
        className="batch-changes-list-intro__alert"
        partialStorageKey="batch-changes-list-intro-changelog-3.26"
    >
        <div className="batch-changes-list-intro__card card h-100 p-2">
            <div className="card-body">
                <h4>New batch changes features in version 3.26</h4>
                <ul className="text-muted mb-0 pl-3">
                    <li>Campaigns have been renamed to Batch Changes!</li>
                </ul>
            </div>
        </div>
    </DismissibleAlert>
)

const BatchChangesUnlicensedAlert: React.FunctionComponent = () => (
    <div className="batch-change-list-intro__alert">
        <div className="batch-change-list-intro__card card p-2 h-100">
            <div className="card-body d-flex align-items-start">
                {/* d-none d-sm-block ensure that we hide the icon on XS displays. */}
                <SourcegraphIcon className="mr-3 col-2 mt-2 d-none d-sm-block" />
                <div>
                    <h4>Batch changes trial</h4>
                    <p className="text-muted">
                        Batch changes is a paid feature of Sourcegraph. All users can create sample batch changes with
                        up to five changesets without a license.
                    </p>
                    <p className="text-muted mb-0">
                        <a href="https://about.sourcegraph.com/contact/sales/">Contact sales</a> to obtain a trial
                        license.
                    </p>
                </div>
            </div>
        </div>
    </div>
)
