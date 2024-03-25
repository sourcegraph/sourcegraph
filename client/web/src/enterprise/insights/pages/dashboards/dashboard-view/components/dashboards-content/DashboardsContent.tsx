import { type FC, useEffect, useState } from 'react'

import classNames from 'classnames'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import ViewDashboardOutlineIcon from 'mdi-react/ViewDashboardOutlineIcon'
import { useNavigate } from 'react-router-dom'

import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary/useTemporarySetting'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Button, Link, Tooltip } from '@sourcegraph/wildcard'

import { HeroPage } from '../../../../../../../components/HeroPage'
import { type GridApi, LimitedAccessLabel } from '../../../../../components'
import type { CustomInsightDashboard } from '../../../../../core'
import { useCopyURLHandler, useUiFeatures } from '../../../../../hooks'
import { AddInsightModal } from '../add-insight-modal'
import { DashboardMenu, DashboardMenuAction } from '../dashboard-menu/DashboardMenu'
import { DashboardSelect } from '../dashboard-select/DashboardSelect'
import { DeleteDashboardModal } from '../delete-dashboard-modal/DeleteDashboardModal'

import { DashboardHeader } from './components/dashboard-header/DashboardHeader'
import { DashboardInsights } from './components/dashboard-inisghts/DashboardInsights'

import styles from './DashboardsContent.module.scss'

export interface DashboardsContentProps extends TelemetryProps, TelemetryV2Props {
    currentDashboard: CustomInsightDashboard | undefined
    dashboards: CustomInsightDashboard[]
}

export const DashboardsContent: FC<DashboardsContentProps> = props => {
    const { currentDashboard, dashboards, telemetryService, telemetryRecorder } = props

    const navigate = useNavigate()
    const [, setLasVisitedDashboard] = useTemporarySetting('insights.lastVisitedDashboardId', null)
    const { dashboard: dashboardPermission, licensed } = useUiFeatures()

    const [copyURL, isCopied] = useCopyURLHandler()
    const [isAddInsightOpen, setAddInsightsState] = useState<boolean>(false)
    const [isDeleteDashboardActive, setDeleteDashboardActive] = useState<boolean>(false)
    const [dashboardGridApi, setDashboardGridApi] = useState<GridApi>()

    useEffect(() => {
        telemetryService.logViewEvent('Insights')
        telemetryRecorder.recordEvent('insights.dashboard', 'view')
    }, [telemetryService, telemetryRecorder])
    useEffect(() => setLasVisitedDashboard(currentDashboard?.id ?? null), [currentDashboard, setLasVisitedDashboard])

    const handleDashboardSelect = (dashboard: CustomInsightDashboard): void =>
        navigate(`/insights/dashboards/${dashboard.id}`)

    const handleSelect = (action: DashboardMenuAction): void => {
        switch (action) {
            case DashboardMenuAction.Configure: {
                if (currentDashboard) {
                    navigate(`/insights/dashboards/${currentDashboard.id}/edit`)
                }
                return
            }
            case DashboardMenuAction.Delete: {
                setDeleteDashboardActive(true)
                return
            }
            case DashboardMenuAction.CopyLink: {
                copyURL()
                return
            }
            case DashboardMenuAction.ResetGridLayout: {
                dashboardGridApi?.resetGridLayout()
                return
            }
        }
    }

    const addRemovePermissions = dashboardPermission.getAddRemoveInsightsPermission(currentDashboard)

    return (
        <div className={styles.root}>
            <DashboardHeader className={styles.header}>
                <span className={styles.dashboardSelectLabel}>Dashboard:</span>

                <DashboardSelect
                    dashboard={currentDashboard}
                    dashboards={dashboards}
                    className={styles.dashboardSelect}
                    onSelect={handleDashboardSelect}
                />

                <DashboardMenu
                    dashboard={currentDashboard}
                    tooltipText={isCopied ? 'Copied!' : undefined}
                    className={styles.dashboardMenu}
                    onSelect={handleSelect}
                />

                <Tooltip content={addRemovePermissions.tooltip} placement="bottom">
                    <Button
                        outline={true}
                        variant="secondary"
                        disabled={addRemovePermissions.disabled}
                        onClick={() => setAddInsightsState(true)}
                    >
                        Add or remove insights
                    </Button>
                </Tooltip>
            </DashboardHeader>

            {!licensed && (
                <LimitedAccessLabel
                    className={classNames(styles.limitedAccessLabel)}
                    message="Unlock Code Insights for full access to custom dashboards"
                />
            )}

            {currentDashboard ? (
                <DashboardInsights
                    currentDashboard={currentDashboard}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                    className={styles.insights}
                    onAddInsightRequest={() => setAddInsightsState(true)}
                    onDashboardCreate={setDashboardGridApi}
                />
            ) : (
                <DashboardEmptyContent dashboards={dashboards} />
            )}

            {isAddInsightOpen && currentDashboard && (
                <AddInsightModal dashboard={currentDashboard} onClose={() => setAddInsightsState(false)} />
            )}

            {isDeleteDashboardActive && currentDashboard && (
                <DeleteDashboardModal dashboard={currentDashboard} onClose={() => setDeleteDashboardActive(false)} />
            )}
        </div>
    )
}

interface DashboardEmptyContentProps {
    dashboards: CustomInsightDashboard[]
}

const DashboardEmptyContent: FC<DashboardEmptyContentProps> = props => {
    const { dashboards } = props

    if (dashboards.length === 0) {
        return (
            <HeroPage
                lessPadding={true}
                icon={ViewDashboardOutlineIcon}
                title="Your dashboard will appear here"
                subtitle="Your instance does not have any dashboards or you may not have permissions to view them."
                body={
                    <Button as={Link} to="/insights/add-dashboard" variant="primary" className="mt-4">
                        Create your first dashboard
                    </Button>
                }
            />
        )
    }

    return <HeroPage icon={MapSearchIcon} title="Hmm, the dashboard wasn't found." />
}
