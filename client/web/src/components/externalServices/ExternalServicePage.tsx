import { parse as parseJSONC } from '@sqs/jsonc-parser'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import React, { useEffect, useState, useCallback } from 'react'
import { catchError } from 'rxjs/operators'
import * as GQL from '../../../../shared/src/graphql/schema'
import { asError, ErrorLike, isErrorLike } from '../../../../shared/src/util/errors'
import { PageTitle } from '../PageTitle'
import { ExternalServiceCard } from './ExternalServiceCard'
import { ErrorAlert } from '../alerts'
import { defaultExternalServices, codeHostExternalServices } from './externalServices'
import { hasProperty } from '../../../../shared/src/util/types'
import * as H from 'history'
import { TelemetryProps } from '../../../../shared/src/telemetry/telemetryService'
import { isExternalService, updateExternalService, fetchExternalService as _fetchExternalService } from './backend'
import { ExternalServiceWebhook } from './ExternalServiceWebhook'
import { ExternalServiceForm } from './ExternalServiceForm'
import { ExternalServiceFields, Scalars } from '../../graphql-operations'

interface Props extends TelemetryProps {
    externalServiceID: Scalars['ID']
    isLightTheme: boolean
    history: H.History
    afterUpdateRoute: string

    /** For testing only. */
    fetchExternalService?: typeof _fetchExternalService
    /** For testing only. */
    autoFocusForm?: boolean
}

export const ExternalServicePage: React.FunctionComponent<Props> = ({
    externalServiceID,
    history,
    isLightTheme,
    telemetryService,
    afterUpdateRoute,
    fetchExternalService = _fetchExternalService,
    autoFocusForm,
}) => {
    useEffect(() => {
        telemetryService.logViewEvent('SiteAdminExternalService')
    }, [telemetryService])

    const [externalServiceOrError, setExternalServiceOrError] = useState<ExternalServiceFields | ErrorLike>()

    useEffect(() => {
        const subscription = fetchExternalService(externalServiceID)
            .pipe(catchError(error => [asError(error)]))
            .subscribe(result => {
                setExternalServiceOrError(result)
            })
        return () => subscription.unsubscribe()
    }, [externalServiceID, fetchExternalService])

    const onChange = useCallback(
        (input: GQL.IAddExternalServiceInput) => {
            if (isExternalService(externalServiceOrError)) {
                setExternalServiceOrError({ ...externalServiceOrError, ...input })
            }
        },
        [externalServiceOrError, setExternalServiceOrError]
    )

    const [isUpdating, setIsUpdating] = useState<boolean | Error>()
    const onSubmit = useCallback(
        async (event?: React.FormEvent<HTMLFormElement>): Promise<void> => {
            if (event) {
                event.preventDefault()
            }
            if (isExternalService(externalServiceOrError)) {
                try {
                    setIsUpdating(true)
                    const updatedService = await updateExternalService({ input: externalServiceOrError })
                    setIsUpdating(false)
                    // If the update was successful, and did not surface a warning, redirect to the
                    // repositories page, adding `?repositoriesUpdated` to the query string so that we display
                    // a banner at the top of the page.
                    if (updatedService.warning) {
                        setExternalServiceOrError(updatedService)
                    } else {
                        history.push(afterUpdateRoute)
                    }
                } catch (error) {
                    setIsUpdating(asError(error))
                }
            }
        },
        [afterUpdateRoute, externalServiceOrError, history]
    )
    let error: ErrorLike | undefined
    if (isErrorLike(isUpdating)) {
        error = isUpdating
    }

    const externalService = (!isErrorLike(externalServiceOrError) && externalServiceOrError) || undefined

    let externalServiceCategory = externalService && defaultExternalServices[externalService.kind]
    if (
        externalService &&
        [GQL.ExternalServiceKind.GITHUB, GQL.ExternalServiceKind.GITLAB].includes(externalService.kind)
    ) {
        const parsedConfig: unknown = parseJSONC(externalService.config)
        const url =
            typeof parsedConfig === 'object' &&
            parsedConfig !== null &&
            hasProperty('url')(parsedConfig) &&
            typeof parsedConfig.url === 'string'
                ? new URL(parsedConfig.url)
                : undefined
        // We have no way of finding out whether a externalservice of kind GITHUB is GitHub.com or GitHub enterprise, so we need to guess based on the URL.
        if (externalService.kind === GQL.ExternalServiceKind.GITHUB && url?.hostname !== 'github.com') {
            externalServiceCategory = codeHostExternalServices.ghe
        }
        // We have no way of finding out whether a externalservice of kind GITLAB is Gitlab.com or Gitlab self-hosted, so we need to guess based on the URL.
        if (externalService.kind === GQL.ExternalServiceKind.GITLAB && url?.hostname !== 'gitlab.com') {
            externalServiceCategory = codeHostExternalServices.gitlab
        }
    }

    return (
        <div className="site-admin-configuration-page">
            {externalService ? (
                <PageTitle title={`External service - ${externalService.displayName}`} />
            ) : (
                <PageTitle title="External service" />
            )}
            <h2>Update synced repositories</h2>
            {externalServiceOrError === undefined && <LoadingSpinner className="icon-inline" />}
            {isErrorLike(externalServiceOrError) && <ErrorAlert className="mb-3" error={externalServiceOrError} />}
            {externalServiceCategory && (
                <div className="mb-3">
                    <ExternalServiceCard {...externalServiceCategory} />
                </div>
            )}
            {externalService && externalServiceCategory && (
                <ExternalServiceForm
                    input={externalService}
                    editorActions={externalServiceCategory.editorActions}
                    jsonSchema={externalServiceCategory.jsonSchema}
                    error={error}
                    warning={externalService.warning}
                    mode="edit"
                    loading={isUpdating === true}
                    onSubmit={onSubmit}
                    onChange={onChange}
                    history={history}
                    isLightTheme={isLightTheme}
                    telemetryService={telemetryService}
                    autoFocus={autoFocusForm}
                />
            )}
            {externalService && <ExternalServiceWebhook externalService={externalService} />}
        </div>
    )
}
