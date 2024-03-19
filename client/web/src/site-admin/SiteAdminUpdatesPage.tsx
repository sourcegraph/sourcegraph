import React, { type FunctionComponent, useCallback, useEffect, useMemo, useState } from 'react'

import { mdiOpenInNew, mdiCheckCircle, mdiChevronUp, mdiChevronDown, mdiAlertOctagram, mdiContentCopy } from '@mdi/js'
import classNames from 'classnames'
import { parseISO, formatDistance } from 'date-fns'

import { Toggle } from '@sourcegraph/branded/src/components/Toggle'
import { useQuery, useMutation } from '@sourcegraph/http-client'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    LoadingSpinner,
    Link,
    PageHeader,
    Alert,
    Icon,
    Code,
    Container,
    Text,
    ErrorAlert,
    Button,
    Collapse,
    CollapseHeader,
    CollapsePanel,
    H3,
    H4,
    Label,
    AnchorLink,
} from '@sourcegraph/wildcard'

import { LogOutput } from '../components/LogOutput'
import { PageTitle } from '../components/PageTitle'
import type {
    SetAutoUpgradeResult,
    SetAutoUpgradeVariables,
    SiteUpdateCheckResult,
    SiteUpdateCheckVariables,
    SiteUpgradeReadinessResult,
    SiteUpgradeReadinessVariables,
} from '../graphql-operations'

import { SITE_UPDATE_CHECK, SITE_UPGRADE_READINESS, SET_AUTO_UPGRADE } from './backend'

import styles from './SiteAdminUpdatesPage.module.scss'

interface Props extends TelemetryProps {}
const capitalize = (text: string): string => (text && text[0].toUpperCase() + text.slice(1)) || ''

const SiteUpdateCheck: React.FC = () => {
    const { data, loading, error } = useQuery<SiteUpdateCheckResult, SiteUpdateCheckVariables>(SITE_UPDATE_CHECK, {})
    const autoUpdateCheckingEnabled = window.context.site['update.channel'] === 'release'

    return (
        <>
            {error && !loading && <ErrorAlert error={error} />}
            {loading && !error && <LoadingSpinner />}
            {data && (
                <>
                    <Text className="mb-1">
                        Version {data.site.productVersion}{' '}
                        <small className="text-muted">
                            (
                            <Link to="https://sourcegraph.com/changelog" target="_blank" rel="noopener">
                                changelog
                            </Link>
                            )
                        </small>
                        <br />
                    </Text>

                    <div>
                        {data.site.updateCheck.pending && (
                            <Alert className={styles.alert} variant="primary">
                                <LoadingSpinner /> Checking for updates... (reload in a few seconds)
                            </Alert>
                        )}
                        {data.site.updateCheck.errorMessage && (
                            <ErrorAlert
                                className={styles.alert}
                                prefix="Error checking for updates"
                                error={data.site.updateCheck.errorMessage}
                            />
                        )}
                        {!data.site.updateCheck.errorMessage && (
                            <small>
                                {data.site.updateCheck.updateVersionAvailable ? (
                                    <AnchorLink
                                        to="/help/admin/updates"
                                        target="_blank"
                                        rel="noopener"
                                        className="ml-1"
                                    >
                                        Update available to version {data.site.updateCheck.updateVersionAvailable}{' '}
                                        <Icon aria-hidden={true} svgPath={mdiOpenInNew} />
                                    </AnchorLink>
                                ) : (
                                    <span>
                                        <Icon
                                            aria-hidden={true}
                                            className="text-success mr-1"
                                            svgPath={mdiCheckCircle}
                                        />
                                        Up to date
                                    </span>
                                )}
                                <span className={classNames('text-muted pl-2 ml-2', styles.lastChecked)}>
                                    {data.site.updateCheck.checkedAt
                                        ? `Last checked ${formatDistance(
                                              parseISO(data.site.updateCheck.checkedAt),
                                              new Date(),
                                              {
                                                  addSuffix: true,
                                              }
                                          )}`
                                        : 'Never checked for updates'}
                                </span>
                            </small>
                        )}
                    </div>
                </>
            )}

            <small>
                {autoUpdateCheckingEnabled
                    ? 'Automatically checking for updates.'
                    : 'Automatic checking for updates disabled.'}{' '}
                Change <Code>update.channel</Code> in <Link to="/site-admin/configuration">site configuration</Link> to{' '}
                {autoUpdateCheckingEnabled ? 'disable' : 'enable'} automatic checking.
            </small>
        </>
    )
}

const SiteUpgradeReadiness: FunctionComponent = () => {
    const { data, loading, error, refetch } = useQuery<SiteUpgradeReadinessResult, SiteUpgradeReadinessVariables>(
        SITE_UPGRADE_READINESS,
        {}
    )

    const exportDrift = useCallback(() => {
        if (!data) {
            return
        }

        const content = JSON.stringify(data.site.upgradeReadiness.schemaDrift)

        // Followed this advice on SO :shrug:
        // https://stackoverflow.com/questions/44656610/download-a-string-as-txt-file-in-react

        const element = document.createElement('a')
        element.download = 'drift.json'
        element.href = URL.createObjectURL(new Blob([content], { type: 'application/json' }))
        document.body.append(element)
        element.click()
    }, [data])

    const [setAutoUpgrade] = useMutation<SetAutoUpgradeResult, SetAutoUpgradeVariables>(SET_AUTO_UPGRADE)
    const [autoUpgradeEnabled, setAutoUpgradeEnabled] = useState(data?.site.autoUpgradeEnabled)
    const handleToggle = async (): Promise<void> => {
        setAutoUpgradeEnabled(!autoUpgradeEnabled)
        await setAutoUpgrade({
            variables: { enable: !autoUpgradeEnabled },
        })
    }
    useEffect(() => {
        if (data) {
            setAutoUpgradeEnabled(data.site.autoUpgradeEnabled)
        }
    }, [data])
    const [isExpanded, setIsExpanded] = useState(true)
    return (
        <>
            {error && !loading && <ErrorAlert error={error} />}
            {loading && !error && <LoadingSpinner />}
            {data && !loading && (
                <>
                    <div className="d-flex flex-row justify-content-between">
                        <H3>Automatic Upgrade State</H3>
                        <div>
                            <Label>
                                <Toggle
                                    title="Enable Auto Upgrade"
                                    value={autoUpgradeEnabled}
                                    onToggle={handleToggle}
                                    className="mr-2"
                                    aria-describedby="auto-upgrade-toggle-description"
                                />
                                {autoUpgradeEnabled &&
                                (data.site.upgradeReadiness.requiredOutOfBandMigrations.length > 0 ||
                                    data.site.upgradeReadiness.schemaDrift.length > 0) ? (
                                    <Icon aria-hidden={true} svgPath={mdiAlertOctagram} className="text-danger" />
                                ) : null}
                                {autoUpgradeEnabled ? 'Enabled' : 'Disabled'}
                            </Label>
                        </div>
                    </div>
                    <div>
                        {data?.site.upgradeReadiness.schemaDrift.length > 0 ? (
                            <span>
                                <Icon aria-hidden={true} svgPath={mdiAlertOctagram} className="text-danger" /> Schema
                                drift is detected. Please resolve schema drift before attempting an upgrade.
                                <br />
                                <br /> Learn more about the migrator{' '}
                                <Link to="/help/admin/updates/migrator/migrator-operations">upgrade command</Link>.
                            </span>
                        ) : data?.site.upgradeReadiness.requiredOutOfBandMigrations.length > 0 ? (
                            <span>
                                Some oob migrations must complete before a multi version upgrade can finish. Learn more
                                at the <Link to="/site-admin/migrations?filters=pending">migrations</Link> page, and
                                reach out to{' '}
                                <Link to="mailto:support@sourcegraph.com" target="_blank" rel="noopener noreferrer">
                                    Sourcegraph support
                                </Link>{' '}
                                for clarifications.
                                <br />
                                <br /> Learn more about the migrator{' '}
                                <Link to="/help/admin/updates/migrator/migrator-operations">upgrade command</Link>.
                            </span>
                        ) : (
                            <span>
                                This instance is prepared for a multiversion upgrade. If automatic upgrades are enabled
                                the migrator upgrade command will now infer to and from versions.
                                <br />
                                <br />
                                Learn more about the migrator{' '}
                                <Link to="/help/admin/updates/migrator/migrator-operations">upgrade command</Link>.
                            </span>
                        )}
                    </div>
                    <hr className="my-3" />
                    <div className="d-flex flex-row justify-content-between">
                        <H3>Schema drift</H3>

                        <div>
                            {data.site.upgradeReadiness.schemaDrift.length > 0 && (
                                <Button
                                    onClick={() => exportDrift()}
                                    variant="secondary"
                                    size="sm"
                                    aria-label="export schema drift"
                                    className="mr-2"
                                >
                                    Export
                                </Button>
                            )}
                            <Button
                                onClick={() => refetch()}
                                variant="primary"
                                size="sm"
                                aria-label="refresh drift check"
                            >
                                Refresh
                            </Button>
                        </div>
                    </div>
                    {data.site.upgradeReadiness.schemaDrift.length > 0 ? (
                        <Collapse isOpen={isExpanded} onOpenChange={setIsExpanded} openByDefault={false}>
                            <Alert className={classNames('mb-0', styles.alert)} variant="danger">
                                <span>
                                    There are schema drifts detected, please contact{' '}
                                    <Link to="mailto:support@sourcegraph.com" target="_blank" rel="noopener noreferrer">
                                        Sourcegraph support
                                    </Link>{' '}
                                    for assistance.
                                </span>
                            </Alert>
                            <CollapseHeader
                                as={Button}
                                variant="secondary"
                                outline={true}
                                className="p-0 m-0 mt-2 mb-2 border-0 w-100 font-weight-normal d-flex justify-content-between align-items-center"
                            >
                                <H4 className="m-0">View drift output</H4>
                                <Icon
                                    aria-hidden={true}
                                    svgPath={isExpanded ? mdiChevronUp : mdiChevronDown}
                                    className="mr-1"
                                    size="md"
                                />
                            </CollapseHeader>

                            <CollapsePanel>
                                {data.site.upgradeReadiness.schemaDrift.map(summary => (
                                    <div key={summary.name} className={styles.container}>
                                        <div className={styles.tableContainer}>
                                            <div className={styles.table}>
                                                <div className={styles.label}>Problem:</div>
                                                <div>{summary.problem}</div>
                                            </div>
                                            <div className={styles.table}>
                                                <div className={styles.label}>Solution:</div>
                                                <div>{capitalize(summary.solution)}</div>
                                            </div>
                                            <div className={styles.table}>
                                                <div className={styles.label}>Hint:</div>
                                                <div>
                                                    {summary.urlHint ? (
                                                        <Link to={summary.urlHint}>
                                                            See Sourcegraph query for potential fix
                                                        </Link>
                                                    ) : (
                                                        'Not Applicable'
                                                    )}
                                                </div>
                                            </div>
                                        </div>
                                        <div className={styles.outputContainer}>
                                            <div className={styles.infoContainer}>
                                                <div className={styles.label}>Current Delta:</div>
                                                <div>
                                                    <LogOutput
                                                        text={summary.diff ? summary.diff : 'None'}
                                                        logDescription="The object diff"
                                                    />
                                                </div>
                                            </div>

                                            <div className={styles.infoContainer}>
                                                <div className="d-flex flex-row justify-content-between">
                                                    <div className={styles.label}>Suggested statements to repair:</div>
                                                    <Button
                                                        onClick={async () => {
                                                            if (summary.statements) {
                                                                await navigator.clipboard.writeText(
                                                                    summary.statements.join('\n')
                                                                )
                                                            }

                                                            return null
                                                        }}
                                                        variant="primary"
                                                        size="sm"
                                                        aria-label="copy sql statements to repair"
                                                        className="mb-1"
                                                    >
                                                        <Icon aria-hidden={true} svgPath={mdiContentCopy} />
                                                    </Button>
                                                </div>
                                                <div>
                                                    <LogOutput
                                                        text={
                                                            summary.statements ? summary.statements.join('\n') : 'None'
                                                        }
                                                        logDescription="SQL statements to repair"
                                                    />
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </CollapsePanel>
                        </Collapse>
                    ) : (
                        <Text>
                            <Alert className={classNames('mb-0', styles.alert)} variant="success">
                                There is no schema drift detected.
                            </Alert>
                        </Text>
                    )}
                    <hr className="my-3" />
                    <H3>Required out-of-band migrations</H3>
                    {data.site.upgradeReadiness.requiredOutOfBandMigrations.length > 0 ? (
                        <>
                            <span>
                                <Alert className={classNames('mb-0', styles.alert)} variant="warning">
                                    There are pending out-of-band migrations that need to complete, please go to{' '}
                                    <Link to="/site-admin/migrations?filters=pending">migrations</Link> to check
                                    details.
                                </Alert>
                            </span>
                            <ul className="mt-2 pl-3">
                                {data.site.upgradeReadiness.requiredOutOfBandMigrations.map(oobm => (
                                    <li key={oobm.id}>{oobm.description}</li>
                                ))}
                            </ul>
                        </>
                    ) : (
                        <Text>
                            <Alert className={classNames('mb-0', styles.alert)} variant="success">
                                There are no pending out-of-band migrations that need to complete.
                            </Alert>
                        </Text>
                    )}
                    <hr className="my-3" />
                </>
            )}
        </>
    )
}

/**
 * A page displaying information about available updates for the Sourcegraph instance. As well as the readiness status of the instance for upgrade.
 */
export const SiteAdminUpdatesPage: React.FC<Props> = ({ telemetryService }) => {
    useMemo(() => {
        telemetryService.logViewEvent('SiteAdminUpdates')
    }, [telemetryService])

    return (
        <div>
            <PageTitle title="Updates - Admin" />

            <PageHeader path={[{ text: 'Updates' }]} headingElement="h2" className="mb-3" />
            <Container className="mb-3">
                <SiteUpdateCheck />
            </Container>

            <PageHeader path={[{ text: 'Upgrade Readiness' }]} headingElement="h2" className="mb-3" />
            <Container className="mb-3">
                <SiteUpgradeReadiness />
            </Container>
        </div>
    )
}
