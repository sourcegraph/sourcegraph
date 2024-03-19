import { type FC, useEffect, useMemo, useState } from 'react'

import { mdiCog, mdiDelete, mdiOpenInNew, mdiPlus } from '@mdi/js'
import classNames from 'classnames'
import { useNavigate, useParams } from 'react-router-dom'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import type { ErrorLike } from '@sourcegraph/common'
import { useQuery } from '@sourcegraph/http-client'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Container,
    ErrorAlert,
    PageHeader,
    ButtonLink,
    Icon,
    LoadingSpinner,
    Button,
    H2,
    H3,
    Link,
    Text,
    Grid,
    AnchorLink,
} from '@sourcegraph/wildcard'
// eslint-disable-next-line no-restricted-imports
import type { BreadcrumbItem } from '@sourcegraph/wildcard/src/components/PageHeader'

import { GitHubAppDomain, type GitHubAppByIDResult, type GitHubAppByIDVariables } from '../../graphql-operations'
import { ExternalServiceNode } from '../externalServices/ExternalServiceNode'
import { ConnectionList, SummaryContainer, ConnectionSummary } from '../FilteredConnection/ui'
import { PageTitle } from '../PageTitle'

import { AppLogo } from './AppLogo'
import { AuthProviderMessage } from './AuthProviderMessage'
import { GITHUB_APP_BY_ID_QUERY } from './backend'
import { RemoveGitHubAppModal } from './RemoveGitHubAppModal'

import styles from './GitHubAppCard.module.scss'

interface Props extends TelemetryProps, TelemetryV2Props {
    /**
     * The parent breadcrumb item to show for this page in the header.
     */
    headerParentBreadcrumb: BreadcrumbItem
    /** An optional annotation to show in the page header. */
    headerAnnotation?: React.ReactNode
}

export const GitHubAppPage: FC<Props> = ({
    telemetryService,
    telemetryRecorder,
    headerParentBreadcrumb,
    headerAnnotation,
}) => {
    const { appID } = useParams()
    const navigate = useNavigate()
    const [removeModalOpen, setRemoveModalOpen] = useState<boolean>(false)

    useEffect(() => {
        telemetryService.logPageView('SiteAdminGitHubApp')
        telemetryRecorder.recordEvent('admin.GitHubApp', 'view')
    }, [telemetryService, telemetryRecorder])
    const [fetchError, setError] = useState<ErrorLike>()

    const { data, loading, error } = useQuery<GitHubAppByIDResult, GitHubAppByIDVariables>(GITHUB_APP_BY_ID_QUERY, {
        variables: { id: appID ?? '' },
    })

    const app = useMemo(() => data?.gitHubApp, [data])

    if (!appID) {
        return null
    }

    const handleError = (error: ErrorLike): [] => {
        setError(error)
        return []
    }

    const onAddInstallation = async (app: NonNullable<GitHubAppByIDResult['gitHubApp']>): Promise<void> => {
        try {
            const req = await fetch(`/githubapp/state?id=${app?.id}&domain=${app?.domain}`)
            const state = await req.text()
            const trailingSlash = app.appURL.endsWith('/') ? '' : '/'
            window.location.assign(`${app.appURL}${trailingSlash}installations/new?state=${state}`)
        } catch (error) {
            handleError(error)
        }
    }

    return (
        <div>
            {app ? <PageTitle title={`GitHub App - ${app.name}`} /> : <PageTitle title="GitHub App" />}
            {(error || fetchError) && <ErrorAlert className="mb-3" error={error ?? fetchError} />}
            {loading && !app && <LoadingSpinner />}
            {app && (
                <>
                    {removeModalOpen && (
                        <RemoveGitHubAppModal
                            onCancel={() => setRemoveModalOpen(false)}
                            afterDelete={() => navigate('/site-admin/github-apps')}
                            app={app}
                        />
                    )}
                    <PageHeader
                        path={[
                            { icon: mdiCog },
                            headerParentBreadcrumb,
                            {
                                text: (
                                    <span className="d-flex align-items-center">
                                        <AppLogo
                                            src={app.logo}
                                            name={app.name}
                                            className={classNames(styles.logo, 'mr-2')}
                                        />
                                        <span>{app.name}</span>
                                    </span>
                                ),
                            },
                        ]}
                        annotation={headerAnnotation}
                        className="mb-3"
                        headingElement="h2"
                    />
                    <div className="d-flex align-items-sm-center flex-sm-row flex-column">
                        <span className="timestamps text-muted mb-2">
                            Created <Timestamp date={app.createdAt} /> | Updated <Timestamp date={app.updatedAt} />
                        </span>
                        <span className="ml-sm-auto">
                            <AnchorLink to={app.appURL} target="_blank">
                                View In GitHub <Icon inline={true} svgPath={mdiOpenInNew} aria-hidden={true} />
                            </AnchorLink>
                            <Button onClick={() => navigate(-1)} variant="secondary" className="mx-2">
                                Cancel
                            </Button>
                            <Button
                                className="text-nowrap"
                                aria-label="Remove GitHub App"
                                onClick={() => setRemoveModalOpen(true)}
                                variant="danger"
                            >
                                <Icon aria-hidden={true} svgPath={mdiDelete} /> Delete
                            </Button>
                        </span>
                    </div>
                </>
            )}
            {app && (
                <Container className="my-3">
                    <Grid columnCount={2} templateColumns="auto 1fr" spacing={[0.6, 2]}>
                        <span className="font-weight-bold">GitHub App Name</span>
                        <span>{app.name}</span>
                        <span className="font-weight-bold">URL</span>
                        <AnchorLink to={app.appURL} target="_blank" className="text-decoration-none text-break">
                            {app.appURL}
                        </AnchorLink>
                        <span className="font-weight-bold">AppID</span>
                        <span>{app.appID}</span>
                    </Grid>
                    {/* Auth provider is only relevant to repos domain GitHub Apps */}
                    {app.domain === GitHubAppDomain.REPOS && <AuthProviderMessage app={app} id={appID} />}

                    <hr className="mt-4 mb-4" />

                    <div>
                        <H2 className="d-flex flex-sm-row flex-column align-items-sm-center mb-3">
                            Installations
                            <Button
                                className="ml-sm-auto mr-sm-0 mr-auto mt-sm-0 mt-2"
                                onClick={() => onAddInstallation(app)}
                                variant="primary"
                            >
                                <Icon svgPath={mdiPlus} aria-hidden={true} /> Add installation
                            </Button>
                        </H2>
                        <Text>
                            An installation is a connection between a GitHub App and a user or organization on GitHub.
                            An installation allows the GitHub App to access resources owned by that account and perform
                            actions on behalf of it.
                        </Text>
                        <Text>
                            A GitHub App can only be installed in multiple accounts if it is{' '}
                            <AnchorLink to="https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/making-a-github-app-public-or-private">
                                public
                            </AnchorLink>
                            . A private GitHub App can only be installed on the account that originally created it.{' '}
                            <Link
                                to="/help/admin/external_service/github#multiple-installations"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                Learn more about public vs. private GitHub Apps.
                            </Link>
                        </Text>
                        <Text>
                            Only{' '}
                            <Link
                                to="https://docs.github.com/en/organizations/managing-peoples-access-to-your-organization-with-roles/roles-in-an-organization#organization-owners"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                organization owners
                            </Link>{' '}
                            can view and manage installations for an organization.
                        </Text>
                        <div className="list-group mb-3" aria-label="GitHub App Installations">
                            {app.installations?.length === 0 ? (
                                <Text>
                                    This GitHub App does not have any installations. Install the App to create a new
                                    connection.
                                </Text>
                            ) : (
                                app.installations?.map(installation => (
                                    <Container className={classNames(styles.installation, 'p-3')} key={installation.id}>
                                        <div className="d-flex align-items-center">
                                            <Link to={installation.account.url} className="d-flex align-items-center">
                                                <AppLogo
                                                    src={installation.account.avatarURL}
                                                    name={installation.account.login}
                                                    alt="Account avatar"
                                                    className={styles.logo}
                                                />
                                                <div className="d-flex flex-column ml-3">
                                                    {installation.account.login}
                                                    <span className="text-muted">
                                                        ID: {installation.id} | Type: {installation.account.type}
                                                    </span>
                                                </div>
                                            </Link>
                                            <AnchorLink to={installation.url} target="_blank" className="ml-auto">
                                                <small>
                                                    View In GitHub{' '}
                                                    <Icon inline={true} svgPath={mdiOpenInNew} aria-hidden={true} />
                                                </small>
                                            </AnchorLink>
                                        </div>
                                        {/* Code host connections are only relevant to repos domain GitHub Apps */}
                                        {app.domain === GitHubAppDomain.REPOS && (
                                            <div className="mt-4">
                                                <H3 className="d-flex align-items-center mb-0">
                                                    Code host connections
                                                    <ButtonLink
                                                        variant="primary"
                                                        className="ml-auto"
                                                        to={`/site-admin/external-services/new?id=ghapp&appID=${
                                                            app.appID
                                                        }&installationID=${installation.id}&url=${encodeURI(
                                                            app.baseURL
                                                        )}`}
                                                        size="sm"
                                                    >
                                                        <Icon svgPath={mdiPlus} aria-hidden={true} /> Add connection
                                                    </ButtonLink>
                                                </H3>
                                                {installation.externalServices?.nodes?.length > 0 ? (
                                                    <>
                                                        <ConnectionList
                                                            as="ul"
                                                            className={styles.listGroup}
                                                            aria-label="Code Host Connections"
                                                        >
                                                            {installation.externalServices?.nodes?.map(node => (
                                                                <ExternalServiceNode
                                                                    key={node.id}
                                                                    node={node}
                                                                    editingDisabled={false}
                                                                />
                                                            ))}
                                                        </ConnectionList>
                                                        {installation.externalServices && (
                                                            <SummaryContainer className="mt-2" centered={true}>
                                                                <ConnectionSummary
                                                                    noSummaryIfAllNodesVisible={false}
                                                                    first={100}
                                                                    centered={true}
                                                                    connection={installation.externalServices}
                                                                    noun="code host connection"
                                                                    pluralNoun="code host connections"
                                                                    hasNextPage={false}
                                                                />
                                                            </SummaryContainer>
                                                        )}
                                                    </>
                                                ) : (
                                                    <Text className="text-center mt-4">
                                                        You haven't added any code host connections yet.
                                                    </Text>
                                                )}
                                            </div>
                                        )}
                                    </Container>
                                ))
                            )}
                            <SummaryContainer className="mt-3" centered={true}>
                                <ConnectionSummary
                                    noSummaryIfAllNodesVisible={false}
                                    first={app?.installations?.length ?? 0}
                                    centered={true}
                                    connection={{
                                        nodes: app?.installations ?? [],
                                        totalCount: app?.installations?.length ?? 0,
                                    }}
                                    noun="installation"
                                    pluralNoun="installations"
                                    hasNextPage={false}
                                />
                            </SummaryContainer>
                        </div>
                    </div>
                </Container>
            )}
        </div>
    )
}
