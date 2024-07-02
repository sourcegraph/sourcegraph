import React, { useCallback, useEffect, useMemo } from 'react'

import { mdiAlert, mdiAlertCircle, mdiArrowLeftBold, mdiArrowRightBold } from '@mdi/js'
import classNames from 'classnames'
import { of, timer, type Observable } from 'rxjs'
import { catchError, concatMap, map, repeat, takeWhile } from 'rxjs/operators'
import { parse as _parseVersion, type SemVer } from 'semver'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { asError, isErrorLike, type ErrorLike } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Alert,
    Code,
    Container,
    ErrorAlert,
    H3,
    Icon,
    LoadingSpinner,
    PageHeader,
    Text,
    Tooltip,
    useObservable,
} from '@sourcegraph/wildcard'

import { Collapsible } from '../components/Collapsible'
import { FilteredConnection, type Connection, type Filter } from '../components/FilteredConnection'
import { PageTitle } from '../components/PageTitle'
import type { OutOfBandMigrationFields } from '../graphql-operations'

import {
    fetchOutOfBandMigrations as defaultFetchMigrations,
    fetchSiteUpdateCheck as defaultFetchSiteUpdateCheck,
} from './backend'

import styles from './SiteAdminMigrationsPage.module.scss'

export interface SiteAdminMigrationsPageProps extends TelemetryProps, TelemetryV2Props {
    fetchMigrations?: typeof defaultFetchMigrations
    fetchSiteUpdateCheck?: () => Observable<{ productVersion: string }>
    now?: () => Date
}

const filters: Filter[] = [
    {
        id: 'filters',
        label: 'Migration state',
        type: 'select',
        options: [
            {
                label: 'All',
                value: 'all',
                tooltip: 'Show all migrations',
                args: {},
            },
            {
                label: 'Pending',
                value: 'pending',
                tooltip: 'Show pending migrations',
                args: { completed: false },
            },
            {
                label: 'Completed',
                value: 'completed',
                tooltip: 'Show completed migrations',
                args: { completed: true },
            },
        ],
    },
]

/* How frequently to refresh data from the API. */
const REFRESH_INTERVAL_MS = 5000

/* How many (minor) versions we can upgrade at once. */
const UPGRADE_RANGE = 1

/* How many (minor) versions we can downgrade at once. */
const DOWNGRADE_RANGE = 1

export const SiteAdminMigrationsPage: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminMigrationsPageProps>
> = ({
    fetchMigrations = defaultFetchMigrations,
    fetchSiteUpdateCheck = defaultFetchSiteUpdateCheck,
    now,
    telemetryRecorder,
}) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('admin.migrations', 'view')
    }, [telemetryRecorder])

    const migrationsOrError = useObservable(
        useMemo(
            () =>
                timer(0, REFRESH_INTERVAL_MS, undefined).pipe(
                    concatMap(() =>
                        // Exclude ExcludeDeprecatedBeforeFirstVersion: true
                        fetchMigrations(true).pipe(
                            catchError((error): [ErrorLike] => [asError(error)]),
                            repeat({ delay: REFRESH_INTERVAL_MS })
                        )
                    ),
                    takeWhile(() => true, true)
                ),
            [fetchMigrations]
        )
    )

    const queryMigrations = useCallback(
        ({
            query,
            completed,
        }: {
            query?: string
            completed?: boolean
        }): Observable<Connection<OutOfBandMigrationFields>> => {
            if (isErrorLike(migrationsOrError) || migrationsOrError === undefined) {
                return of({ nodes: [] })
            }

            return of({
                nodes: migrationsOrError.filter(
                    migration =>
                        (completed === undefined || completed === isComplete(migration)) &&
                        (!query || matchesQuery(migration, query))
                ),
                totalCount: migrationsOrError.length,
                pageInfo: { hasNextPage: false },
            })
        },
        [migrationsOrError]
    )

    return (
        <div className="site-admin-migrations-page">
            {isErrorLike(migrationsOrError) ? (
                <ErrorAlert prefix="Error loading out of band migrations" error={migrationsOrError} />
            ) : migrationsOrError === undefined ? (
                <LoadingSpinner />
            ) : (
                <>
                    <PageTitle title="Out of band migrations - Admin" />
                    <PageHeader
                        path={[{ text: 'Out-of-band migrations' }]}
                        headingElement="h2"
                        description={
                            <>
                                Out-of-band migrations run in the background of the Sourcegraph instance convert data
                                from an old format into a new format. Consult this page prior to upgrading your
                                Sourcegraph instance to ensure that all expected migrations have completed.
                            </>
                        }
                        className="mb-3"
                    />

                    <MigrationBanners migrations={migrationsOrError} fetchSiteUpdateCheck={fetchSiteUpdateCheck} />

                    <Container className="mb-3">
                        <FilteredConnection<OutOfBandMigrationFields, Omit<MigrationNodeProps, 'node'>>
                            className="mb-0"
                            listComponent="div"
                            listClassName={classNames('list-group mb-3', styles.migrationsGrid)}
                            noun="migration"
                            pluralNoun="migrations"
                            queryConnection={queryMigrations}
                            nodeComponent={MigrationNode}
                            nodeComponentProps={{ now }}
                            filters={filters}
                            autoFocus={false}
                        />
                    </Container>
                </>
            )}
        </div>
    )
}

interface MigrationBannersProps {
    migrations: OutOfBandMigrationFields[]
    fetchSiteUpdateCheck?: () => Observable<{ productVersion: string }>
}

const MigrationBanners: React.FunctionComponent<React.PropsWithChildren<MigrationBannersProps>> = ({
    migrations,
    fetchSiteUpdateCheck = defaultFetchSiteUpdateCheck,
}) => {
    const productVersion = useObservable(
        useMemo(
            () => fetchSiteUpdateCheck().pipe(map(site => parseVersion(site.productVersion))),
            [fetchSiteUpdateCheck]
        )
    )
    if (!productVersion) {
        return <></>
    }

    const nextVersion =
        productVersion.major === 3 && productVersion.minor === 43
            ? parseVersion('4.0.0')
            : parseVersion(`${productVersion.major}.${productVersion.minor + UPGRADE_RANGE}.0`)
    const previousVersion =
        productVersion.major === 4 && productVersion.minor === 0
            ? parseVersion('3.43.0')
            : parseVersion(`${productVersion.major}.${productVersion.minor - DOWNGRADE_RANGE}.0`)

    const invalidMigrations = migrationsInvalidForVersion(migrations, productVersion)
    const invalidMigrationsAfterUpgrade = migrationsInvalidForVersion(migrations, nextVersion)
    const invalidMigrationsAfterDowngrade = migrationsInvalidForVersion(migrations, previousVersion)

    if (invalidMigrations.length > 0) {
        return <MigrationInvalidBanner migrations={invalidMigrations} />
    }
    return (
        <>
            {invalidMigrationsAfterUpgrade.length > 0 && (
                <MigrationUpgradeWarningBanner migrations={invalidMigrationsAfterUpgrade} />
            )}
            {invalidMigrationsAfterDowngrade.length > 0 && (
                <MigrationDowngradeWarningBanner migrations={invalidMigrationsAfterDowngrade} />
            )}
        </>
    )
}

interface MigrationInvalidBannerProps {
    migrations: OutOfBandMigrationFields[]
}

const MigrationInvalidBanner: React.FunctionComponent<React.PropsWithChildren<MigrationInvalidBannerProps>> = ({
    migrations,
}) => (
    <Alert variant="danger">
        <Text>
            <Icon className="mr-2" aria-hidden={true} svgPath={mdiAlertCircle} />
            <strong>Contact support.</strong> The following migrations are not in the expected state. You have partially
            migrated or un-migrated data in a format that is incompatible with the currently deployed version of
            Sourcegraph.{' '}
            <strong>Continuing to run your instance in this state will result in errors and possible data loss.</strong>
        </Text>

        <ul className="mb-0">
            {migrations.map(migration => (
                <li key={migration.id}>{migration.description}</li>
            ))}
        </ul>
    </Alert>
)

interface MigrationUpgradeWarningBannerProps {
    migrations: OutOfBandMigrationFields[]
}

const MigrationUpgradeWarningBanner: React.FunctionComponent<
    React.PropsWithChildren<MigrationUpgradeWarningBannerProps>
> = ({ migrations }) => (
    <Alert variant="warning">
        <Text>
            The next version of Sourcegraph removes support for reading an old data format. Your Sourcegraph instance
            must complete the following migrations to ensure your data remains readable.{' '}
            <strong>If you upgrade your Sourcegraph instance now, you may corrupt or lose data.</strong>
        </Text>
        <ul>
            {migrations.map(migration => (
                <li key={migration.id}>{migration.description}</li>
            ))}
        </ul>
        <span>Contact support if these migrations are not making progress or if there are associated errors.</span>
    </Alert>
)

interface MigrationDowngradeWarningBannerProps {
    migrations: OutOfBandMigrationFields[]
}

const MigrationDowngradeWarningBanner: React.FunctionComponent<
    React.PropsWithChildren<MigrationDowngradeWarningBannerProps>
> = ({ migrations }) => (
    <Alert variant="warning">
        <Text>
            <Icon className="mr-2" aria-hidden={true} svgPath={mdiAlert} />
            <span>
                The previous version of Sourcegraph does not support reading data that has been migrated into a new
                format. Your Sourcegraph instance must undo the following migrations to ensure your data can be read by
                the previous version.{' '}
                <strong>If you downgrade your Sourcegraph instance now, you may corrupt or lose data.</strong>
            </span>
        </Text>

        <ul>
            {migrations.map(migration => (
                <li key={migration.id}>{migration.description}</li>
            ))}
        </ul>

        <span>Contact support for assistance with downgrading your instance.</span>
    </Alert>
)

interface MigrationNodeProps {
    node: OutOfBandMigrationFields
    now?: () => Date
}

const MigrationNode: React.FunctionComponent<React.PropsWithChildren<MigrationNodeProps>> = ({ node, now }) => (
    <React.Fragment key={node.id}>
        <span className={styles.separator} />

        <div className={classNames('d-flex flex-column', styles.information)}>
            <div>
                <H3>{node.description}</H3>

                <Text className="m-0">
                    <span className="text-muted">Team</span> <strong>{node.team}</strong>{' '}
                    <span className="text-muted">is migrating data in</span> <strong>{node.component}</strong>
                    <span className="text-muted">.</span>
                </Text>

                <Text className="m-0">
                    <span className="text-muted">Began running in v</span>
                    {node.introduced}
                    {node.deprecated && (
                        <>
                            {' '}
                            <span className="text-muted">and will cease running in v</span>
                            {node.deprecated}
                        </>
                    )}
                    .
                </Text>
            </div>
        </div>

        <span className={classNames('d-none d-md-inline', styles.progress)}>
            <div className="m-0 text-nowrap d-flex flex-column align-items-center justify-content-center">
                <div>
                    {node.applyReverse ? (
                        <Icon className="mr-1 text-danger" aria-hidden={true} svgPath={mdiArrowLeftBold} />
                    ) : (
                        <Icon className="mr-1" aria-hidden={true} svgPath={mdiArrowRightBold} />
                    )}
                    {Math.floor(node.progress * 100)}%
                </div>

                <Tooltip content={`${Math.floor(node.progress * 100)}%`} placement="bottom">
                    <div>
                        <meter
                            min={0}
                            low={0.2}
                            high={0.8}
                            max={1}
                            optimum={1}
                            value={node.progress}
                            aria-label="migration progress"
                        />
                    </div>
                </Tooltip>

                {node.lastUpdated && node.lastUpdated !== '' && (
                    <>
                        <div className="text-center">
                            <span className="text-muted">Last updated</span>
                        </div>
                        <div className="text-center">
                            <small>
                                <Timestamp date={node.lastUpdated} now={now} noAbout={true} />
                            </small>
                        </div>
                    </>
                )}
            </div>
        </span>

        {node.errors.length > 0 && (
            <Collapsible
                title={<strong>Recent errors ({node.errors.length})</strong>}
                className="p-0 font-weight-normal"
                buttonClassName="mb-0"
                titleAtStart={true}
                defaultExpanded={false}
            >
                <div className={classNames('pt-2', styles.nodeGrid)}>
                    {node.errors
                        .map((error, index) => ({ ...error, index }))
                        .map(error => (
                            <React.Fragment key={error.index}>
                                <div className="py-1 pr-2">
                                    <Timestamp date={error.created} now={now} />
                                </div>

                                <span className={classNames('py-1 pl-2', styles.nodeGridCode)}>
                                    <Code>{error.message}</Code>
                                </span>
                            </React.Fragment>
                        ))}
                </div>
            </Collapsible>
        )}
    </React.Fragment>
)

type PartialVersion = SemVer | null

/** Parse the given version safely. */
const parseVersion = (version: string): PartialVersion => {
    try {
        return _parseVersion(version)
    } catch {
        return null
    }
}

/** Returns true if the given migration state is invalid for the given version. */
export const isInvalidForVersion = (migration: OutOfBandMigrationFields, version: PartialVersion): boolean => {
    if (!version) {
        return false
    }

    // Migrations only store major/minor version components
    const introduced = parseVersion(`${migration.introduced}.0`)
    if (introduced && version.major === introduced.major && version.minor < introduced.minor) {
        return migration.progress !== 0 && !migration.nonDestructive
    }

    if (migration.deprecated) {
        // Migrations only store major/minor version components
        const deprecated = parseVersion(`${migration.deprecated}.0`)
        if (deprecated && version.major === deprecated.major && version.minor >= deprecated.minor) {
            return migration.progress !== 1
        }

        if (deprecated && version.major > deprecated.major) {
            return migration.progress !== 1
        }
    }

    return false
}

/** Returns the set of migrations that are invalid for the given version. */
const migrationsInvalidForVersion = (
    migrations: OutOfBandMigrationFields[],
    version: PartialVersion
): OutOfBandMigrationFields[] => migrations.filter(migration => isInvalidForVersion(migration, version))

/** Returns true if the given migration is has completed (100% if forward, 0% if reverse). */
export const isComplete = (migration: OutOfBandMigrationFields): boolean =>
    (migration.progress === 0 && migration.applyReverse) || (migration.progress === 1 && !migration.applyReverse)

/** Returns the searchable text from a migration. */
const searchFields = (migration: OutOfBandMigrationFields): string[] => [
    migration.team,
    migration.component,
    migration.description,
    migration.introduced,
    migration.deprecated || '',
    ...migration.errors.map(error => error.message),
]

/** Returns true if the migration matches the given query. */
const matchesQuery = (migration: OutOfBandMigrationFields, query: string): boolean => {
    const fields = searchFields(migration)
        .map(value => value.toLowerCase())
        .filter(value => value !== '')

    return query
        .toLowerCase()
        .split(' ')
        .filter(query => query !== '')
        .every(query => fields.some(value => value.includes(query)))
}
