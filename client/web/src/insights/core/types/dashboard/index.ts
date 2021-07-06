import { InsightsDashboardType } from './core'
import { RealInsightDashboard } from './real-dashboard'
import { VirtualInsightsDashboard } from './virtual-dashboard'

/**
 * Main insight dashboard definition
 */
export type InsightDashboard = RealInsightDashboard | VirtualInsightsDashboard

export { InsightsDashboardType, RealInsightDashboard, VirtualInsightsDashboard }

/**
 * Key for accessing insights dashboards in a subject settings.
 */
export const INSIGHTS_DASHBOARDS_SETTINGS_KEY = 'insights.dashboards'

// Type guards for code insights dashboards
export const isOrganizationDashboard = (dashboard: InsightDashboard): dashboard is RealInsightDashboard =>
    dashboard.type === InsightsDashboardType.Organization
export const isPersonalDashboard = (dashboard: InsightDashboard): dashboard is RealInsightDashboard =>
    dashboard.type === InsightsDashboardType.Personal
export const isVirtualDashboard = (dashboard: InsightDashboard): dashboard is VirtualInsightsDashboard =>
    dashboard.type === InsightsDashboardType.All
