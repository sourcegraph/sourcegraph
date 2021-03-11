import React from 'react'

export const EmptyChangesetListElement: React.FunctionComponent<{}> = () => (
    <div className="col-md-8 offset-md-2 col-sm-12 card mt-5">
        <div className="card-body p-5 empty-changeset-list-element__body">
            <h2 className="text-center mb-4">This batch change does not contain changesets</h2>
            <p>This can occur for several reasons:</p>
            <p>
                <strong>
                    The query specified in <span className="text-monospace">repositoriesMatchingQuery:</span> may not
                    have matched any repositories.
                </strong>
            </p>
            <p>Test your query in the search bar and ensure it returns results.</p>
            <p>
                <strong>
                    The code specified in <span className="text-monospace">steps:</span> may not have resulted in
                    changes being made.
                </strong>
            </p>
            <p>
                Try the command on a local instance of one of the repositories returned in your search results. Run{' '}
                <span className="text-monospace">git status</span> and ensure it produced changed files.
            </p>
        </div>
    </div>
)
