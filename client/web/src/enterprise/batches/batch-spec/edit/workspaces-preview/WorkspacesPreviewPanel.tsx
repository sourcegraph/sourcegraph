import React from 'react'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Panel, useWindowSize, VIEWPORT_LG } from '@sourcegraph/wildcard'

import { WorkspacesPreview } from './WorkspacesPreview'

import styles from './WorkspacesPreviewPanel.module.scss'

const WORKSPACES_PREVIEW_SIZE = 'batch-changes.ssbc-workspaces-preview-size'

export const WorkspacesPreviewPanel: React.FunctionComponent<
    React.PropsWithChildren<{ isReadOnly?: boolean } & TelemetryV2Props>
> = ({ isReadOnly, telemetryRecorder }) => {
    const { width } = useWindowSize()

    // On sufficiently small screens, we break out of the 3-column layout and wrap the
    // workspaces preview panel to its own row. In its own row, we no longer need the
    // panel to be resizable.
    return width < VIEWPORT_LG ? (
        <WorkspacesPreview isReadOnly={isReadOnly} telemetryRecorder={telemetryRecorder} />
    ) : (
        <Panel
            className="d-flex"
            defaultSize={500}
            minSize={405}
            maxSize={0.45 * width}
            position="right"
            storageKey={WORKSPACES_PREVIEW_SIZE}
            ariaLabel="workspaces preview"
        >
            <div className={styles.container}>
                <WorkspacesPreview isReadOnly={isReadOnly} telemetryRecorder={telemetryRecorder} />
            </div>
        </Panel>
    )
}
