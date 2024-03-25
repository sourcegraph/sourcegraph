import { type FC, type PropsWithChildren, useState } from 'react'

import { mdiDotsVertical } from '@mdi/js'
import classNames from 'classnames'
import { noop } from 'lodash'

import { useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Link,
    Menu,
    MenuButton,
    MenuDivider,
    MenuItem,
    MenuLink,
    MenuList,
    Position,
    Checkbox,
    Tooltip,
    Icon,
    MenuHeader,
} from '@sourcegraph/wildcard'

import {
    type Insight,
    type InsightDashboard,
    InsightType,
    isLangStatsInsight,
    isVirtualDashboard,
} from '../../../../core'
import { useUiFeatures } from '../../../../hooks'
import { encodeDashboardIdQueryParam } from '../../../../routers.constant'
import { ConfirmDeleteModal } from '../../../modals/ConfirmDeleteModal'
import { ExportInsightDataModal } from '../../../modals/ExportInsightDataModal'
import { ShareLinkModal } from '../../../modals/ShareLinkModal/ShareLinkModal'

import { ConfirmRemoveModal } from './ConfirmRemoveModal'

import styles from './InsightContextMenu.module.scss'

export interface InsightCardMenuProps extends TelemetryV2Props {
    insight: Insight
    currentDashboard: InsightDashboard | null
    zeroYAxisMin: boolean
    onToggleZeroYAxisMin?: () => void
}

/**
 * Renders context menu (three dots menu) for particular insight card.
 */
export const InsightContextMenu: FC<InsightCardMenuProps> = props => {
    const { insight, currentDashboard, zeroYAxisMin, onToggleZeroYAxisMin = noop, telemetryRecorder } = props

    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
    const [showRemoveConfirm, setShowRemoveConfirm] = useState(false)
    const [showExportDataConfirm, setShowExportDataConfirm] = useState(false)
    const [showShareModal, setShowShareModal] = useState(false)

    const { insight: insightPermissions } = useUiFeatures()
    const goCodeCheckerTemplates = useExperimentalFeatures(features => features.goCodeCheckerTemplates)

    const menuPermissions = insightPermissions.getContextActionsPermissions(insight)
    const showQuickFix = insight.title.includes('[quickfix]') && goCodeCheckerTemplates

    const quickFixUrl =
        insight.type === InsightType.SearchBased
            ? `/batch-changes/create?kind=goChecker${insight.series[0]?.name}&title=${insight.title}`
            : undefined

    const withinVirtualDashboard = !!currentDashboard && isVirtualDashboard(currentDashboard)

    return (
        <>
            <Menu>
                {({ isOpen }) => (
                    <>
                        <MenuButton
                            aria-label="Insight options"
                            outline={true}
                            variant="icon"
                            data-testid="InsightContextMenuButton"
                            className={classNames('p-1 d-inline-flex', styles.button)}
                        >
                            <Icon
                                className={classNames(styles.buttonIcon, { [styles.buttonIconActive]: isOpen })}
                                svgPath={mdiDotsVertical}
                                inline={false}
                                aria-hidden={true}
                                height={16}
                                width={16}
                            />
                        </MenuButton>
                        <MenuList
                            position={Position.bottomStart}
                            data-testid={`context-menu.${insight.id}`}
                            onKeyDown={event => event.stopPropagation()}
                        >
                            <MenuSection name="Insight">
                                <MenuLink
                                    as={Link}
                                    data-testid="InsightContextMenuEditLink"
                                    className={styles.item}
                                    to={encodeDashboardIdQueryParam(
                                        `/insights/${insight.id}/edit`,
                                        currentDashboard?.id
                                    )}
                                >
                                    Edit
                                </MenuLink>

                                <MenuLink
                                    data-testid="InsightContextMenuShareLink"
                                    className={styles.item}
                                    onSelect={() => setShowShareModal(true)}
                                >
                                    Get shareable link
                                </MenuLink>

                                {!isLangStatsInsight(insight) && (
                                    <MenuItem className={styles.item} onSelect={() => setShowExportDataConfirm(true)}>
                                        Export data
                                    </MenuItem>
                                )}
                            </MenuSection>

                            <MenuSection name="Chart settings">
                                {menuPermissions.showYAxis && (
                                    <MenuItem
                                        role="menuitemcheckbox"
                                        aria-checked={zeroYAxisMin}
                                        data-testid="InsightContextMenuEditLink"
                                        className={styles.item}
                                        onSelect={onToggleZeroYAxisMin}
                                    >
                                        <Checkbox
                                            id="InsightContextMenuEditInput"
                                            aria-hidden="true"
                                            checked={zeroYAxisMin}
                                            tabIndex={-1}
                                            label={<span className="font-weight-normal">Start Y Axis at 0</span>}
                                        />
                                    </MenuItem>
                                )}
                            </MenuSection>

                            <MenuSection name="Others" divider={false}>
                                {quickFixUrl && showQuickFix && (
                                    <MenuLink as={Link} className={styles.item} to={quickFixUrl}>
                                        Golang quick fixes
                                    </MenuLink>
                                )}

                                {currentDashboard && (
                                    <Tooltip
                                        content={
                                            withinVirtualDashboard
                                                ? "Removing insight isn't available for the All insights dashboard"
                                                : undefined
                                        }
                                        placement="left"
                                    >
                                        <MenuItem
                                            data-testid="insight-context-remove-from-dashboard-button"
                                            onSelect={() => setShowRemoveConfirm(true)}
                                            disabled={withinVirtualDashboard}
                                            className={styles.item}
                                        >
                                            Remove from this dashboard
                                        </MenuItem>
                                    </Tooltip>
                                )}

                                <MenuItem
                                    data-testid="insight-context-menu-delete-button"
                                    onSelect={() => setShowDeleteConfirm(true)}
                                    className={styles.item}
                                >
                                    Delete
                                </MenuItem>
                            </MenuSection>
                        </MenuList>
                    </>
                )}
            </Menu>

            <ConfirmDeleteModal
                insight={insight}
                showModal={showDeleteConfirm}
                onCancel={() => setShowDeleteConfirm(false)}
                telemetryRecorder={telemetryRecorder}
            />

            <ConfirmRemoveModal
                insight={insight}
                dashboard={currentDashboard}
                showModal={showRemoveConfirm}
                onCancel={() => setShowRemoveConfirm(false)}
                telemetryRecorder={telemetryRecorder}
            />

            <ShareLinkModal
                aria-label="Share insight"
                insight={insight}
                isOpen={showShareModal}
                onDismiss={() => setShowShareModal(false)}
            />

            <ExportInsightDataModal
                insightId={insight.id}
                insightTitle={insight.title}
                showModal={showExportDataConfirm}
                onCancel={() => setShowExportDataConfirm(false)}
                onConfirm={() => setShowExportDataConfirm(false)}
            />
        </>
    )
}

interface MenuSectionProps {
    name: string
    divider?: boolean
}

const MenuSection: FC<PropsWithChildren<MenuSectionProps>> = props => {
    const { name, children, divider = true } = props

    if (!children) {
        return null
    }

    return (
        <>
            <MenuHeader>{name}</MenuHeader>
            {children}
            {divider && <MenuDivider />}
        </>
    )
}
