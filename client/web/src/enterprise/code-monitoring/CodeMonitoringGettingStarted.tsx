import React, { useCallback } from 'react'

import { mdiPlus } from '@mdi/js'
import classNames from 'classnames'

import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Link, Button, CardBody, Card, Icon, H2, H3, H4, Text } from '@sourcegraph/wildcard'

import { CallToActionBanner } from '../../components/CallToActionBanner'
import { eventLogger } from '../../tracking/eventLogger'

import styles from './CodeMonitoringGettingStarted.module.scss'

interface CodeMonitoringGettingStartedProps extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
}

interface ExampleCodeMonitor {
    title: string
    description: string
    monitorName: string
    monitorQuery: string
}

const exampleCodeMonitors: ExampleCodeMonitor[] = [
    {
        title: 'Uses of a deprecated method',
        description:
            'Get notified when a deprecated method is added or removed. This example uses leftPad in JavaScript files.',
        monitorName: 'Uses of leftPad in JavaScript',
        monitorQuery: 'lang:JavaScript require(("|\')left-pad("|\')) patternType:regexp type:diff ',
    },
    {
        title: 'New library usage',
        description:
            'After you add a new library, you can watch your codebase for its usage and get notified when it’s imported or certain functions from it are used. This example uses faker in TypeScript.',
        monitorName: 'New uses of faker in TypeScript',
        monitorQuery:
            'lang:TypeScript import faker from "faker" OR import faker from \'faker\' type:diff select:commit.diff.added ',
    },
    {
        title: 'Bad coding patterns',
        description:
            'Get notified when someone uses a pattern that your team is trying to avoid. This example uses React class components in JavaScript.',
        monitorName: 'New React class components in JavaScript',
        monitorQuery:
            'lang:JavaScript class \\w extends React.Component type:diff patternType:regexp select:commit.diff.added ',
    },
    {
        title: 'IP address range',
        description:
            'Detect the usage of banned or invalid IP addresses in your code. This example uses local IP address in the 192.168.1.x range.',
        monitorName: 'New uses of local IP addresses',
        monitorQuery: '^192\\.168\\.1\\.([1-9]|[1-9]d|100)$ type:diff select:commit.diff.added patternType:regexp ',
    },
]

const createCodeMonitorUrl = (example: ExampleCodeMonitor): string => {
    const searchParameters = new URLSearchParams()
    searchParameters.set('trigger-query', example.monitorQuery)
    searchParameters.set('description', example.monitorName)
    return `/code-monitoring/new?${searchParameters.toString()}`
}

export const CodeMonitoringGettingStarted: React.FunctionComponent<
    React.PropsWithChildren<CodeMonitoringGettingStartedProps>
> = ({ authenticatedUser, telemetryRecorder }) => {
    const isLightTheme = useIsLightTheme()
    const isSourcegraphDotCom: boolean = window.context?.sourcegraphDotComMode || false
    const assetsRoot = window.context?.assetsRoot || ''

    const logExampleMonitorClicked = useCallback(() => {
        eventLogger.log('CodeMonitoringExampleMonitorClicked')
        telemetryRecorder.recordEvent('codeMonitor.example', 'click')
    }, [telemetryRecorder])

    const ctaBannerUrl = 'https://sourcegraph.com/get-started?t=enterprise'

    return (
        <div>
            <Card className={classNames('mb-4 flex-column flex-lg-row', styles.hero)}>
                <img
                    src={`${assetsRoot}/img/codemonitoring-illustration-${isLightTheme ? 'light' : 'dark'}.svg`}
                    alt="A code monitor observes a depcreated library being used in code and sends an email alert."
                    className={classNames('mr-lg-5', styles.heroImage)}
                />
                <div className="align-self-center">
                    <H2 className={classNames('mb-3', styles.heading)}>Proactively monitor changes to your codebase</H2>
                    <Text className={classNames('mb-4')}>
                        With code monitoring, you can automatically track changes made across multiple code hosts and
                        repositories.
                    </Text>

                    <H3>Common use cases</H3>
                    <ul>
                        <li>Identify when bad patterns are committed </li>
                        <li>Identify use of deprecated libraries</li>
                    </ul>
                    {authenticatedUser && (
                        <Button to="/code-monitoring/new" className={styles.createButton} variant="primary" as={Link}>
                            <Icon aria-hidden={true} className="mr-2" svgPath={mdiPlus} />
                            Create a code monitor
                        </Button>
                    )}
                </div>
            </Card>

            {isSourcegraphDotCom && (
                <CallToActionBanner variant="filled">
                    To monitor changes across your team's private repositories,{' '}
                    <Link
                        to={ctaBannerUrl}
                        onClick={() => {
                            eventLogger.log('ClickedOnEnterpriseCTA', { location: 'MonitoringGettingStarted' })
                            telemetryRecorder.recordEvent('codeMonitor.enterpriseCTA', 'click', {
                                metadata: { location: 0 },
                            })
                        }}
                    >
                        get Sourcegraph Enterprise
                    </Link>
                    .
                </CallToActionBanner>
            )}

            <div>
                <H3 className="mb-3">Example code monitors</H3>

                <div className={classNames('mb-3', styles.startingPointsContainer)}>
                    {exampleCodeMonitors.map(monitor => (
                        <div className={styles.startingPoint} key={monitor.title}>
                            <Card className="h-100">
                                <CardBody className="d-flex flex-column">
                                    <H3>{monitor.title}</H3>
                                    <Text className="text-muted flex-grow-1">{monitor.description}</Text>
                                    <Link to={createCodeMonitorUrl(monitor)} onClick={logExampleMonitorClicked}>
                                        Create copy of monitor
                                    </Link>
                                </CardBody>
                            </Card>
                        </div>
                    ))}
                </div>
            </div>
            <div className="mt-5 px-0">
                <div className="row">
                    <div className="col-4">
                        <div>
                            <H4>Get started</H4>
                            <Text className="text-muted">
                                Craft searches that will monitor your code and trigger actions such as email
                                notifications.
                            </Text>
                            <Link to="/help/code_monitoring" className="link">
                                Code monitoring documentation
                            </Link>
                        </div>
                    </div>
                    <div className="col-4">
                        <div>
                            <H4>Starting points and ideas</H4>
                            <Text className="text-muted">
                                Find specific examples of useful code monitors to keep on top of security and
                                consistency concerns.
                            </Text>
                            <Link to="/help/code_monitoring/how-tos/starting_points" className="link">
                                Explore starting points
                            </Link>
                        </div>
                    </div>
                    <div className="col-4">
                        <div>
                            <H4>Questions and feedback</H4>
                            <Text className="text-muted">
                                Have a question or idea about code monitoring? We want to hear your feedback!
                            </Text>
                            <Link to="mailto:feedback@sourcegraph.com" className="link">
                                Share your thoughts
                            </Link>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}
