import * as H from 'history'
import classnames from 'classnames'
import React, { useCallback, useMemo, useState } from 'react'
import { PageHeader } from '../../components/PageHeader'
import { PageTitle } from '../../components/PageTitle'
import { AuthenticatedUser } from '../../auth'
import { FilteredConnection } from '../../components/FilteredConnection'
import { CodeMonitorFields, ListUserCodeMonitorsResult, ListUserCodeMonitorsVariables } from '../../graphql-operations'
import { Link } from '../../../../shared/src/components/Link'
import { CodeMonitoringProps } from '.'
import PlusIcon from 'mdi-react/PlusIcon'
import { CodeMonitorNode, CodeMonitorNodeProps } from './CodeMonitoringNode'
import { catchError, map, startWith } from 'rxjs/operators'
import { asError, isErrorLike } from '../../../../shared/src/util/errors'
import { useObservable } from '../../../../shared/src/util/useObservable'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { SettingsCascadeProps } from '../../../../shared/src/settings/settings'
import { Settings } from '../../schema/settings.schema'
import { CodeMonitoringLogo } from './CodeMonitoringLogo'
import { FeedbackBadge } from '../../components/FeedbackBadge'

export interface CodeMonitoringPageProps
    extends Pick<CodeMonitoringProps, 'fetchUserCodeMonitors' | 'toggleCodeMonitorEnabled'>,
        SettingsCascadeProps<Settings> {
    authenticatedUser: AuthenticatedUser
    location: H.Location
    history: H.History
}

type CodeMonitorFilter = 'all' | 'user'

export const CodeMonitoringPage: React.FunctionComponent<CodeMonitoringPageProps> = props => {
    const { authenticatedUser, fetchUserCodeMonitors, toggleCodeMonitorEnabled } = props

    const queryConnection = useCallback(
        (args: Partial<ListUserCodeMonitorsVariables>) =>
            fetchUserCodeMonitors({
                id: authenticatedUser.id,
                first: args.first ?? null,
                after: args.after ?? null,
            }),
        [authenticatedUser, fetchUserCodeMonitors]
    )

    const LOADING = 'loading' as const

    const userHasCodeMonitors = useObservable(
        useMemo(
            () =>
                fetchUserCodeMonitors({
                    id: authenticatedUser.id,
                    first: 1,
                    after: null,
                }).pipe(
                    map(monitors => monitors.nodes.length > 0),
                    startWith(LOADING),
                    catchError(error => [asError(error)])
                ),
            [authenticatedUser.id, fetchUserCodeMonitors]
        )
    )

    const [monitorListFilter, setMonitorListFilter] = useState<CodeMonitorFilter>('all')

    const setAllFilter = useCallback<React.MouseEventHandler>(() => {
        setMonitorListFilter('all')
    }, [])

    const setUserFilter = useCallback<React.MouseEventHandler>(() => {
        setMonitorListFilter('user')
    }, [])

    return (
        <div className="code-monitoring-page">
            <PageTitle title="Code Monitoring" />
            <PageHeader
                annotation={<FeedbackBadge status="prototype" feedback={{ mailto: 'support@sourcegraph.com' }} />}
                path={[
                    {
                        icon: CodeMonitoringLogo,
                        text: 'Code monitoring',
                    },
                ]}
                actions={
                    userHasCodeMonitors &&
                    userHasCodeMonitors !== 'loading' &&
                    !isErrorLike(userHasCodeMonitors) && (
                        <Link to="/code-monitoring/new" className="btn btn-secondary">
                            <PlusIcon className="icon-inline" />
                            Create new code monitor
                        </Link>
                    )
                }
                className="mb-3"
            />
            {userHasCodeMonitors === 'loading' && <LoadingSpinner />}
            {!userHasCodeMonitors && (
                <div className="mt-5">
                    <div className="d-flex flex-column mb-5">
                        <h2>Get started with code monitoring</h2>
                        <p className="text-muted code-monitoring-page__start-subheading mb-4">
                            Watch your code for changes and trigger actions to get notifications, send webhooks, and
                            more. <a href="https://docs.sourcegraph.com/code_monitoring">Learn more.</a>
                        </p>
                        <Link to="/code-monitoring/new" className="code-monitoring-page__start-button btn btn-primary">
                            Create your first code monitor →
                        </Link>
                    </div>
                    <div className="code-monitoring-page__start-points container">
                        <h3>Starting points for your first monitor</h3>
                        <div className="row no-gutters code-monitoring-page__start-points-panel-container mb-3">
                            <div className="col-6">
                                <div className="code-monitoring-page__start-points-panel">
                                    <h3>Watch for AWS secrets in commits</h3>
                                    <p className="text-muted">
                                        Use a search query to watch for new search results, and choose how to receive
                                        notifications in response.
                                    </p>
                                    <a
                                        href="https://docs.sourcegraph.com/code_monitoring/how-tos/starting_points#watch-for-potential-secrets"
                                        className="btn btn-secondary"
                                    >
                                        View in docs →
                                    </a>
                                </div>
                            </div>
                            <div className="col-6">
                                <div className="code-monitoring-page__start-points-panel">
                                    <h3>Watch for new consumers of deprecated methods</h3>
                                    <p className="text-muted">
                                        Keep an eye on commits with new consumers of deprecated methods to keep your
                                        code base up-to-date.
                                    </p>
                                    <a
                                        href="https://docs.sourcegraph.com/code_monitoring/how-tos/starting_points#watch-for-consumers-of-deprecated-endpoints"
                                        className="btn btn-secondary"
                                    >
                                        View in docs →
                                    </a>
                                </div>
                            </div>
                        </div>
                        <a className="link" href="https://docs.sourcegraph.com/code_monitoring/how-tos/starting_points">
                            Find more starting points in the docs
                        </a>
                    </div>
                    <div className="code-monitoring-page__learn-more container mt-5">
                        <h3 className="mb-3">Learn more about code monitoring</h3>
                        <div className="row">
                            <div className="col-4">
                                <div>
                                    <h4>Core concepts</h4>
                                    <p className="text-muted">
                                        Craft searches that will monitor your code and trigger actions.{' '}
                                        <a
                                            href="https://docs.sourcegraph.com/code_monitoring/explanations/core_concepts"
                                            className="link"
                                        >
                                            Read the docs
                                        </a>
                                    </p>
                                </div>
                            </div>
                            <div className="col-4">
                                <div>
                                    <h4>Starting points and ideas</h4>
                                    <p className="text-muted">
                                        Find specific examples of useful code monitors to keep on top of security and
                                        consistency concerns.{' '}
                                        <a
                                            href="https://docs.sourcegraph.com/code_monitoring/how-tos/starting_points"
                                            className="link"
                                        >
                                            Explore starting points
                                        </a>
                                    </p>
                                </div>
                            </div>
                            <div className="col-4">
                                <div>
                                    <h4>Questions and feedback</h4>
                                    <p className="text-muted">
                                        We want to hear your feedback.{' '}
                                        <a href="mailto:feedback@sourcegraph.com" className="link">
                                            Share your thoughts
                                        </a>
                                    </p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            )}
            {userHasCodeMonitors && userHasCodeMonitors !== 'loading' && !isErrorLike(userHasCodeMonitors) && (
                <>
                    <div className="text-muted mb-4">
                        Watch your code for changes and trigger actions to get notifications, send webhooks, and more.{' '}
                        <a href="https://docs.sourcegraph.com/code_monitoring">Learn more.</a>
                    </div>
                    <div className="d-flex flex-column">
                        <div className="code-monitoring-page-tabs border-bottom mb-4">
                            <div className="nav nav-tabs border-bottom-0">
                                <div className="nav-item">
                                    <div className="nav-link active">Code monitors</div>
                                </div>
                            </div>
                        </div>
                        <div className="row mb-5">
                            <div className="d-flex flex-column col-2 mr-2">
                                <h3>Filters</h3>
                                <button
                                    type="button"
                                    className={classnames('btn text-left', {
                                        'btn-primary': monitorListFilter === 'all',
                                    })}
                                    onClick={setAllFilter}
                                >
                                    All
                                </button>
                                <button
                                    type="button"
                                    className={classnames('btn text-left', {
                                        'btn-primary': monitorListFilter === 'user',
                                    })}
                                    onClick={setUserFilter}
                                >
                                    Your code monitors
                                </button>
                            </div>
                            <div className="d-flex flex-column w-100 col">
                                <h3 className="mb-2">
                                    {`${monitorListFilter === 'all' ? 'All code monitors' : 'Your code monitors'}`}
                                </h3>
                                <FilteredConnection<
                                    CodeMonitorFields,
                                    Omit<CodeMonitorNodeProps, 'node'>,
                                    (ListUserCodeMonitorsResult['node'] & { __typename: 'User' })['monitors']
                                >
                                    location={props.location}
                                    history={props.history}
                                    defaultFirst={10}
                                    queryConnection={queryConnection}
                                    hideSearch={true}
                                    nodeComponent={CodeMonitorNode}
                                    nodeComponentProps={{
                                        authentictedUser: props.authenticatedUser,
                                        location: props.location,
                                        showCodeMonitoringTestEmailButton:
                                            (!isErrorLike(props.settingsCascade.final) &&
                                                props.settingsCascade.final?.experimentalFeatures
                                                    ?.showCodeMonitoringTestEmailButton) ||
                                            false,
                                        toggleCodeMonitorEnabled,
                                    }}
                                    noun="code monitor"
                                    pluralNoun="code monitors"
                                    noSummaryIfAllNodesVisible={true}
                                    cursorPaging={true}
                                />
                            </div>
                        </div>
                        <div className="mt-5">
                            We want to hear your feedback!{' '}
                            <a href="mailto:feedback@sourcegraph.com">Share your thoughts</a>
                        </div>
                    </div>
                </>
            )}
        </div>
    )
}
