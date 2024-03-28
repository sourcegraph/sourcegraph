import React, { useEffect, useState } from 'react'

import { mdiMagnify, mdiPlus } from '@mdi/js'

import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SearchContextProps } from '@sourcegraph/shared/src/search'
import { PageHeader, Link, Button, Icon, Alert } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { CallToActionBanner } from '../../components/CallToActionBanner'
import { Page } from '../../components/Page'
import { eventLogger } from '../../tracking/eventLogger'

import { SearchContextsList } from './SearchContextsList'

import styles from './SearchContextsListPage.module.scss'

export interface SearchContextsListPageProps
    extends Pick<SearchContextProps, 'fetchSearchContexts' | 'getUserSearchContextNamespaces'>,
        PlatformContextProps<'requestGraphQL' | 'telemetryRecorder'> {
    isSourcegraphDotCom: boolean
    authenticatedUser: AuthenticatedUser | null
}

export const SearchContextsListPage: React.FunctionComponent<SearchContextsListPageProps> = ({
    authenticatedUser,
    getUserSearchContextNamespaces,
    fetchSearchContexts,
    platformContext,
    isSourcegraphDotCom,
}) => {
    const [alert, setAlert] = useState<string | undefined>()

    useEffect(() => {
        platformContext.telemetryRecorder.recordEvent('searchContexts.list', 'view')
    }, [platformContext.telemetryRecorder])

    return (
        <div data-testid="search-contexts-list-page" className="w-100">
            <Page>
                <PageHeader
                    actions={
                        <div className={styles.actions}>
                            <Button to="/contexts/new" variant="primary" as={Link}>
                                <Icon aria-hidden={true} svgPath={mdiPlus} />
                                Create search context
                            </Button>
                        </div>
                    }
                    description={
                        <>
                            <span className="text-muted">
                                Search code you care about with search contexts.{' '}
                                <Link
                                    to="/help/code_search/explanations/features#search-contexts"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    Learn more
                                </Link>
                            </span>
                            {isSourcegraphDotCom && (
                                <CallToActionBanner variant="filled" className="mb-0">
                                    To search across your team's private repositories,{' '}
                                    <Link
                                        to="https://sourcegraph.com"
                                        onClick={() => {
                                            eventLogger.log('ClickedOnEnterpriseCTA', { location: 'ContextsSettings' })
                                            platformContext.telemetryRecorder.recordEvent(
                                                'searchContexts.enterpriseCTA',
                                                'click'
                                            )
                                        }}
                                    >
                                        get Sourcegraph Enterprise
                                    </Link>
                                    .
                                </CallToActionBanner>
                            )}
                        </>
                    }
                    className="mb-3"
                >
                    <PageHeader.Heading as="h2" styleAs="h1">
                        <PageHeader.Breadcrumb icon={mdiMagnify} to="/search" aria-label="Code Search" />
                        <PageHeader.Breadcrumb>Contexts</PageHeader.Breadcrumb>
                    </PageHeader.Heading>
                </PageHeader>
                {alert && <Alert variant="danger">{alert}</Alert>}
                <div role="tabpanel" id="search-context-list">
                    <SearchContextsList
                        authenticatedUser={authenticatedUser}
                        getUserSearchContextNamespaces={getUserSearchContextNamespaces}
                        fetchSearchContexts={fetchSearchContexts}
                        platformContext={platformContext}
                        setAlert={setAlert}
                    />
                </div>
            </Page>
        </div>
    )
}
