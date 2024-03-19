import React, { useEffect, useState, useCallback, useMemo, type FC } from 'react'

import { useApolloClient } from '@apollo/client'
import { mdiCog, mdiConnection, mdiDelete } from '@mdi/js'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import { useNavigate, useParams } from 'react-router-dom'
import { Subject } from 'rxjs'

import { asError, isErrorLike } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Alert, Button, Container, ErrorAlert, H2, H3, Icon, Link, PageHeader, Tooltip } from '@sourcegraph/wildcard'

import { DynamicallyImportedMonacoSettingsEditor } from '../../settings/DynamicallyImportedMonacoSettingsEditor'
import { refreshSiteFlags } from '../../site/backend'
import { CreatedByAndUpdatedByInfoByline } from '../Byline/CreatedByAndUpdatedByInfoByline'
import { useFetchGithubAppForES } from '../gitHubApps/backend'
import { HeroPage } from '../HeroPage'
import { LoaderButton } from '../LoaderButton'
import { PageTitle } from '../PageTitle'

import {
    useSyncExternalService,
    queryExternalServiceSyncJobs as _queryExternalServiceSyncJobs,
    deleteExternalService,
    useExternalServiceCheckConnectionByIdLazyQuery,
    type ExternalServiceFieldsWithConfig,
    useFetchExternalService,
} from './backend'
import { getBreadCrumbs } from './breadCrumbs'
import { ExternalServiceInformation } from './ExternalServiceInformation'
import { resolveExternalServiceCategory } from './externalServices'
import { ExternalServiceSyncJobsList } from './ExternalServiceSyncJobsList'
import { ExternalServiceWebhook } from './ExternalServiceWebhook'

import styles from './ExternalServicePage.module.scss'

interface Props extends TelemetryProps, TelemetryV2Props {
    afterDeleteRoute: string

    externalServicesFromFile: boolean
    allowEditExternalServicesWithFile: boolean

    /** For testing only. */
    queryExternalServiceSyncJobs?: typeof _queryExternalServiceSyncJobs
}

const NotFoundPage: FC = () => (
    <HeroPage icon={MapSearchIcon} title="404: Not Found" subtitle="Sorry, the requested code host was not found." />
)

export const ExternalServicePage: FC<Props> = props => {
    const {
        telemetryService,
        telemetryRecorder,
        afterDeleteRoute,
        externalServicesFromFile,
        allowEditExternalServicesWithFile,
        queryExternalServiceSyncJobs = _queryExternalServiceSyncJobs,
    } = props

    const isLightTheme = useIsLightTheme()
    const { externalServiceID } = useParams()
    const navigate = useNavigate()

    useEffect(() => {
        telemetryService.logViewEvent('SiteAdminExternalService')
        telemetryRecorder.recordEvent('admin.externalService', 'view')
    }, [telemetryService, telemetryRecorder])

    const [syncInProgress, setSyncInProgress] = useState<boolean>(false)
    // Callback used in ExternalServiceSyncJobsList to update the state in current component.
    const updateSyncInProgress = useCallback((updatedSyncInProgress: boolean) => {
        setSyncInProgress(updatedSyncInProgress)
    }, [])

    const [externalService, setExternalService] = useState<ExternalServiceFieldsWithConfig>()

    const { error: fetchError, loading: fetchLoading } = useFetchExternalService(externalServiceID!, setExternalService)
    const { error: fetchGHAppError, data: ghAppData } = useFetchGithubAppForES(externalService)
    const ghApp = useMemo(() => ghAppData?.gitHubAppByAppID, [ghAppData])

    const [numberOfRepos, setNumberOfRepos] = useState<number>(externalService?.repoCount ?? 0)
    // Callback used in ExternalServiceSyncJobsList to update the number of repos in current component.
    const updateNumberOfRepos = useCallback((updatedNumberOfRepos: number) => {
        setNumberOfRepos(updatedNumberOfRepos)
    }, [])

    const [syncExternalService, { error: syncExternalServiceError, loading: syncExternalServiceLoading }] =
        useSyncExternalService()

    const syncJobUpdates = useMemo(() => new Subject<void>(), [])
    const triggerSync = useCallback(
        () =>
            externalService &&
            syncExternalService({ variables: { id: externalService.id } }).then(() => {
                syncJobUpdates.next()
            }),
        [externalService, syncExternalService, syncJobUpdates]
    )

    const editingDisabled = externalServicesFromFile && !allowEditExternalServicesWithFile

    const [isDeleting, setIsDeleting] = useState<boolean | Error>(false)
    const client = useApolloClient()
    const onDelete = useCallback<React.MouseEventHandler>(async () => {
        if (!externalService) {
            return
        }
        if (!window.confirm(`Delete the external service ${externalService.displayName}?`)) {
            return
        }
        setIsDeleting(true)
        try {
            await deleteExternalService(externalService.id)
            setIsDeleting(false)
            await refreshSiteFlags(client)
            navigate(afterDeleteRoute)
        } catch (error) {
            setIsDeleting(asError(error))
        }
    }, [afterDeleteRoute, navigate, externalService, client])

    // If external service is undefined, we won't use doCheckConnection anyway,
    // that's why it's safe to pass an empty ID to useExternalServiceCheckConnectionByIdLazyQuery
    const [doCheckConnection, { loading, data, error }] = useExternalServiceCheckConnectionByIdLazyQuery(
        externalService?.id || ''
    )

    const checkConnectionNode = data?.node?.__typename === 'ExternalService' ? data.node.checkConnection : null

    const path = useMemo(() => getBreadCrumbs(externalService), [externalService])

    const mergedError = fetchError || fetchGHAppError

    const renderExternalService = (externalService: ExternalServiceFieldsWithConfig): JSX.Element => {
        let externalServiceAvailabilityStatus
        if (loading) {
            externalServiceAvailabilityStatus = (
                <Alert className="mt-2" variant="waiting">
                    Checking code host connection status...
                </Alert>
            )
        } else if (!error) {
            if (checkConnectionNode?.__typename === 'ExternalServiceAvailable') {
                externalServiceAvailabilityStatus = (
                    <Alert className="mt-2" variant="success">
                        Code host is reachable.
                    </Alert>
                )
            } else if (checkConnectionNode?.__typename === 'ExternalServiceUnavailable') {
                externalServiceAvailabilityStatus = (
                    <ErrorAlert
                        className="mt-2"
                        prefix="Error during code host connection check"
                        error={checkConnectionNode.suspectedReason}
                    />
                )
            }
        } else {
            externalServiceAvailabilityStatus = (
                <ErrorAlert
                    className="mt-2"
                    prefix="Unexpected error during code host connection check"
                    error={error.message}
                />
            )
        }
        const externalServiceCategory = resolveExternalServiceCategory(externalService)
        return (
            <Container className="mb-3">
                <PageHeader
                    path={path}
                    byline={
                        <CreatedByAndUpdatedByInfoByline
                            createdAt={externalService.createdAt}
                            updatedAt={externalService.updatedAt}
                            createdBy={externalService.creator}
                            updatedBy={externalService.lastUpdater}
                            type={externalService.__typename}
                        />
                    }
                    className="mb-3"
                    headingElement="h2"
                    actions={
                        <div className="d-flex align-items-center justify-content-between">
                            <div className="align-self-start">
                                <Tooltip
                                    content={
                                        externalService.hasConnectionCheck
                                            ? 'Test if code host is reachable from Sourcegraph'
                                            : 'Connection check unavailable'
                                    }
                                >
                                    <LoaderButton
                                        className="test-connection-external-service-button"
                                        variant="secondary"
                                        onClick={() => doCheckConnection()}
                                        disabled={!externalService.hasConnectionCheck || loading}
                                        size="sm"
                                        loading={loading}
                                        alwaysShowLabel={true}
                                        icon={<Icon aria-hidden={true} svgPath={mdiConnection} />}
                                        label="Test connection"
                                    />
                                </Tooltip>
                            </div>
                            {!editingDisabled && (
                                <div className="flex-grow-1 ml-1">
                                    <Tooltip content="Edit code host connection settings">
                                        <Button
                                            className="test-edit-external-service-button"
                                            to={`/site-admin/external-services/${encodeURIComponent(
                                                externalService.id
                                            )}/edit`}
                                            variant="primary"
                                            size="sm"
                                            as={Link}
                                        >
                                            <Icon aria-hidden={true} svgPath={mdiCog} />
                                            {' Edit'}
                                        </Button>
                                    </Tooltip>
                                </div>
                            )}
                            <div className="flex-shrink-0 ml-1">
                                <Tooltip
                                    content={
                                        editingDisabled
                                            ? 'Deleting code host connections through the UI is disabled when the EXTSVC_CONFIG_FILE environment variable is set.'
                                            : 'Delete code host connection'
                                    }
                                >
                                    <Button
                                        aria-label="Delete"
                                        className="test-delete-external-service-button"
                                        onClick={onDelete}
                                        disabled={isDeleting === true || editingDisabled}
                                        variant="danger"
                                        size="sm"
                                    >
                                        <Icon aria-hidden={true} svgPath={mdiDelete} />
                                        {' Delete'}
                                    </Button>
                                </Tooltip>
                            </div>
                        </div>
                    }
                />
                {isErrorLike(isDeleting) && <ErrorAlert className="mt-2" error={isDeleting} />}
                {externalServiceAvailabilityStatus}
                <H2>Information</H2>
                {externalService.unrestricted && (
                    <Alert className="mt-2" variant="warning">
                        <H3>All repositories will be unrestricted</H3>
                        This code host connection does not have authorization configured. Any repositories added by this
                        code host will be accessible by all users on the instance, even if another code host connection
                        with authorization syncs the same repository. See{' '}
                        <Link to="/help/admin/permissions#getting-started">the documentation</Link> for instructions on
                        configuring authorization.
                    </Alert>
                )}
                {externalServiceCategory && (
                    <ExternalServiceInformation
                        displayName={externalService.displayName}
                        codeHostID={externalService.id}
                        rateLimiterState={externalService.rateLimiterState}
                        reposNumber={numberOfRepos === 0 ? externalService.repoCount : numberOfRepos}
                        syncInProgress={syncInProgress}
                        gitHubApp={ghApp}
                        {...externalServiceCategory}
                    />
                )}
                <H2>Configuration</H2>
                {externalServiceCategory && (
                    <DynamicallyImportedMonacoSettingsEditor
                        value={externalService.config}
                        jsonSchema={externalServiceCategory.jsonSchema}
                        canEdit={false}
                        loading={fetchLoading}
                        height={350}
                        readOnly={true}
                        isLightTheme={isLightTheme}
                        className="test-external-service-editor"
                        telemetryService={telemetryService}
                        telemetryRecorder={telemetryRecorder}
                    />
                )}
                <ExternalServiceWebhook externalService={externalService} className="mt-3" />
                <LoaderButton
                    label="Trigger manual sync"
                    className="mt-3 mb-2 float-right"
                    alwaysShowLabel={true}
                    variant="secondary"
                    onClick={triggerSync}
                    loading={syncExternalServiceLoading}
                    disabled={syncExternalServiceLoading}
                />
                {syncExternalServiceError && <ErrorAlert error={syncExternalServiceError} />}
                <ExternalServiceSyncJobsList
                    queryExternalServiceSyncJobs={queryExternalServiceSyncJobs}
                    updateSyncInProgress={updateSyncInProgress}
                    updateNumberOfRepos={updateNumberOfRepos}
                    externalServiceID={externalService.id}
                    updates={syncJobUpdates}
                />
            </Container>
        )
    }

    return (
        <div className={styles.externalServicePage}>
            {externalService ? (
                <PageTitle title={`Code host - ${externalService.displayName}`} />
            ) : (
                <PageTitle title="Code host" />
            )}
            {mergedError && <ErrorAlert className="mb-3" error={fetchError} />}
            {!fetchLoading && !externalService && !fetchError && <NotFoundPage />}
            {externalService && renderExternalService(externalService)}
        </div>
    )
}
