import AddIcon from 'mdi-react/AddIcon'
import React, { useEffect, useMemo, useCallback, useState } from 'react'
import { Redirect } from 'react-router'
import { Subject } from 'rxjs'
import { tap } from 'rxjs/operators'
import { ActivationProps } from '../../../../shared/src/components/activation/Activation'
import { FilteredConnection, FilteredConnectionQueryArguments } from '../FilteredConnection'
import { PageTitle } from '../PageTitle'
import * as H from 'history'
import { queryExternalServices as _queryExternalServices } from './backend'
import { ExternalServiceNodeProps, ExternalServiceNode } from './ExternalServiceNode'
import { ListExternalServiceFields, Scalars, ExternalServicesResult } from '../../graphql-operations'
import { TelemetryProps } from '../../../../shared/src/telemetry/telemetryService'
import { Link } from '../../../../shared/src/components/Link'
import { AuthenticatedUser } from '../../auth'
import { isErrorLike, ErrorLike } from '../../../../shared/src/util/errors'

interface Props extends ActivationProps, TelemetryProps {
    history: H.History
    location: H.Location
    routingPrefix: string
    afterDeleteRoute: string
    userID?: Scalars['ID']
    authenticatedUser: Pick<AuthenticatedUser, 'id'>

    /** For testing only. */
    queryExternalServices?: typeof _queryExternalServices
}

/**
 * A page displaying the external services on this site.
 */
export const ExternalServicesPage: React.FunctionComponent<Props> = ({
    afterDeleteRoute,
    history,
    location,
    routingPrefix,
    activation,
    userID,
    telemetryService,
    authenticatedUser,
    queryExternalServices = _queryExternalServices,
}) => {
    useEffect(() => {
        telemetryService.logViewEvent('SiteAdminExternalServices')
    }, [telemetryService])
    const updates = useMemo(() => new Subject<void>(), [])
    const onDidUpdateExternalServices = useCallback(() => updates.next(), [updates])

    const queryConnection = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryExternalServices({
                first: args.first ?? null,
                after: args.after ?? null,
                namespace: userID ?? null,
            }).pipe(
                tap(externalServices => {
                    if (activation && externalServices.totalCount > 0) {
                        activation.update({ ConnectedCodeHost: true })
                    }
                })
            ),
        // Activation changes in here, so we cannot recreate the callback on change,
        // or queryConnection will constantly change, resulting in infinite refetch loops.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [userID, queryExternalServices]
    )

    const [noExternalServices, setNoExternalServices] = useState<boolean>(false)
    const onUpdate = useCallback<
        (connection: ExternalServicesResult['externalServices'] | ErrorLike | undefined) => void
    >(connection => {
        if (connection && !isErrorLike(connection)) {
            setNoExternalServices(connection.totalCount === 0)
        }
    }, [])

    const isManagingOtherUser = !!userID && userID !== authenticatedUser.id

    if (!isManagingOtherUser && noExternalServices) {
        return <Redirect to={`${routingPrefix}/external-services/new`} />
    }
    return (
        <div className="site-admin-external-services-page">
            <PageTitle title="Manage repositories" />
            <div className="d-flex justify-content-between align-items-center mb-3">
                <h2 className="mb-0">Manage repositories</h2>
                {!isManagingOtherUser && (
                    <Link
                        className="btn btn-primary test-goto-add-external-service-page"
                        to={`${routingPrefix}/external-services/new`}
                    >
                        <AddIcon className="icon-inline" /> Add code host
                    </Link>
                )}
            </div>
            <p className="mt-2">Manage code host connections to sync repositories.</p>
            <FilteredConnection<
                ListExternalServiceFields,
                Omit<ExternalServiceNodeProps, 'node'>,
                ExternalServicesResult['externalServices']
            >
                className="list-group list-group-flush mt-3"
                noun="external service"
                pluralNoun="external services"
                queryConnection={queryConnection}
                nodeComponent={ExternalServiceNode}
                nodeComponentProps={{
                    onDidUpdate: onDidUpdateExternalServices,
                    history,
                    routingPrefix,
                    afterDeleteRoute,
                }}
                hideSearch={true}
                noSummaryIfAllNodesVisible={true}
                cursorPaging={true}
                updates={updates}
                history={history}
                location={location}
                onUpdate={onUpdate}
            />
        </div>
    )
}
