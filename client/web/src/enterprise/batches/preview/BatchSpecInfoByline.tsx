import React from 'react'
import { BatchSpecFields } from '../../../graphql-operations'
import { Link } from '../../../../../shared/src/components/Link'
import { Timestamp } from '../../../components/time/Timestamp'

interface Props extends Pick<BatchSpecFields, 'createdAt' | 'creator'> {}

/**
 * The uploaded at byline to the batch change apply page header.
 */
export const BatchSpecInfoByline: React.FunctionComponent<Props> = ({ createdAt, creator }) => (
    <>
        Uploaded <Timestamp date={createdAt} /> by {creator && <Link to={creator.url}>{creator.username}</Link>}
        {!creator && <strong>deleted user</strong>}
    </>
)
