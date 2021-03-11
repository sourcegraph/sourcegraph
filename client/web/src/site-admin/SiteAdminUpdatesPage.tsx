import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { parseISO } from 'date-fns'
import formatDistance from 'date-fns/formatDistance'
import CheckIcon from 'mdi-react/CheckIcon'
import CloudDownloadIcon from 'mdi-react/CloudDownloadIcon'
import React, { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { PageTitle } from '../components/PageTitle'
import { fetchSiteUpdateCheck } from './backend'
import { ErrorAlert } from '../components/alerts'
import { TelemetryProps } from '../../../shared/src/telemetry/telemetryService'
import { useObservable } from '../../../shared/src/util/useObservable'
import { isErrorLike } from '../../../shared/src/util/errors'

interface Props extends TelemetryProps {}

/**
 * A page displaying information about available updates for the server.
 */
export const SiteAdminUpdatesPage: React.FunctionComponent<Props> = ({ telemetryService }) => {
    useMemo(() => {
        telemetryService.logViewEvent('SiteAdminUpdates')
    }, [telemetryService])

    const state = useObservable(useMemo(() => fetchSiteUpdateCheck(), []))
    const autoUpdateCheckingEnabled = window.context.site['update.channel'] === 'release'

    if (state === undefined) {
        return <LoadingSpinner />
    }

    const updateCheck = state.updateCheck

    return (
        <div className="site-admin-updates-page">
            <PageTitle title="Updates - Admin" />
            <h2>Updates</h2>
            {isErrorLike(state) && <ErrorAlert className="site-admin-updates-page__error" error={state} />}
            {updateCheck && (updateCheck.pending || updateCheck.checkedAt) && (
                <div>
                    {updateCheck.pending && (
                        <div className="site-admin-updates-page__alert alert alert-primary">
                            <LoadingSpinner className="icon-inline" /> Checking for updates... (reload in a few seconds)
                        </div>
                    )}
                    {!updateCheck.errorMessage &&
                        (updateCheck.updateVersionAvailable ? (
                            <div className="site-admin-updates-page__alert alert alert-success">
                                <CloudDownloadIcon className="icon-inline" /> Update available:{' '}
                                <a href="https://about.sourcegraph.com">{updateCheck.updateVersionAvailable}</a>
                            </div>
                        ) : (
                            <div className="site-admin-updates-page__alert alert alert-success">
                                <CheckIcon className="icon-inline" /> Up to date.
                            </div>
                        ))}
                    {updateCheck.errorMessage && (
                        <ErrorAlert
                            className="site-admin-updates-page__alert"
                            prefix="Error checking for updates"
                            error={updateCheck.errorMessage}
                        />
                    )}
                </div>
            )}

            {!autoUpdateCheckingEnabled && (
                <div className="site-admin-updates-page__alert alert alert-warning">
                    Automatic update checking is disabled.
                </div>
            )}

            <p className="site-admin-updates_page__info">
                <small>
                    <strong>Current product version:</strong> {state.productVersion} ({state.buildVersion})
                </small>
                <br />
                <small>
                    <strong>Last update check:</strong>{' '}
                    {updateCheck.checkedAt
                        ? formatDistance(parseISO(updateCheck.checkedAt), new Date(), {
                              addSuffix: true,
                          })
                        : 'never'}
                    .
                </small>
                <br />
                <small>
                    <strong>Automatic update checking:</strong> {autoUpdateCheckingEnabled ? 'on' : 'off'}.{' '}
                    <Link to="/site-admin/configuration">Configure</Link> <code>update.channel</code> to{' '}
                    {autoUpdateCheckingEnabled ? 'disable' : 'enable'}.
                </small>
            </p>
            <p>
                <a href="https://about.sourcegraph.com/changelog" target="_blank" rel="noopener">
                    Sourcegraph changelog
                </a>
            </p>
        </div>
    )
}
