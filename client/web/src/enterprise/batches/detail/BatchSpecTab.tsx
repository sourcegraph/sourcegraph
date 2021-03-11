import FileDownloadIcon from 'mdi-react/FileDownloadIcon'
import React, { useMemo } from 'react'
import { Link } from '../../../../../shared/src/components/Link'
import { CodeSnippet } from '../../../../../branded/src/components/CodeSnippet'
import { Timestamp } from '../../../components/time/Timestamp'
import { BatchChangeFields } from '../../../graphql-operations'

export interface BatchSpecTabProps {
    batchChange: Pick<BatchChangeFields, 'name' | 'createdAt' | 'lastApplier' | 'lastAppliedAt'>
    originalInput: BatchChangeFields['currentSpec']['originalInput']
}

/** Reports whether str is a valid JSON document. */
const isJSON = (string: string): boolean => {
    try {
        JSON.parse(string)
        return true
    } catch {
        return false
    }
}

export const BatchSpecTab: React.FunctionComponent<BatchSpecTabProps> = ({
    batchChange: { name: batchChangeName, createdAt, lastApplier, lastAppliedAt },
    originalInput,
}) => {
    const downloadUrl = useMemo(() => 'data:text/plain;charset=utf-8,' + encodeURIComponent(originalInput), [
        originalInput,
    ])

    // JSON is valid YAML, so the input might be JSON. In that case, we'll highlight and indent it
    // as JSON. This is especially nice when the input is a "minified" (no extraneous whitespace)
    // JSON document that's difficult to read unless indented.
    const inputIsJSON = isJSON(originalInput)
    const input = useMemo(() => (inputIsJSON ? JSON.stringify(JSON.parse(originalInput), null, 2) : originalInput), [
        inputIsJSON,
        originalInput,
    ])

    return (
        <>
            <div className="d-flex flex-wrap justify-content-between align-items-baseline mb-2 test-batches-spec">
                <p className="mb-2 batch-spec-tab__header-col">
                    {lastApplier ? <Link to={lastApplier.url}>{lastApplier.username}</Link> : 'A deleted user'}{' '}
                    {createdAt === lastAppliedAt ? 'created' : 'updated'} this batch change{' '}
                    <Timestamp date={lastAppliedAt} /> by applying the following batch spec:
                </p>
                <div className="batch-spec-tab__header-col">
                    <a
                        download={`${batchChangeName}.batch.yaml`}
                        href={downloadUrl}
                        className="text-right btn btn-secondary text-nowrap"
                        data-tooltip={`Download ${batchChangeName}.batch.yaml`}
                    >
                        <FileDownloadIcon className="icon-inline" /> Download YAML
                    </a>
                </div>
            </div>
            <CodeSnippet code={input} language={inputIsJSON ? 'json' : 'yaml'} className="mb-3" />
        </>
    )
}
