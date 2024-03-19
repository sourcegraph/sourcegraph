import React, { useMemo } from 'react'

import { mdiFileDownload } from '@mdi/js'
import { kebabCase } from 'lodash'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Link, Icon, Text, Tooltip, Button, AnchorLink } from '@sourcegraph/wildcard'

import type { BatchChangeFields } from '../../graphql-operations'
import { eventLogger } from '../../tracking/eventLogger'

import { MonacoBatchSpecEditor } from './batch-spec/edit/editor/MonacoBatchSpecEditor'

/** Reports whether `string` is a valid JSON document. */
const isJSON = (string: string): boolean => {
    try {
        JSON.parse(string)
        return true
    } catch {
        return false
    }
}

export const getFileName = (name: string): string => `${kebabCase(name)}.batch.yaml`

export interface BatchSpecProps extends TelemetryV2Props {
    name: string
    originalInput: BatchChangeFields['currentSpec']['originalInput']
    className?: string
}

export const BatchSpec: React.FunctionComponent<BatchSpecProps> = ({ originalInput, className, name }) => {
    const isLightTheme = useIsLightTheme()
    // JSON is valid YAML, so the input might be JSON. In that case, we'll highlight and indent it
    // as JSON. This is especially nice when the input is a "minified" (no extraneous whitespace)
    // JSON document that's difficult to read unless indented.
    const inputIsJSON = isJSON(originalInput)
    const input = useMemo(
        () => (inputIsJSON ? JSON.stringify(JSON.parse(originalInput), null, 2) : originalInput),
        [inputIsJSON, originalInput]
    )

    return (
        <MonacoBatchSpecEditor
            batchChangeName={name}
            isLightTheme={isLightTheme}
            value={input}
            readOnly={true}
            className={className}
        />
    )
}

interface BatchSpecDownloadLinkProps extends Omit<BatchSpecProps, 'isLightTheme'>, Pick<BatchChangeFields, 'name'> {
    className?: string
    asButton: boolean
}

export const BatchSpecDownloadLink: React.FunctionComponent<React.PropsWithChildren<BatchSpecDownloadLinkProps>> =
    React.memo(function BatchSpecDownloadLink({
        children,
        className,
        name,
        originalInput,
        asButton,
        telemetryRecorder,
    }) {
        const onClick: () => void = () => {
            eventLogger.log('batch_change_editor:download_for_src_cli:clicked')
            telemetryRecorder.recordEvent('batchChange.editor.downloadForCLI', 'click')
        }
        const component = asButton ? (
            <Button
                variant="primary"
                as={AnchorLink}
                download={getFileName(name)}
                to={'data:text/plain;charset=utf-8,' + encodeURIComponent(originalInput)}
                target="_blank"
                rel="noopener noreferrer"
                className={className}
                onClick={onClick}
            >
                {children}
            </Button>
        ) : (
            <Link
                download={getFileName(name)}
                to={'data:text/plain;charset=utf-8,' + encodeURIComponent(originalInput)}
                className={className}
                onClick={onClick}
            >
                {children}
            </Link>
        )

        return <Tooltip content={`Download ${getFileName(name)}`}>{component}</Tooltip>
    })

// TODO: Consider merging this component with BatchSpecDownloadLink
export const BatchSpecDownloadButton: React.FunctionComponent<
    Omit<BatchSpecProps, 'isLightTheme'> & Pick<BatchChangeFields, 'name'>
> = React.memo(function BatchSpecDownloadButton(props) {
    return (
        <BatchSpecDownloadLink className="text-right text-nowrap" {...props} asButton={false}>
            <Icon aria-hidden={true} svgPath={mdiFileDownload} /> Download YAML
        </BatchSpecDownloadLink>
    )
})

type BatchSpecMetaProps = Pick<BatchChangeFields, 'createdAt' | 'lastApplier' | 'lastAppliedAt'>

export const BatchSpecMeta: React.FunctionComponent<React.PropsWithChildren<BatchSpecMetaProps>> = ({
    createdAt,
    lastApplier,
    lastAppliedAt,
}) => (
    <Text className="mb-2">
        {lastApplier ? <Link to={lastApplier.url}>{lastApplier.username}</Link> : 'A deleted user'}{' '}
        {createdAt === lastAppliedAt ? 'created' : 'updated'} this batch change{' '}
        <Timestamp date={lastAppliedAt ?? createdAt} /> by applying the following batch spec:
    </Text>
)
