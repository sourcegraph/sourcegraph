import format from 'date-fns/format'
import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Subscription } from 'rxjs'
import * as GQL from '../../../shared/src/graphql/schema'
import { BarChart } from '../components/d3/BarChart'
import { FilteredConnection, FilteredConnectionFilter } from '../components/FilteredConnection'
import { PageTitle } from '../components/PageTitle'
import { RadioButtons } from '../components/RadioButtons'
import { Timestamp } from '../components/time/Timestamp'
import { eventLogger } from '../tracking/eventLogger'
import { fetchSiteUsageStatistics, fetchUserUsageStatistics } from './backend'
import { ErrorAlert } from '../components/alerts'
import FileDownloadIcon from 'mdi-react/FileDownloadIcon'
import { UserActivePeriod } from '../../../shared/src/graphql-operations'

interface ChartData {
    label: string
    dateFormat: string
}

type ChartOptions = Record<'daus' | 'waus' | 'maus', ChartData>

const chartGeneratorOptions: ChartOptions = {
    daus: { label: 'Daily unique users', dateFormat: 'E, MMM d' },
    waus: { label: 'Weekly unique users', dateFormat: 'E, MMM d' },
    maus: { label: 'Monthly unique users', dateFormat: 'MMMM yyyy' },
}

const CHART_ID_KEY = 'latest-usage-statistics-chart-id'

interface UsageChartPageProps {
    isLightTheme: boolean
    stats: GQL.ISiteUsageStatistics
    chartID: keyof ChartOptions
    header?: JSX.Element
    showLegend?: boolean
}

export const UsageChart: React.FunctionComponent<UsageChartPageProps> = (props: UsageChartPageProps) => (
    <div className="site-admin-usage-statistics-page">
        {props.header ? props.header : <h3>{chartGeneratorOptions[props.chartID].label}</h3>}
        <BarChart
            showLabels={true}
            showLegend={props.showLegend === undefined ? true : props.showLegend}
            width={500}
            height={200}
            isLightTheme={props.isLightTheme}
            data={props.stats[props.chartID].map(usagePeriod => ({
                xLabel: format(
                    Date.parse(usagePeriod.startTime) + 1000 * 60 * 60 * 24,
                    chartGeneratorOptions[props.chartID].dateFormat
                ),
                yValues: {
                    Registered: usagePeriod.registeredUserCount,
                    'Deleted or anonymous': usagePeriod.anonymousUserCount,
                },
            }))}
        />
        <small className="site-admin-usage-statistics-page__tz-note">
            <i>GMT/UTC time</i>
        </small>
    </div>
)

interface UserUsageStatisticsHeaderFooterProps {
    nodes: GQL.IUser[]
}

class UserUsageStatisticsHeader extends React.PureComponent<UserUsageStatisticsHeaderFooterProps> {
    public render(): JSX.Element | null {
        return (
            <thead>
                <tr>
                    <th>User</th>
                    <th>Page views</th>
                    <th>Search queries</th>
                    <th>Code intelligence actions</th>
                    <th className="site-admin-usage-statistics-page__date-column">Last active</th>
                    <th className="site-admin-usage-statistics-page__date-column">
                        Last active in code host or code review
                    </th>
                </tr>
            </thead>
        )
    }
}

class UserUsageStatisticsFooter extends React.PureComponent<UserUsageStatisticsHeaderFooterProps> {
    public render(): JSX.Element | null {
        return (
            <tfoot>
                <tr>
                    <th>Total</th>
                    <td>
                        {this.props.nodes.reduce(
                            (count, node) => count + (node.usageStatistics ? node.usageStatistics.pageViews : 0),
                            0
                        )}
                    </td>
                    <td>
                        {this.props.nodes.reduce(
                            (count, node) => count + (node.usageStatistics ? node.usageStatistics.searchQueries : 0),
                            0
                        )}
                    </td>
                    <td>
                        {this.props.nodes.reduce(
                            (count, node) =>
                                count + (node.usageStatistics ? node.usageStatistics.codeIntelligenceActions : 0),
                            0
                        )}
                    </td>
                    <td className="site-admin-usage-statistics-page__date-column" />
                    <td className="site-admin-usage-statistics-page__date-column" />
                </tr>
            </tfoot>
        )
    }
}

interface UserUsageStatisticsNodeProps {
    /**
     * The user to display in this list item.
     */
    node: GQL.IUser
}

class UserUsageStatisticsNode extends React.PureComponent<UserUsageStatisticsNodeProps> {
    public render(): JSX.Element | null {
        return (
            <tr>
                <td>{this.props.node.username}</td>
                <td>{this.props.node.usageStatistics ? this.props.node.usageStatistics.pageViews : 'n/a'}</td>
                <td>{this.props.node.usageStatistics ? this.props.node.usageStatistics.searchQueries : 'n/a'}</td>
                <td>
                    {this.props.node.usageStatistics ? this.props.node.usageStatistics.codeIntelligenceActions : 'n/a'}
                </td>
                <td className="site-admin-usage-statistics-page__date-column">
                    {this.props.node.usageStatistics?.lastActiveTime ? (
                        <Timestamp date={this.props.node.usageStatistics.lastActiveTime} />
                    ) : (
                        'never'
                    )}
                </td>
                <td className="site-admin-usage-statistics-page__date-column">
                    {this.props.node.usageStatistics?.lastActiveCodeHostIntegrationTime ? (
                        <Timestamp date={this.props.node.usageStatistics.lastActiveCodeHostIntegrationTime} />
                    ) : (
                        'never'
                    )}
                </td>
            </tr>
        )
    }
}

class FilteredUserConnection extends FilteredConnection<GQL.IUser, {}> {}
export const USER_ACTIVITY_FILTERS: FilteredConnectionFilter[] = [
    {
        label: '',
        type: 'radio',
        id: 'user-activity-filters',
        values: [
            {
                label: 'All users',
                value: 'all',
                tooltip: 'Show all users',
                args: { activePeriod: UserActivePeriod.ALL_TIME },
            },
            {
                label: 'Active today',
                value: 'today',
                tooltip: 'Show users active since this morning at 00:00 UTC',
                args: { activePeriod: UserActivePeriod.TODAY },
            },
            {
                label: 'Active this week',
                value: 'week',
                tooltip: 'Show users active since Monday at 00:00 UTC',
                args: { activePeriod: UserActivePeriod.THIS_WEEK },
            },
            {
                label: 'Active this month',
                value: 'month',
                tooltip: 'Show users active since the first day of the month at 00:00 UTC',
                args: { activePeriod: UserActivePeriod.THIS_MONTH },
            },
        ],
    },
]

interface SiteAdminUsageStatisticsPageProps extends RouteComponentProps<{}> {
    isLightTheme: boolean
}

interface SiteAdminUsageStatisticsPageState {
    users?: GQL.IUserConnection
    stats?: GQL.ISiteUsageStatistics
    error?: Error
    chartID: keyof ChartOptions
}

/**
 * A page displaying usage statistics for the site.
 */
export class SiteAdminUsageStatisticsPage extends React.Component<
    SiteAdminUsageStatisticsPageProps,
    SiteAdminUsageStatisticsPageState
> {
    public state: SiteAdminUsageStatisticsPageState = {
        chartID: this.loadLatestChartFromStorage(),
    }

    private subscriptions = new Subscription()

    private loadLatestChartFromStorage(): keyof ChartOptions {
        const latest = localStorage.getItem(CHART_ID_KEY)
        return latest && latest in chartGeneratorOptions ? (latest as keyof ChartOptions) : 'daus'
    }

    public componentDidMount(): void {
        eventLogger.logViewEvent('SiteAdminUsageStatistics')

        this.subscriptions.add(
            fetchSiteUsageStatistics().subscribe(
                stats => this.setState({ stats }),
                error => this.setState({ error })
            )
        )
    }

    public componentDidUpdate(): void {
        localStorage.setItem(CHART_ID_KEY, this.state.chartID)
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <div className="site-admin-usage-statistics-page">
                <PageTitle title="Usage statistics - Admin" />
                <h2>Usage statistics</h2>
                {this.state.error && <ErrorAlert className="mb-3" error={this.state.error} />}

                <a
                    href="/site-admin/usage-statistics/archive"
                    className="btn btn-secondary"
                    data-tooltip="Download usage stats archive"
                    download="true"
                >
                    <FileDownloadIcon className="icon-inline" /> Download usage stats archive
                </a>

                {this.state.stats && (
                    <>
                        <RadioButtons
                            nodes={Object.entries(chartGeneratorOptions).map(([key, { label }]) => ({
                                label,
                                id: key,
                            }))}
                            onChange={this.onChartIndexChange}
                            selected={this.state.chartID}
                        />
                        <UsageChart {...this.props} chartID={this.state.chartID} stats={this.state.stats} />
                    </>
                )}
                <h3 className="mt-4">All registered users</h3>
                {!this.state.error && (
                    <FilteredUserConnection
                        listComponent="table"
                        className="table"
                        hideSearch={false}
                        filters={USER_ACTIVITY_FILTERS}
                        noShowMore={false}
                        noun="user"
                        pluralNoun="users"
                        queryConnection={fetchUserUsageStatistics}
                        nodeComponent={UserUsageStatisticsNode}
                        headComponent={UserUsageStatisticsHeader}
                        footComponent={UserUsageStatisticsFooter}
                        history={this.props.history}
                        location={this.props.location}
                    />
                )}
            </div>
        )
    }

    private onChartIndexChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
        switch (event.target.value as keyof ChartOptions) {
            case 'daus':
                eventLogger.log('DAUsChartSelected')
                break
            case 'waus':
                eventLogger.log('WAUsChartSelected')
                break
            case 'maus':
                eventLogger.log('MAUsChartSelected')
                break
        }
        this.setState({ chartID: event.target.value as keyof ChartOptions })
    }
}
