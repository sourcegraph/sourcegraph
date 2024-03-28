import { useEffect, useMemo } from 'react'

import { mdiChevronRight, mdiCircleOffOutline } from '@mdi/js'

import { useQuery } from '@sourcegraph/http-client'
import { RepoLink } from '@sourcegraph/shared/src/components/RepoLink'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Badge, Container, ErrorAlert, H3, Icon, Link, LoadingSpinner, PageHeader, Text } from '@sourcegraph/wildcard'

import type {
    DashboardRepoFields,
    GlobalCodeIntelStatusResult,
    CodeIntelIndexerFields,
} from '../../../../graphql-operations'
import { ExternalRepositoryIcon } from '../../../../site-admin/components/ExternalRepositoryIcon'
import { globalCodeIntelStatusQuery } from '../backend'
import { DataSummary, type DataSummaryItem } from '../components/DataSummary'

import { buildParamsFromFilterState } from './RepoDashboardPage'

import styles from './GlobalDashboardPage.module.scss'

type DashboardNodeProps =
    | {
          type: 'error'
          repository: DashboardRepoFields
          errorCount: number
      }
    | {
          type: 'configurable'
          repository: DashboardRepoFields
          indexers: { indexer: CodeIntelIndexerFields | null }[]
      }

const DashboardNode: React.FunctionComponent<DashboardNodeProps> = props => {
    const repoLink =
        props.type === 'error'
            ? `${props.repository.url}/-/code-graph/dashboard?${buildParamsFromFilterState({
                  show: 'all',
                  language: 'all',
                  indexState: 'error',
              }).toString()}`
            : `${props.repository.url}/-/code-graph/dashboard?${buildParamsFromFilterState({
                  show: 'suggestions',
                  language: 'all',
              }).toString()}`

    return (
        <li key={props.repository.name} className={styles.detailsListItem}>
            <div>
                {props.repository.externalRepository && (
                    <ExternalRepositoryIcon
                        externalRepo={{
                            serviceType: props.repository.externalRepository.serviceType,
                        }}
                    />
                )}
                <RepoLink repoName={props.repository.name} to={repoLink} />
            </div>
            <Link to={repoLink} className={styles.detailsLink}>
                {props.type === 'error' ? (
                    <>
                        <Badge variant="danger" className={styles.badge} pill={true}>
                            {props.errorCount}
                        </Badge>
                        <Icon svgPath={mdiChevronRight} size="md" aria-label="Fix" />
                    </>
                ) : (
                    <>
                        <Badge variant="primary" className={styles.badge} pill={true}>
                            {props.indexers.length}
                        </Badge>
                        <Icon svgPath={mdiChevronRight} size="md" aria-label="Configure" />
                    </>
                )}
            </Link>
        </li>
    )
}

export interface GlobalDashboardPageProps extends TelemetryProps, TelemetryV2Props {
    indexingEnabled?: boolean
}

export const GlobalDashboardPage: React.FunctionComponent<GlobalDashboardPageProps> = ({
    telemetryService,
    indexingEnabled = window.context?.codeIntelAutoIndexingEnabled,
    telemetryRecorder,
}) => {
    useEffect(() => {
        telemetryService.logPageView('CodeIntelGlobalDashboard')
        telemetryRecorder.recordEvent('admin.codeIntel.dashboard', 'view')
    }, [telemetryService, telemetryRecorder])

    const { data, error, loading } = useQuery<GlobalCodeIntelStatusResult>(globalCodeIntelStatusQuery, {
        notifyOnNetworkStatusChange: false,
        fetchPolicy: 'no-cache',
    })

    const summaryItems = useMemo((): DataSummaryItem[] => {
        if (!data) {
            return []
        }

        const countWithPreciseCodeIntel = data.codeIntelSummary.numRepositoriesWithCodeIntelligence
        const countWithErrors = data.codeIntelSummary.repositoriesWithErrors?.nodes.length || 0
        const countConfigurable = data.codeIntelSummary.repositoriesWithConfiguration?.nodes.length || 0

        return [
            {
                label: `${
                    countWithPreciseCodeIntel === 1 ? 'Repository' : 'Repositories'
                } with precise code intelligence`,
                value: <>{countWithPreciseCodeIntel}</>,
                className: styles.summaryItemExtended,
                valueClassName: 'text-success',
            },
            {
                label: `${countWithErrors === 1 ? 'Repository' : 'Repositories'} with errors`,
                value: <>{countWithErrors}</>,
                valueClassName: 'text-danger',
            },
            ...(indexingEnabled
                ? [
                      {
                          label: `Configurable ${countConfigurable === 1 ? 'repository' : 'repositories'}`,
                          value: <>{countConfigurable}</>,
                          valueClassName: 'text-primary',
                      },
                  ]
                : [
                      {
                          label: 'Auto-indexing is disabled',
                          value: (
                              <Icon size="sm" aria-label="Auto-indexing is disabled" svgPath={mdiCircleOffOutline} />
                          ),
                          valueClassName: 'text-muted',
                      },
                  ]),
        ]
    }, [data, indexingEnabled])

    if (loading || !data) {
        return <LoadingSpinner />
    }

    if (error) {
        return <ErrorAlert prefix="Failed to load code intelligence summary" error={error} />
    }

    return (
        <>
            <PageHeader
                headingElement="h2"
                path={[
                    {
                        text: <>Code intelligence summary</>,
                    },
                ]}
                className="mb-3"
            />
            <Container>
                <DataSummary items={summaryItems} className="pb-3" />
            </Container>

            <Container className="mt-3">
                {data.codeIntelSummary.repositoriesWithErrors &&
                    data.codeIntelSummary.repositoriesWithErrors.nodes.length > 0 && (
                        <div className={styles.details}>
                            <H3 className="px-3">Repositories with errors</H3>

                            <Text className="px-3 text-muted">
                                The following repositories have failures on the most recent attempt to automatically
                                index or process precise code intelligence index.
                            </Text>

                            <ul className={styles.detailsList}>
                                {data.codeIntelSummary.repositoriesWithErrors.nodes.map(({ repository, count }) => (
                                    <DashboardNode
                                        type="error"
                                        repository={repository}
                                        errorCount={count}
                                        key={repository.name}
                                    />
                                ))}
                            </ul>
                        </div>
                    )}

                {indexingEnabled ? (
                    data.codeIntelSummary.repositoriesWithConfiguration &&
                    data.codeIntelSummary.repositoriesWithConfiguration.nodes.length > 0 && (
                        <div className={styles.details}>
                            <H3 className="px-3">Repositories with suggestions</H3>

                            <Text className="px-3 text-muted">
                                We have inferred auto-indexing jobs for the following repositories.
                            </Text>

                            <Text className="px-3 text-muted">
                                The repositories in this list are ordered by their <strong>searched-based</strong> code
                                navigation activity (and increasing precise coverage on these repositories will have the
                                biggest impact on current users).
                            </Text>

                            <ul className={styles.detailsList}>
                                {data.codeIntelSummary.repositoriesWithConfiguration.nodes.map(
                                    ({ repository, indexers }) => (
                                        <DashboardNode
                                            type="configurable"
                                            repository={repository}
                                            indexers={indexers}
                                            key={repository.name}
                                        />
                                    )
                                )}
                            </ul>
                        </div>
                    )
                ) : (
                    <div className="text-center p-2">
                        <Link to="/help/code_navigation/how-to/enable_auto_indexing">Enable auto-indexing</Link> to
                        automatically create and upload a precise index for your source code.
                    </div>
                )}
            </Container>
        </>
    )
}
