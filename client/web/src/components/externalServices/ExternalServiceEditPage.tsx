import React, { type FC, useEffect, useState, useCallback, useMemo } from 'react'

import { useNavigate, useParams } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Container, ErrorAlert, PageHeader, ButtonLink } from '@sourcegraph/wildcard'

import type { AddExternalServiceInput } from '../../graphql-operations'
import { CreatedByAndUpdatedByInfoByline } from '../Byline/CreatedByAndUpdatedByInfoByline'
import { useFetchGithubAppForES } from '../gitHubApps/backend'
import { PageTitle } from '../PageTitle'

import {
    useUpdateExternalService,
    type ExternalServiceFieldsWithConfig,
    useFetchExternalService,
    getExternalService,
} from './backend'
import { getBreadCrumbs } from './breadCrumbs'
import { ExternalServiceForm } from './ExternalServiceForm'
import { resolveExternalServiceCategory } from './externalServices'
import { ExternalServiceWebhook } from './ExternalServiceWebhook'

interface Props extends TelemetryProps, TelemetryV2Props {
    externalServicesFromFile: boolean
    allowEditExternalServicesWithFile: boolean

    /** For testing only. */
    autoFocusForm?: boolean
}

export const ExternalServiceEditPage: FC<Props> = ({
    telemetryService,
    externalServicesFromFile,
    allowEditExternalServicesWithFile,
    autoFocusForm,
    telemetryRecorder,
}) => {
    const { externalServiceID } = useParams()
    const navigate = useNavigate()

    useEffect(() => {
        telemetryService.logViewEvent('SiteAdminExternalService')
        telemetryRecorder.recordEvent('admin.externalService.edit', 'view')
    }, [telemetryService, telemetryRecorder])

    const [externalService, setExternalService] = useState<ExternalServiceFieldsWithConfig>()

    const { error: fetchError, loading: fetchLoading } = useFetchExternalService(externalServiceID!, setExternalService)
    const {
        error: fetchGHAppError,
        loading: fetchGHAppLoading,
        data: ghAppData,
    } = useFetchGithubAppForES(externalService)
    const ghApp = useMemo(() => ghAppData?.gitHubAppByAppID, [ghAppData])

    const [updated, setUpdated] = useState(false)
    const [updateExternalService, { error: updateExternalServiceError, loading: updateExternalServiceLoading }] =
        useUpdateExternalService(result => {
            setExternalService(getExternalService(result.updateExternalService))
            setUpdated(true)
        })

    const onSubmit = useCallback(
        async (event?: React.FormEvent<HTMLFormElement>) => {
            event?.preventDefault()

            if (externalService !== undefined) {
                await updateExternalService({
                    variables: {
                        input: {
                            id: externalService.id,
                            displayName: externalService.displayName,
                            config: externalService.config,
                        },
                    },
                })
            }
        },
        [externalService, updateExternalService]
    )

    const onChange = useCallback(
        (input: AddExternalServiceInput) => {
            if (externalService !== undefined) {
                setExternalService(
                    getExternalService({
                        ...externalService,
                        ...input,
                    })
                )
            }
        },
        [externalService, setExternalService]
    )

    const path = useMemo(() => getBreadCrumbs(externalService, true), [externalService])

    const combinedError = fetchError || updateExternalServiceError || fetchGHAppError
    const combinedLoading = fetchLoading || updateExternalServiceLoading || fetchGHAppLoading

    if (updated && !combinedLoading && externalService?.warning === null) {
        navigate(`/site-admin/external-services/${encodeURIComponent(externalService.id)}`, { replace: true })
    }

    const renderService = (externalService: ExternalServiceFieldsWithConfig): JSX.Element => {
        const externalServiceCategory = resolveExternalServiceCategory(externalService, ghApp)
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
                        <ButtonLink
                            to={`/site-admin/external-services/${encodeURIComponent(externalService.id)}`}
                            variant="secondary"
                        >
                            Cancel
                        </ButtonLink>
                    }
                />
                {externalServiceCategory && (
                    <ExternalServiceForm
                        input={{ ...externalService }}
                        externalServiceID={externalServiceID}
                        editorActions={externalServiceCategory.editorActions}
                        jsonSchema={externalServiceCategory.jsonSchema}
                        error={updateExternalServiceError}
                        warning={externalService.warning}
                        mode="edit"
                        loading={combinedLoading}
                        onSubmit={onSubmit}
                        onChange={onChange}
                        telemetryService={telemetryService}
                        autoFocus={autoFocusForm}
                        externalServicesFromFile={externalServicesFromFile}
                        allowEditExternalServicesWithFile={allowEditExternalServicesWithFile}
                        additionalFormComponent={externalServiceCategory.additionalFormComponent}
                        telemetryRecorder={telemetryRecorder}
                    />
                )}
                <ExternalServiceWebhook externalService={externalService} className="mt-3" />
            </Container>
        )
    }

    return (
        <div>
            {externalService ? (
                <PageTitle title={`Code host - ${externalService.displayName}`} />
            ) : (
                <PageTitle title="Code host" />
            )}
            {combinedError !== undefined && !combinedLoading && <ErrorAlert className="mb-3" error={combinedError} />}
            {externalService && renderService(externalService)}
        </div>
    )
}
