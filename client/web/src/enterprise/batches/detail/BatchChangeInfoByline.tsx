import React from 'react'
import { BatchChangeFields } from '../../../graphql-operations'
import { Link } from '../../../../../shared/src/components/Link'
import { Timestamp } from '../../../components/time/Timestamp'

interface Props extends Pick<BatchChangeFields, 'createdAt' | 'initialApplier' | 'lastAppliedAt' | 'lastApplier'> {}

/**
 * The created/updated byline in the batch change header.
 */
export const BatchChangeInfoByline: React.FunctionComponent<Props> = ({
    createdAt,
    initialApplier,
    lastAppliedAt,
    lastApplier,
}) => (
    <>
        Created <Timestamp date={createdAt} /> by{' '}
        {initialApplier ? <Link to={initialApplier.url}>{initialApplier.username}</Link> : 'a deleted user'}
        {lastAppliedAt !== createdAt && (
            <>
                <span className="mx-2">|</span>
                Updated <Timestamp date={lastAppliedAt} />
                {lastApplier?.username !== initialApplier?.username && (
                    <> by {lastApplier ? <Link to={lastApplier.url}>{lastApplier.username}</Link> : 'a deleted user'}</>
                )}
            </>
        )}
    </>
)
