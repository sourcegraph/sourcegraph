import React, { useCallback, useState, useEffect } from 'react'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'

import { CodeHostItem } from './CodeHostItem'
import { PageTitle } from '../../../components/PageTitle'
import { AddExternalServiceOptions } from '../../../components/externalServices/externalServices'
import { queryExternalServices } from '../../../components/externalServices/backend'
import { ErrorAlert } from '../../../components/alerts'
import { Link } from '../../../../../shared/src/components/Link'
import { isDefined, keyExistsIn } from '../../../../../shared/src/util/types'

import { asError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { Scalars, ExternalServiceKind, ListExternalServiceFields } from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'

export interface UserAddCodeHostsPageProps {
    userID: Scalars['ID']
    codeHostExternalServices: Record<string, AddExternalServiceOptions>
    routingPrefix: string
}

type ServicesByKind = Partial<Record<ExternalServiceKind, ListExternalServiceFields>>
type Status = undefined | 'loading' | ServicesByKind | ErrorLike

const isServicesByKind = (status: Status): status is ServicesByKind =>
    typeof status === 'object' && Object.keys(status).every(key => keyExistsIn(key, ExternalServiceKind))

export const UserAddCodeHostsPage: React.FunctionComponent<UserAddCodeHostsPageProps> = ({
    userID,
    codeHostExternalServices,
    routingPrefix,
}) => {
    const [statusOrError, setStatusOrError] = useState<Status>()

    const [isUpdateModalOpen, setIssUpdateModalOpen] = useState(false)
    const toggleUpdateModal = useCallback(() => {
        setIssUpdateModalOpen(!isUpdateModalOpen)
    }, [isUpdateModalOpen])

    const fetchExternalServices = useCallback(async () => {
        setStatusOrError('loading')

        const { nodes: fetchedServices } = await queryExternalServices({
            namespace: userID,
            first: null,
            after: null,
        }).toPromise()

        const services: ServicesByKind = fetchedServices.reduce<ServicesByKind>((accumulator, service) => {
            // backend constraint - non-admin users have only one external service per ExternalServiceKind
            accumulator[service.kind] = service
            return accumulator
        }, {})

        setStatusOrError(services)
    }, [userID])

    useEffect(() => {
        eventLogger.logViewEvent('UserSettingsCodeHostConnections')
    }, [])

    useEffect(() => {
        fetchExternalServices().catch(error => {
            setStatusOrError(asError(error))
        })
    }, [fetchExternalServices])

    const handleServiceUpsert = useCallback(
        (service: ListExternalServiceFields): void => {
            if (isServicesByKind(statusOrError)) {
                setStatusOrError({ ...statusOrError, [service.kind]: service })
            }
        },
        [statusOrError]
    )

    const getServiceWarningFragment = ({ id, displayName }: ListExternalServiceFields): React.ReactFragment => (
        <div className="alert alert-danger my-4" key={id}>
            <strong className="align-middle">Could not connect to {displayName}.</strong>
            <span className="align-middle"> Please </span>
            <button type="button" className="btn btn-link text-primary p-0" onClick={toggleUpdateModal}>
                update your access token
            </button>{' '}
            <span className="align-middle">to restore the connection.</span>
        </div>
    )

    return (
        <div className="add-user-code-hosts-page">
            <PageTitle title="Code host connections" />
            <div className="d-flex justify-content-between align-items-center mb-3">
                <h2 className="mb-0">Code host connections</h2>
            </div>
            <p className="text-muted mt-2">
                Connect with providers where your source code is hosted. Then,{' '}
                <Link className="text-primary" to={`${routingPrefix}/repositories`}>
                    add repositories
                </Link>{' '}
                to search with Sourcegraph.
            </p>

            {/* display external service errors */}
            {isServicesByKind(statusOrError) &&
                Object.values(statusOrError)
                    .filter(isDefined)
                    // Services may return warnings/errors immediately or after
                    // the sync. We want to display an alert for both.
                    .filter(service => service.warning || service.lastSyncError)
                    .map(getServiceWarningFragment)}

            {/* display other errors */}
            {isErrorLike(statusOrError) && (
                <ErrorAlert error={statusOrError} prefix="Code host action error" icon={false} />
            )}

            {codeHostExternalServices && statusOrError !== 'loading' ? (
                <ul className="list-group">
                    {Object.entries(codeHostExternalServices).map(([id, { kind, defaultDisplayName, icon }]) => (
                        <li key={id} className="list-group-item">
                            <CodeHostItem
                                service={isServicesByKind(statusOrError) ? statusOrError[kind] : undefined}
                                userID={userID}
                                kind={kind}
                                name={defaultDisplayName}
                                icon={icon}
                                isUpdateModalOpen={isUpdateModalOpen}
                                toggleUpdateModal={toggleUpdateModal}
                                onDidUpsert={handleServiceUpsert}
                                onDidRemove={fetchExternalServices}
                                onDidError={setStatusOrError}
                            />
                        </li>
                    ))}
                </ul>
            ) : (
                <div className="d-flex justify-content-center">
                    <LoadingSpinner className="icon-inline" />
                </div>
            )}
        </div>
    )
}
