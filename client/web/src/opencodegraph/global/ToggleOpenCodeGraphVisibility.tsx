import { useCallback } from 'react'

import { mdiWeb, mdiWebOff } from '@mdi/js'
import { RepoActionInfo } from 'src/repo/RepoActionInfo'

import { SimpleActionItem } from '@sourcegraph/shared/src/actions/SimpleActionItem'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { RenderMode } from '@sourcegraph/shared/src/util/url'
import { Button, Icon, Tooltip } from '@sourcegraph/wildcard'

import { RepoHeaderActionAnchor, RepoHeaderActionMenuLink } from '../../repo/components/RepoHeaderActions'

import { useOpenCodeGraphVisibility } from './useOpenCodeGraphVisibility'

import styles from './ToggleOpenCodeGraphVisibility.module.scss'

interface Props extends TelemetryV2Props {
    source?: 'repoHeader' | 'actionItemsBar'
    actionType?: 'nav' | 'dropdown'
    renderMode?: RenderMode
}

export const ToggleOpenCodeGraphVisibilityAction: React.FC<Props> = props => {
    const [visible, setVisible] = useOpenCodeGraphVisibility()

    const disabled = props.renderMode === 'rendered'
    const descriptiveText = disabled
        ? 'OpenCodeGraph metadata is not available in rendered files'
        : `${visible ? 'Hide' : 'Show'} OpenCodeGraph metadata`

    const onCycle = useCallback(() => {
        props.telemetryRecorder.recordEvent('blob.openCodeGraph.metadata', visible ? 'hide' : 'show')
        setVisible(prevVisible => !prevVisible)
    }, [setVisible, visible, props.telemetryRecorder])

    const iconPath = disabled || !visible ? mdiWebOff : mdiWeb
    const icon = <Icon aria-hidden={true} svgPath={iconPath} className={styles.repoActionIcon} />

    if (props.source === 'actionItemsBar') {
        return (
            <SimpleActionItem tooltip={descriptiveText} isActive={visible} onSelect={onCycle} disabled={disabled}>
                {icon}
            </SimpleActionItem>
        )
    }

    if (props.actionType === 'dropdown') {
        return (
            <RepoHeaderActionMenuLink file={true} as={Button} onClick={onCycle} disabled={disabled}>
                {icon}
                <span>{descriptiveText}</span>
            </RepoHeaderActionMenuLink>
        )
    }

    return (
        <Tooltip content={descriptiveText}>
            <RepoHeaderActionAnchor
                onSelect={onCycle}
                disabled={disabled}
                className="d-flex justify-content-center align-items-center"
            >
                <RepoActionInfo icon={icon} displayName="Graph" />
            </RepoHeaderActionAnchor>
        </Tooltip>
    )
}
