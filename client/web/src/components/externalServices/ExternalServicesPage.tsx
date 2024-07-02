import { useEffect, type FC } from 'react'

import { mdiPlus } from '@mdi/js'
import { Navigate, useLocation } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ButtonLink, Container, Icon, Link, PageHeader } from '@sourcegraph/wildcard'

import {
    ConnectionContainer,
    ConnectionError,
    ConnectionList,
    ConnectionLoading,
    ConnectionSummary,
    ShowMoreButton,
    SummaryContainer,
} from '../FilteredConnection/ui'
import { PageTitle } from '../PageTitle'

import { useExternalServicesConnection } from './backend'
import { ExternalServiceEditingDisabledAlert } from './ExternalServiceEditingDisabledAlert'
import { ExternalServiceEditingTemporaryAlert } from './ExternalServiceEditingTemporaryAlert'
import { ExternalServiceNode } from './ExternalServiceNode'

interface Props extends TelemetryProps, TelemetryV2Props {
    externalServicesFromFile: boolean
    allowEditExternalServicesWithFile: boolean
}

/**
 * A page displaying the external services on this site.
 */
export const ExternalServicesPage: FC<Props> = ({
    telemetryService,
    telemetryRecorder,
    externalServicesFromFile,
    allowEditExternalServicesWithFile,
}) => {
    useEffect(() => {
        telemetryService.logViewEvent('SiteAdminExternalServices')
        telemetryRecorder.recordEvent('admin.codeHostConnections', 'view')
    }, [telemetryService, telemetryRecorder])

    const location = useLocation()
    const searchParameters = new URLSearchParams(location.search)
    const repoID = searchParameters.get('repoID') || null

    const { loading, hasNextPage, fetchMore, connection, error } = useExternalServicesConnection({
        first: null,
        after: null,
        repo: repoID,
    })

    const editingDisabled = externalServicesFromFile && !allowEditExternalServicesWithFile

    return !loading && (connection?.nodes?.length ?? 0) === 0 ? (
        <Navigate to="/site-admin/external-services/new" replace={true} />
    ) : (
        <div className="site-admin-external-services-page">
            <PageTitle title="Code host connections" />
            <PageHeader
                path={[{ text: 'Code host connections' }]}
                description="Code host connections to sync repositories."
                headingElement="h2"
                actions={
                    <>
                        <ButtonLink
                            className="test-goto-add-external-service-page"
                            to="/site-admin/external-services/new"
                            variant="primary"
                            as={Link}
                            disabled={editingDisabled}
                        >
                            <Icon aria-hidden={true} svgPath={mdiPlus} /> Add connection
                        </ButtonLink>
                    </>
                }
                className="mb-3"
            />

            {editingDisabled && <ExternalServiceEditingDisabledAlert />}
            {externalServicesFromFile && allowEditExternalServicesWithFile && <ExternalServiceEditingTemporaryAlert />}

            <Container className="mb-3">
                <ConnectionContainer>
                    {error && <ConnectionError errors={[error.message]} />}
                    {loading && !connection && <ConnectionLoading />}
                    <ConnectionList as="ul" className="list-group" aria-label="Code Host Connections">
                        {connection?.nodes?.map(node => (
                            <ExternalServiceNode key={node.id} node={node} editingDisabled={editingDisabled} />
                        ))}
                    </ConnectionList>
                    {connection && (
                        <SummaryContainer className="mt-2" centered={true}>
                            <ConnectionSummary
                                noSummaryIfAllNodesVisible={false}
                                first={connection.totalCount ?? 0}
                                centered={true}
                                connection={connection}
                                noun="code host connection"
                                pluralNoun="code host connections"
                                hasNextPage={hasNextPage}
                            />
                            {hasNextPage && <ShowMoreButton centered={true} onClick={fetchMore} />}
                        </SummaryContainer>
                    )}
                </ConnectionContainer>
            </Container>
        </div>
    )
}
