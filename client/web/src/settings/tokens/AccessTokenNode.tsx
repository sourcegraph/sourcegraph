import React, { useCallback, useState } from 'react'

import classNames from 'classnames'
import { parseISO } from 'date-fns'
import { lastValueFrom } from 'rxjs'
import { map } from 'rxjs/operators'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { asError, isErrorLike } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { Button, Link, ErrorAlert } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../backend/graphql'
import type {
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
        expiresAt
    }
`

function deleteAccessToken(tokenID: Scalars['ID']): Promise<void> {
    return lastValueFrom(
        requestGraphQL<DeleteAccessTokenResult, DeleteAccessTokenVariables>(
            gql`
                mutation DeleteAccessToken($tokenID: ID!) {
                    deleteAccessToken(byID: $tokenID) {
                        alwaysNil
                    }
                }
            `,
            { tokenID }
        ).pipe(
            map(dataOrThrowErrors),
            map(() => undefined)
        )
    )
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

export const AccessTokenNode: React.FunctionComponent<React.PropsWithChildren<AccessTokenNodeProps>> = ({
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
                                <Link to={userURL(node.subject.username)}>{node.subject.username}</Link>
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
                                by <Link to={userURL(node.creator.username)}>{node.creator.username}</Link>
                            </>
                        )}
                        {node.expiresAt !== null && (
                            <>
                                {' '}
                                | Expires <Timestamp date={parseISO(node.expiresAt)} />
                            </>
                        )}
                    </small>
                </div>
                <div>
                    <Button
                        className="test-access-token-delete"
                        onClick={onDeleteAccessToken}
                        disabled={isDeleting === true}
                        variant="danger"
                    >
                        Delete
                    </Button>
                    {isErrorLike(isDeleting) && <ErrorAlert className="mt-2" error={isDeleting} />}
                </div>
            </div>
            {newToken && node.id === newToken.id && (
                <AccessTokenCreatedAlert className="mt-4" tokenSecret={newToken.token} token={node} />
            )}
        </li>
    )
}
