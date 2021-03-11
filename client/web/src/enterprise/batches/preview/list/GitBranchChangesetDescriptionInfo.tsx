import React from 'react'
import { VisibleChangesetApplyPreviewFields } from '../../../../graphql-operations'
import { formatPersonName, PersonLink } from '../../../../person/PersonLink'
import { UserAvatar } from '../../../../user/UserAvatar'

interface Props {
    node: VisibleChangesetApplyPreviewFields
}

export const GitBranchChangesetDescriptionInfo: React.FunctionComponent<Props> = ({ node }) => {
    if (node.targets.__typename === 'VisibleApplyPreviewTargetsDetach') {
        return <></>
    }

    if (node.targets.changesetSpec.description.__typename === 'ExistingChangesetReference') {
        return <></>
    }

    if (node.targets.changesetSpec.description.commits.length !== 1) {
        return <></>
    }

    const commit = node.targets.changesetSpec.description.commits[0]

    const previousCommit =
        node.targets.__typename === 'VisibleApplyPreviewTargetsUpdate' &&
        node.targets.changeset.currentSpec?.description.__typename === 'GitBranchChangesetDescription' &&
        node.targets.changeset.currentSpec.description.commits?.[0]

    return (
        <div className="git-branch-changeset-description-info__grid">
            {(node.delta.authorEmailChanged || node.delta.authorNameChanged || node.delta.commitMessageChanged) &&
                previousCommit && (
                    <>
                        <DeletedEntry
                            deleted={node.delta.authorEmailChanged || node.delta.authorNameChanged}
                            className="text-muted"
                        >
                            <div className="d-flex flex-column align-items-center mr-3">
                                <UserAvatar
                                    className="icon-inline mb-1"
                                    user={previousCommit.author}
                                    data-tooltip={formatPersonName(previousCommit.author)}
                                />{' '}
                                <PersonLink person={previousCommit.author} className="font-weight-bold text-nowrap" />
                            </div>
                        </DeletedEntry>
                        <div className="text-muted">
                            <DeletedEntry deleted={node.delta.commitMessageChanged}>
                                <h3 className="text-muted">{previousCommit.subject}</h3>
                            </DeletedEntry>
                            {previousCommit.body && (
                                <DeletedEntry deleted={node.delta.commitMessageChanged}>
                                    <pre className="text-wrap">{previousCommit.body}</pre>
                                </DeletedEntry>
                            )}
                        </div>
                    </>
                )}
            <div className="d-flex flex-column align-items-center mr-3">
                <UserAvatar
                    className="icon-inline mb-1"
                    user={commit.author}
                    data-tooltip={formatPersonName(commit.author)}
                />{' '}
                <PersonLink person={commit.author} className="font-weight-bold text-nowrap" />
            </div>
            <div>
                <h3>{commit.subject}</h3>
                {commit.body && <pre className="text-wrap">{commit.body}</pre>}
            </div>
        </div>
    )
}

const DeletedEntry: React.FunctionComponent<{ children: React.ReactNode; deleted: boolean; className?: string }> = ({
    children,
    deleted,
    className,
}) => {
    if (deleted) {
        return <del className={className}>{children}</del>
    }
    return <div className={className}>{children}</div>
}
