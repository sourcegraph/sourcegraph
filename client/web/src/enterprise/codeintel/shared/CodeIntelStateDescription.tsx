import { upperFirst } from 'lodash'
import React, { FunctionComponent } from 'react'
import { ErrorMessage } from '../../../components/alerts'
import { LSIFIndexState, LSIFUploadState } from '../../../graphql-operations'

export interface CodeIntelStateDescriptionProps {
    typeName: string
    pluralTypeName: string
    state: LSIFUploadState | LSIFIndexState
    placeInQueue?: number | null
    failure?: string | null
    className?: string
}

export const CodeIntelStateDescription: FunctionComponent<CodeIntelStateDescriptionProps> = ({
    typeName,
    pluralTypeName,
    state,
    placeInQueue,
    failure,
    className,
}) =>
    state === LSIFUploadState.UPLOADING ? (
        <span className={className}>Still uploading...</span>
    ) : state === LSIFUploadState.QUEUED || state === LSIFIndexState.QUEUED ? (
        <span className={className}>
            {upperFirst(typeName)} is queued.{' '}
            <CodeIntelStateDescriptionPlaceInQueue placeInQueue={placeInQueue} pluralTypeName={pluralTypeName} />
        </span>
    ) : state === LSIFUploadState.PROCESSING || state === LSIFIndexState.PROCESSING ? (
        <span className={className}>{upperFirst(typeName)} is currently being processed...</span>
    ) : state === LSIFUploadState.COMPLETED || state === LSIFIndexState.COMPLETED ? (
        <span className={className}>{upperFirst(typeName)} processed successfully.</span>
    ) : state === LSIFUploadState.ERRORED || state === LSIFIndexState.ERRORED ? (
        <span className={className}>
            {upperFirst(typeName)} failed to complete: <ErrorMessage error={failure} />
        </span>
    ) : (
        <></>
    )

export interface CodeIntelStateDescriptionPlaceInQueueProps {
    placeInQueue?: number | null
    pluralTypeName: string
}

const CodeIntelStateDescriptionPlaceInQueue: FunctionComponent<CodeIntelStateDescriptionPlaceInQueueProps> = ({
    placeInQueue,
    pluralTypeName,
}) => <>{placeInQueue ? `There are ${placeInQueue} ${pluralTypeName} ahead of this one.` : ''}</>
