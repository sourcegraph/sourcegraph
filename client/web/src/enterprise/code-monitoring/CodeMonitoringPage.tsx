import React, { useMemo, useEffect, useState, useLayoutEffect, useCallback } from 'react'

import { mdiPlus } from '@mdi/js'
import classNames from 'classnames'
import { type Location, useNavigate, useLocation, type NavigateFunction } from 'react-router-dom'
import { of } from 'rxjs'
import { catchError, map } from 'rxjs/operators'

import { asError, isErrorLike } from '@sourcegraph/common'
import type { Settings } from '@sourcegraph/shared/src/schema/settings.schema'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    PageHeader,
    LoadingSpinner,
    useObservable,
    Button,
    Link,
    ProductStatusBadge,
    Icon,
    ButtonLink,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { CodeMonitoringLogo } from '../../code-monitoring/CodeMonitoringLogo'
import { PageTitle } from '../../components/PageTitle'
import { eventLogger } from '../../tracking/eventLogger'

import {
    fetchUserCodeMonitors as _fetchUserCodeMonitors,
    fetchCodeMonitors as _fetchCodeMonitors,
    toggleCodeMonitorEnabled as _toggleCodeMonitorEnabled,
} from './backend'
import { CodeMonitoringGettingStarted } from './CodeMonitoringGettingStarted'
import { CodeMonitoringLogs } from './CodeMonitoringLogs'
import { CodeMonitorList } from './CodeMonitorList'

type MonitorsTab = 'list' | 'getting-started' | 'logs'
type Tabs = { tab: MonitorsTab; title: string; isActive: boolean }[]

function getSelectedTabFromLocation(
    locationSearch: string,
    userHasCodeMonitors: boolean | Error | undefined
): MonitorsTab {
    const urlParameters = new URLSearchParams(locationSearch)
    switch (urlParameters.get('tab')) {
        case 'list': {
            return 'list'
        }
        case 'getting-started': {
            return 'getting-started'
        }
        case 'logs': {
            return 'logs'
        }
    }

    return userHasCodeMonitors ? 'list' : 'getting-started'
}

function setSelectedLocationTab(location: Location, navigate: NavigateFunction, selectedTab: MonitorsTab): void {
    const urlParameters = new URLSearchParams(location.search)
    urlParameters.set('tab', selectedTab)
    if (location.search !== urlParameters.toString()) {
        navigate({ ...location, search: urlParameters.toString() }, { replace: true })
    }
}

export interface CodeMonitoringPageProps extends SettingsCascadeProps<Settings>, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    fetchUserCodeMonitors?: typeof _fetchUserCodeMonitors
    fetchCodeMonitors?: typeof _fetchCodeMonitors
    toggleCodeMonitorEnabled?: typeof _toggleCodeMonitorEnabled
    // For testing purposes only
    testForceTab?: 'list' | 'getting-started' | 'logs'
}

export const CodeMonitoringPage: React.FunctionComponent<React.PropsWithChildren<CodeMonitoringPageProps>> = ({
    authenticatedUser,
    fetchUserCodeMonitors = _fetchUserCodeMonitors,
    fetchCodeMonitors = _fetchCodeMonitors,
    toggleCodeMonitorEnabled = _toggleCodeMonitorEnabled,
    testForceTab,
    telemetryRecorder,
}) => {
    const userHasCodeMonitors = useObservable(
        useMemo(
            () =>
                authenticatedUser
                    ? fetchUserCodeMonitors({
                          id: authenticatedUser.id,
                          first: 1,
                          after: null,
                      }).pipe(
                          map(monitors => monitors.nodes.length > 0),
                          catchError(error => [asError(error)])
                      )
                    : of(false),
            [authenticatedUser, fetchUserCodeMonitors]
        )
    )

    const navigate = useNavigate()
    const location = useLocation()

    const [currentTab, setCurrentTab] = useState<MonitorsTab>(() =>
        getSelectedTabFromLocation(location.search, userHasCodeMonitors)
    )

    const onSelectTab = useCallback(
        (tab: MonitorsTab) => {
            setCurrentTab(tab)
            setSelectedLocationTab(location, navigate, tab)
        },
        [navigate, location, setCurrentTab]
    )

    // Force tab for testing
    useLayoutEffect(() => {
        if (testForceTab && testForceTab !== currentTab) {
            setCurrentTab(testForceTab)
        }
    }, [currentTab, testForceTab])

    // Log page view of selected tab
    useEffect(() => {
        if (userHasCodeMonitors !== undefined) {
            switch (currentTab) {
                case 'getting-started': {
                    eventLogger.logPageView('CodeMonitoringGettingStartedPage')
                    telemetryRecorder.recordEvent('codeMonitor.gettingStarted', 'view')
                    break
                }
                case 'logs': {
                    eventLogger.logPageView('CodeMonitoringLogsPage')
                    telemetryRecorder.recordEvent('codeMonitor.logs', 'view')
                    break
                }
                case 'list': {
                    eventLogger.logPageView('CodeMonitoringPage')
                    telemetryRecorder.recordEvent('codeMonitor.list', 'view')
                }
            }
        }
    }, [currentTab, userHasCodeMonitors, telemetryRecorder])

    const showList = userHasCodeMonitors !== undefined && !isErrorLike(userHasCodeMonitors) && currentTab === 'list'

    const tabs: Tabs = useMemo(
        () => [
            {
                tab: 'list',
                title: 'Code monitors',
                isActive: currentTab === 'list',
            },
            {
                tab: 'getting-started',
                title: 'Getting started',
                isActive: currentTab === 'getting-started',
            },
            {
                tab: 'logs',
                title: 'Logs',
                isActive: currentTab === 'logs',
            },
        ],
        [currentTab]
    )

    return (
        <div className="code-monitoring-page" data-testid="code-monitoring-page">
            <PageTitle title="Code Monitoring" />
            <PageHeader
                actions={
                    authenticatedUser && (
                        <Button to="/code-monitoring/new" variant="primary" as={Link}>
                            <Icon aria-hidden={true} svgPath={mdiPlus} /> Create a code monitor
                        </Button>
                    )
                }
                description={
                    <>Watch your code for changes and trigger actions to get notifications, send webhooks, and more.</>
                }
                className="mb-3"
            >
                <PageHeader.Heading as="h2" styleAs="h1">
                    <PageHeader.Breadcrumb icon={CodeMonitoringLogo}>Code monitoring</PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>

            {userHasCodeMonitors === undefined ? (
                <LoadingSpinner inline={false} />
            ) : (
                <div className="d-flex flex-column">
                    <div className="code-monitoring-page-tabs mb-4">
                        <div className="nav nav-tabs">
                            {tabs.map(({ tab, title, isActive }) => (
                                <div className="nav-item" key={tab}>
                                    <ButtonLink
                                        to=""
                                        role="button"
                                        onSelect={event => {
                                            event.preventDefault()
                                            onSelectTab(tab)
                                        }}
                                        className={classNames('nav-link', isActive && 'active')}
                                    >
                                        <span>
                                            {title}
                                            {tab === 'logs' && <ProductStatusBadge status="beta" className="ml-2" />}
                                        </span>
                                    </ButtonLink>
                                </div>
                            ))}
                        </div>
                    </div>

                    {currentTab === 'getting-started' && (
                        <CodeMonitoringGettingStarted
                            authenticatedUser={authenticatedUser}
                            telemetryRecorder={telemetryRecorder}
                        />
                    )}

                    {currentTab === 'logs' && <CodeMonitoringLogs />}

                    {showList && (
                        <CodeMonitorList
                            authenticatedUser={authenticatedUser}
                            fetchUserCodeMonitors={fetchUserCodeMonitors}
                            fetchCodeMonitors={fetchCodeMonitors}
                            toggleCodeMonitorEnabled={toggleCodeMonitorEnabled}
                            telemetryRecorder={telemetryRecorder}
                        />
                    )}
                </div>
            )}
        </div>
    )
}
