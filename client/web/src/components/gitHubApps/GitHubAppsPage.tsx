import { useEffect, useMemo } from 'react'

import { mdiPlus } from '@mdi/js'
import classNames from 'classnames'
import { useLocation } from 'react-router-dom'

import { useQuery } from '@sourcegraph/http-client'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { ButtonLink, Container, ErrorAlert, Icon, Link, LoadingSpinner, PageHeader } from '@sourcegraph/wildcard'

import { type GitHubAppsResult, type GitHubAppsVariables, GitHubAppDomain } from '../../graphql-operations'
import { eventLogger } from '../../tracking/eventLogger'
import {
    ConnectionContainer,
    ConnectionLoading,
    ConnectionList,
    ConnectionSummary,
    SummaryContainer,
} from '../FilteredConnection/ui'
import { PageTitle } from '../PageTitle'

import { GITHUB_APPS_QUERY } from './backend'
import { GitHubAppCard } from './GitHubAppCard'
import { GitHubAppFailureAlert } from './GitHubAppFailureAlert'

import styles from './GitHubAppsPage.module.scss'

interface Props extends TelemetryV2Props {
    batchChangesEnabled: boolean
}

export const GitHubAppsPage: React.FC<Props> = ({ batchChangesEnabled, telemetryRecorder }) => {
    const { data, loading, error, refetch } = useQuery<GitHubAppsResult, GitHubAppsVariables>(GITHUB_APPS_QUERY, {
        variables: {
            domain: GitHubAppDomain.REPOS,
        },
    })
    const gitHubApps = useMemo(() => data?.gitHubApps?.nodes ?? [], [data])

    useEffect(() => {
        eventLogger.logPageView('SiteAdminGitHubApps')
        telemetryRecorder.recordEvent('admin.GitHubApps', 'view')
    }, [telemetryRecorder])

    const location = useLocation()
    const success = new URLSearchParams(location.search).get('success') === 'true'
    const setupError = new URLSearchParams(location.search).get('error')

    const reloadApps = async (): Promise<void> => {
        await refetch({})
    }

    if (loading && !data) {
        return <LoadingSpinner />
    }

    return (
        <>
            <PageTitle title="GitHub Apps" />
            <PageHeader
                headingElement="h2"
                path={[{ text: 'GitHub Apps' }]}
                className={classNames(styles.pageHeader, 'mb-3')}
                description={
                    <>
                        Create and connect a GitHub App to better manage GitHub code host connections.{' '}
                        <Link to="/help/admin/external_service/github#using-a-github-app" target="_blank">
                            See how GitHub App configuration works.
                        </Link>
                        {batchChangesEnabled && (
                            <>
                                {' '}
                                To create a GitHub App to sign Batch Changes commits, visit{' '}
                                <Link to="/site-admin/batch-changes">Batch Changes settings</Link>.
                            </>
                        )}
                    </>
                }
                actions={
                    <ButtonLink
                        to="/site-admin/github-apps/new"
                        className="ml-auto text-nowrap"
                        variant="primary"
                        as={Link}
                    >
                        <Icon aria-hidden={true} svgPath={mdiPlus} /> Create GitHub App
                    </ButtonLink>
                }
            />
            <Container className="mb-3">
                {!success && setupError && <GitHubAppFailureAlert error={setupError} />}
                <ConnectionContainer>
                    {error && <ErrorAlert error={error} />}
                    {loading && !data && <ConnectionLoading />}
                    <ConnectionList as="ul" className="list-group" aria-label="GitHub Apps">
                        {gitHubApps?.map(app => (
                            <GitHubAppCard key={app.id} app={app} refetch={reloadApps} />
                        ))}
                    </ConnectionList>
                    <SummaryContainer className="mt-2" centered={true}>
                        <ConnectionSummary
                            emptyElement={
                                <div className="text-center text-muted">You haven't created any GitHub Apps yet.</div>
                            }
                            noSummaryIfAllNodesVisible={false}
                            first={gitHubApps?.length ?? 0}
                            centered={true}
                            connection={{
                                nodes: gitHubApps ?? [],
                                totalCount: gitHubApps?.length ?? 0,
                            }}
                            noun="GitHub App"
                            pluralNoun="GitHub Apps"
                            hasNextPage={false}
                        />
                    </SummaryContainer>
                </ConnectionContainer>
            </Container>
        </>
    )
}
