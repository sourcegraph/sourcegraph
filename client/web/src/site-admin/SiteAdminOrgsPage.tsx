import AddIcon from 'mdi-react/AddIcon'
import DeleteIcon from 'mdi-react/DeleteIcon'
import SettingsIcon from 'mdi-react/SettingsIcon'
import UserIcon from 'mdi-react/UserIcon'
import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { Subject } from 'rxjs'
import { pluralize } from '../../../shared/src/util/strings'
import { FilteredConnection } from '../components/FilteredConnection'
import { PageTitle } from '../components/PageTitle'
import { orgURL } from '../org'
import { deleteOrganization, fetchAllOrganizations } from './backend'
import { ErrorAlert } from '../components/alerts'
import { asError, isErrorLike } from '../../../shared/src/util/errors'
import * as H from 'history'
import { OrganizationFields } from '../graphql-operations'
import { TelemetryProps } from '../../../shared/src/telemetry/telemetryService'

interface OrgNodeProps {
    /**
     * The org to display in this list item.
     */
    node: OrganizationFields

    /**
     * Called when the org is updated by an action in this list item.
     */
    onDidUpdate?: () => void
    history: H.History
}

const OrgNode: React.FunctionComponent<OrgNodeProps> = ({ node, history, onDidUpdate }) => {
    const [loading, setLoading] = useState<boolean | Error>(false)

    const deleteOrg = useCallback(() => {
        if (!window.confirm(`Delete the organization ${node.name}?`)) {
            return
        }

        setLoading(true)

        deleteOrganization(node.id).then(
            () => {
                setLoading(false)
                if (onDidUpdate) {
                    onDidUpdate()
                }
            },
            error => setLoading(asError(error))
        )
    }, [node.id, node.name, onDidUpdate])

    return (
        <li className="list-group-item py-2">
            <div className="d-flex align-items-center justify-content-between">
                <div>
                    <Link to={orgURL(node.name)}>
                        <strong>{node.name}</strong>
                    </Link>
                    <br />
                    <span className="text-muted">{node.displayName}</span>
                </div>
                <div>
                    <Link
                        to={`${orgURL(node.name)}/settings`}
                        className="btn btn-sm btn-secondary"
                        data-tooltip="Organization settings"
                    >
                        <SettingsIcon className="icon-inline" /> Settings
                    </Link>{' '}
                    <Link
                        to={`${orgURL(node.name)}/members`}
                        className="btn btn-sm btn-secondary"
                        data-tooltip="Organization members"
                    >
                        <UserIcon className="icon-inline" />{' '}
                        {node.members && (
                            <>
                                {node.members.totalCount} {pluralize('member', node.members.totalCount)}
                            </>
                        )}
                    </Link>{' '}
                    <button
                        type="button"
                        className="btn btn-sm btn-danger"
                        onClick={deleteOrg}
                        disabled={loading === true}
                        data-tooltip="Delete organization"
                    >
                        <DeleteIcon className="icon-inline" />
                    </button>
                </div>
            </div>
            {isErrorLike(loading) && <ErrorAlert className="mt-2" error={loading.message} />}
        </li>
    )
}

interface Props extends RouteComponentProps<{}>, TelemetryProps {}

/**
 * A page displaying the orgs on this site.
 */
export const SiteAdminOrgsPage: React.FunctionComponent<Props> = ({ telemetryService, history, location }) => {
    const orgUpdates = useMemo(() => new Subject<void>(), [])
    const onDidUpdateOrg = useCallback((): void => orgUpdates.next(), [orgUpdates])

    useEffect(() => {
        telemetryService.logViewEvent('SiteAdminOrgs')
    }, [telemetryService])

    return (
        <div className="site-admin-orgs-page">
            <PageTitle title="Organizations - Admin" />
            <div className="d-flex justify-content-between align-items-center mb-3">
                <h2 className="mb-0">Organizations</h2>
                <Link to="/organizations/new" className="btn btn-primary test-create-org-button">
                    <AddIcon className="icon-inline" /> Create organization
                </Link>
            </div>
            <p>
                An organization is a set of users with associated configuration. See{' '}
                <Link to="/help/admin/organizations">Sourcegraph documentation</Link> for information about configuring
                organizations.
            </p>
            <FilteredConnection<OrganizationFields, Omit<OrgNodeProps, 'node'>>
                className="list-group list-group-flush mt-3"
                noun="organization"
                pluralNoun="organizations"
                queryConnection={fetchAllOrganizations}
                nodeComponent={OrgNode}
                nodeComponentProps={{
                    onDidUpdate: onDidUpdateOrg,
                    history,
                }}
                updates={orgUpdates}
                history={history}
                location={location}
            />
        </div>
    )
}
