import classNames from 'classnames'
import React, { useCallback, useState } from 'react'
import { map, mapTo } from 'rxjs/operators'

import { asError, isErrorLike } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/shared/src/graphql/graphql'
import { RouterLink } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../backend/graphql'
import { ErrorAlert } from '../../components/alerts'
import { Timestamp } from '../../components/time/Timestamp'
import {
    AccessTokenFields,
    CreateAccessTokenResult,
    DeleteAccessTokenResult,
    DeleteAccessTokenVariables,
    Scalars,
} from '../../graphql-operations'
import { userURL } from '../../user'

import { AccessTokenCreatedAlert } from './AccessTokenCreatedAlert'
import styles from './AccessTokenNode.module.scss'

export const accessTokenFragment = gql`
    fragment AccessTokenFields on AccessToken {
        id
        scopes
        note
        createdAt
        lastUsedAt
        subject {
            username
        }
        creator {
            username
        }
    }
`

function deleteAccessToken(tokenID: Scalars['ID']): Promise<void> {
    return requestGraphQL<DeleteAccessTokenResult, DeleteAccessTokenVariables>(
        gql`
            mutation DeleteAccessToken($tokenID: ID!) {
                deleteAccessToken(byID: $tokenID) {
                    alwaysNil
                }
            }
        `,
        { tokenID }
    )
        .pipe(map(dataOrThrowErrors), mapTo(undefined))
        .toPromise()
}

export interface AccessTokenNodeProps {
    node: AccessTokenFields

    /**
     * The newly created token, if any.
     */
    newToken?: CreateAccessTokenResult['createAccessToken']

    /** Whether the token's subject user should be displayed. */
    showSubject: boolean

    afterDelete: () => void
}

export const AccessTokenNode: React.FunctionComponent<AccessTokenNodeProps> = ({
    node,
    showSubject,
    newToken,
    afterDelete,
}) => {
    const [isDeleting, setIsDeleting] = useState<boolean | Error>(false)
    const onDeleteAccessToken = useCallback(async () => {
        if (
            !window.confirm(
                'Delete and revoke this token? Any clients using it will no longer be able to access the Sourcegraph API.'
            )
        ) {
            return
        }
        setIsDeleting(true)
        try {
            await deleteAccessToken(node.id)
            setIsDeleting(false)
            if (afterDelete) {
                afterDelete()
            }
        } catch (error) {
            setIsDeleting(asError(error))
        }
    }, [node.id, afterDelete])

    const note = node.note || '(no description)'

    return (
        <li
            className={classNames(styles.accessTokenNodeContainer, 'list-group-item d-block')}
            data-test-access-token-description={note}
        >
            <div className="d-flex w-100 justify-content-between align-items-center">
                <div className="mr-2">
                    {showSubject ? (
                        <>
                            <strong>
                                <RouterLink to={userURL(node.subject.username)}>{node.subject.username}</RouterLink>
                            </strong>{' '}
                            &mdash; {note}
                        </>
                    ) : (
                        <strong>{note}</strong>
                    )}{' '}
                    <small className="text-muted">
                        {' '}
                        &mdash; <em>{node.scopes?.join(', ')}</em>
                        <br />
                        {node.lastUsedAt ? (
                            <>
                                Last used <Timestamp date={node.lastUsedAt} />
                            </>
                        ) : (
                            'Never used'
                        )}
                        , created <Timestamp date={node.createdAt} />
                        {node.subject.username !== node.creator.username && (
                            <>
                                {' '}
                                by <RouterLink to={userURL(node.creator.username)}>{node.creator.username}</RouterLink>
                            </>
                        )}
                    </small>
                </div>
                <div>
                    <button
                        type="button"
                        className="btn btn-danger test-access-token-delete"
                        onClick={onDeleteAccessToken}
                        disabled={isDeleting === true}
                    >
                        Delete
                    </button>
                    {isErrorLike(isDeleting) && <ErrorAlert className="mt-2" error={isDeleting} />}
                </div>
            </div>
            {newToken && node.id === newToken.id && (
                <AccessTokenCreatedAlert className="mt-4" tokenSecret={newToken.token} token={node} />
            )}
        </li>
    )
}
