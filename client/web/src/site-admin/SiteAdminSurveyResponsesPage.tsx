import React, { useEffect } from 'react'

import classNames from 'classnames'
import { Subscription } from 'rxjs'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import {
    Badge,
    Button,
    Card,
    H2,
    H3,
    Link,
    Tab,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
    Text,
    useLocalStorage,
    type BADGE_VARIANTS,
} from '@sourcegraph/wildcard'

import { FilteredConnection, type Filter } from '../components/FilteredConnection'
import { PageTitle } from '../components/PageTitle'
import {
    UserActivePeriod,
    type SurveyResponseAggregateFields,
    type SurveyResponseFields,
    type UserWithSurveyResponseFields,
} from '../graphql-operations'
import {
    fetchAllSurveyResponses,
    fetchAllUsersWithSurveyResponses,
    fetchSurveyResponseAggregates,
} from '../marketing/backend'
import { SURVEY_QUESTIONS } from '../marketing/components/SurveyUseCaseForm'
import { userURL } from '../user'

import { ValueLegendItem } from './analytics/components/ValueLegendList'

import styles from './SiteAdminSurveyResponsesPage.module.scss'

const USER_ACTIVITY_FILTERS: Filter[] = [
    {
        label: '',
        type: 'radio',
        id: 'user-activity-filters',
        options: [
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

interface SurveyResponseNodeProps {
    /**
     * The survey response to display in this list item.
     */
    node: SurveyResponseFields
}

function scoreToClassSuffix(score: number): typeof BADGE_VARIANTS[number] {
    return score > 8 ? 'success' : score > 6 ? 'info' : 'danger'
}

const ScoreBadge: React.FunctionComponent<React.PropsWithChildren<{ score: number }>> = props => (
    <Badge className="ml-4" pill={true} variant={scoreToClassSuffix(props.score)} tooltip={`${props.score} out of 10`}>
        Score: {props.score}
    </Badge>
)

const SurveyResponseNode: React.FunctionComponent<SurveyResponseNodeProps> = props => (
    <li className="list-group-item py-2">
        <div className="d-flex align-items-center justify-content-between">
            <div>
                <strong>
                    {props.node.user ? (
                        <Link to={userURL(props.node.user.username)}>{props.node.user.username}</Link>
                    ) : props.node.email ? (
                        props.node.email
                    ) : (
                        'anonymous user'
                    )}
                </strong>
                <ScoreBadge score={props.node.score} />
            </div>
            <div>
                <Timestamp date={props.node.createdAt} />
            </div>
        </div>
        {(props.node.reason || props.node.better) && (
            <dl className="mt-3">
                {props.node.otherUseCase && props.node.otherUseCase !== '' && (
                    <>
                        <dt>{SURVEY_QUESTIONS.otherUseCase}</dt>
                        <dd>{props.node.otherUseCase}</dd>
                    </>
                )}
                {props.node.reason && props.node.reason !== '' && (
                    <>
                        <dt>{SURVEY_QUESTIONS.reason}</dt>
                        <dd>{props.node.reason}</dd>
                    </>
                )}
                {props.node.better && props.node.better !== '' && (
                    <>
                        <dt>{SURVEY_QUESTIONS.better}</dt>
                        <dd>{props.node.better}</dd>
                    </>
                )}
            </dl>
        )}
    </li>
)

const UserSurveyResponsesHeader: React.FunctionComponent<
    React.PropsWithChildren<{ nodes: UserWithSurveyResponseFields[] }>
> = () => (
    <thead>
        <tr>
            <th>User</th>
            <th>Last active on Sourcegraph</th>
            <th>Latest survey response</th>
            <th />
        </tr>
    </thead>
)

interface UserSurveyResponseNodeProps {
    /**
     * The survey response to display in this list item.
     */
    node: UserWithSurveyResponseFields
}

interface UserSurveyResponseNodeState {
    displayAll: boolean
}

class UserSurveyResponseNode extends React.PureComponent<UserSurveyResponseNodeProps, UserSurveyResponseNodeState> {
    public state: UserSurveyResponseNodeState = {
        displayAll: false,
    }

    private showMoreClicked = (): void => this.setState(state => ({ displayAll: !state.displayAll }))

    public render(): JSX.Element | null {
        const responses = this.props.node.surveyResponses
        return (
            <>
                <tr>
                    <td>
                        <strong>
                            <Link to={userURL(this.props.node.username)}>{this.props.node.username}</Link>
                        </strong>
                    </td>
                    <td>
                        {this.props.node.usageStatistics?.lastActiveTime ? (
                            <Timestamp date={this.props.node.usageStatistics.lastActiveTime} />
                        ) : (
                            '?'
                        )}
                    </td>
                    <td>
                        {responses && responses.length > 0 ? (
                            <>
                                <Timestamp date={responses[0].createdAt} />
                                <ScoreBadge score={responses[0].score} />
                            </>
                        ) : (
                            <>No responses</>
                        )}
                    </td>
                    <td>
                        {responses.length > 0 && (
                            <Button onClick={this.showMoreClicked} variant="secondary" size="sm">
                                {this.state.displayAll ? 'Hide' : 'See all'}
                            </Button>
                        )}
                    </td>
                </tr>
                {this.state.displayAll && (
                    <tr>
                        <td colSpan={4}>
                            {responses.map((response, index) => (
                                <dl key={index}>
                                    <div className={classNames('pl-3 border-left', styles.wideBorder)}>
                                        <Timestamp date={response.createdAt} />
                                        <ScoreBadge score={response.score} />
                                        <br />
                                        {(response.otherUseCase || response.reason || response.better) && (
                                            <div className="mt-2" />
                                        )}
                                        {response.otherUseCase && response.otherUseCase !== '' && (
                                            <>
                                                <dt>{SURVEY_QUESTIONS.otherUseCase}</dt>
                                                <dd>{response.otherUseCase}</dd>
                                            </>
                                        )}
                                        {response.reason && response.reason !== '' && (
                                            <>
                                                <dt>{SURVEY_QUESTIONS.reason}</dt>
                                                <dd>{response.reason}</dd>
                                            </>
                                        )}
                                        {response.better && response.better !== '' && (
                                            <>
                                                <dt>{SURVEY_QUESTIONS.better}</dt>
                                                <dd>{response.better}</dd>
                                            </>
                                        )}
                                    </div>
                                </dl>
                            ))}
                        </td>
                    </tr>
                )}
            </>
        )
    }
}

interface SiteAdminSurveyResponsesSummaryState {
    summary?: SurveyResponseAggregateFields
}

class SiteAdminSurveyResponsesSummary extends React.PureComponent<{}, SiteAdminSurveyResponsesSummaryState> {
    private subscriptions = new Subscription()
    public state: SiteAdminSurveyResponsesSummaryState = {}
    constructor(props: {}) {
        super(props)
        this.subscriptions.add(fetchSurveyResponseAggregates().subscribe(summary => this.setState({ summary })))
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        if (!this.state.summary) {
            return null
        }
        const anyResults = this.state.summary.last30DaysCount > 0
        let npsText = `${this.state.summary.netPromoterScore}`
        if (this.state.summary.netPromoterScore > 0) {
            npsText = `+${npsText}`
        } else if (this.state.summary.netPromoterScore < 0) {
            npsText = `${npsText}`
        }
        const npsColor =
            this.state.summary.netPromoterScore > 0
                ? 'var(--success)'
                : this.state.summary.netPromoterScore < 0
                ? 'var(--danger)'
                : 'var(--info)'
        const roundAvg = Math.round(this.state.summary.averageScore * 10) / 10
        const avgColor = roundAvg > 8 ? 'var(--success)' : roundAvg > 6 ? 'var(--info)' : 'var(--danger)'
        return (
            <div className="mb-3">
                <div className="d-flex">
                    <H3>Summary</H3>
                    <Text className="ml-2 text-muted">(Last 30 days)</Text>
                </div>
                <Card className="d-flex justify-content-around flex-row p-5">
                    <ValueLegendItem
                        className={classNames('flex-1', styles.borderRight)}
                        value={this.state.summary.last30DaysCount}
                        description="Number of submissions"
                    />
                    <ValueLegendItem
                        className={classNames('flex-1', styles.borderRight)}
                        value={anyResults ? roundAvg : '-'}
                        description="Average score"
                        color={anyResults ? avgColor : undefined}
                        tooltip={`${roundAvg} out of 10`}
                    />
                    <ValueLegendItem
                        className="flex-1"
                        value={anyResults ? npsText : '-'}
                        description="Net promoter score"
                        color={anyResults ? npsColor : undefined}
                        tooltip={`${npsText} (between -100 and +100)`}
                    />
                </Card>
            </div>
        )
    }
}

interface Props extends TelemetryV2Props {}

const LAST_TAB_STORAGE_KEY = 'site-admin-survey-responses-last-tab'
/**
 * A page displaying the survey responses on this site.
 */

export const SiteAdminSurveyResponsesPage: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    telemetryRecorder,
}) => {
    const [persistedTabIndex, setPersistedTabIndex] = useLocalStorage(LAST_TAB_STORAGE_KEY, 0)

    useEffect(() => {
        EVENT_LOGGER.logViewEvent('SiteAdminSurveyResponses')
        telemetryRecorder.recordEvent('admin.surveyResponses', 'view')
    }, [telemetryRecorder])

    return (
        <div className="site-admin-survey-responses-page">
            <PageTitle title="User feedback survey - Admin" />
            <H2>User feedback survey</H2>
            <Text>
                After using Sourcegraph for a few days, users are presented with a request to answer "How likely is it
                that you would recommend Sourcegraph to a friend?" on a scale from 0–10 and to provide some feedback.
                Responses are visible below (and are also sent to Sourcegraph).
            </Text>

            <SiteAdminSurveyResponsesSummary />

            <H3>Responses</H3>

            <Tabs defaultIndex={persistedTabIndex} onChange={setPersistedTabIndex}>
                <TabList>
                    <Tab>Chronological feed</Tab>
                    <Tab>Sort by user</Tab>
                </TabList>
                <TabPanels>
                    <TabPanel>
                        <FilteredConnection<SurveyResponseFields, {}>
                            key="chronological"
                            className="list-group list-group-flush"
                            hideSearch={true}
                            noun="survey response"
                            pluralNoun="survey responses"
                            queryConnection={fetchAllSurveyResponses}
                            nodeComponent={SurveyResponseNode}
                        />
                    </TabPanel>
                    <TabPanel>
                        <FilteredConnection<UserWithSurveyResponseFields, {}>
                            key="by-user"
                            listComponent="table"
                            headComponent={UserSurveyResponsesHeader}
                            className="table mt-2 site-admin-survey-responses-connection"
                            hideSearch={false}
                            filters={USER_ACTIVITY_FILTERS}
                            noun="user"
                            pluralNoun="users"
                            queryConnection={fetchAllUsersWithSurveyResponses}
                            nodeComponent={UserSurveyResponseNode}
                        />
                    </TabPanel>
                </TabPanels>
            </Tabs>
        </div>
    )
}
