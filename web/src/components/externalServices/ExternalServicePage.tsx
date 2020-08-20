import { parse as parseJSONC } from '@sqs/jsonc-parser'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import React, { useEffect, useState, useCallback } from 'react'
import { RouteComponentProps } from 'react-router'
import { concat, Observable } from 'rxjs'
import { catchError, switchMap } from 'rxjs/operators'
import * as GQL from '../../../../shared/src/graphql/schema'
import { asError, ErrorLike, isErrorLike } from '../../../../shared/src/util/errors'
import { PageTitle } from '../PageTitle'
import { ExternalServiceCard } from './ExternalServiceCard'
import { ErrorAlert } from '../alerts'
import { defaultExternalServices, codeHostExternalServices } from './externalServices'
import { hasProperty } from '../../../../shared/src/util/types'
import * as H from 'history'
import { useEventObservable } from '../../../../shared/src/util/useObservable'
import { TelemetryProps } from '../../../../shared/src/telemetry/telemetryService'
import { isExternalService, updateExternalService, fetchExternalService } from './backend'
import { ExternalServiceWebhook } from './ExternalServiceWebhook'
import { ExternalServiceForm } from './ExternalServiceForm'
import { ExternalServiceFields } from '../../graphql-operations'

interface Props extends RouteComponentProps<{ id: GQL.ID }>, TelemetryProps {
    isLightTheme: boolean
    history: H.History
    afterUpdateRoute: string
}

export const ExternalServicePage: React.FunctionComponent<Props> = ({
    match,
    history,
    isLightTheme,
    telemetryService,
    afterUpdateRoute,
}) => {
    useEffect(() => {
        telemetryService.logViewEvent('SiteAdminExternalService')
    }, [telemetryService])

    const [externalServiceOrError, setExternalServiceOrError] = useState<ExternalServiceFields | ErrorLike>()

    useEffect(() => {
        const subscription = fetchExternalService(match.params.id)
            .pipe(catchError(error => [asError(error)]))
            .subscribe(result => {
                setExternalServiceOrError(result)
            })
        return () => subscription.unsubscribe()
    }, [match.params.id])

    const onChange = useCallback(
        (input: GQL.IAddExternalServiceInput) => {
            if (isExternalService(externalServiceOrError)) {
                setExternalServiceOrError({ ...externalServiceOrError, ...input })
            }
        },
        [externalServiceOrError, setExternalServiceOrError]
    )

    const [nextSubmit, updatedServiceOrError] = useEventObservable(
        useCallback(
            (submits: Observable<ExternalServiceFields>): Observable<ErrorLike | ExternalServiceFields> =>
                submits.pipe(
                    switchMap(input =>
                        concat(updateExternalService({ input }).pipe(catchError((error: Error) => [asError(error)])))
                    )
                ),
            []
        )
    )

    // If the update was successful, and did not surface a warning, redirect to the
    // repositories page, adding `?repositoriesUpdated` to the query string so that we display
    // a banner at the top of the page.
    useEffect(() => {
        if (updatedServiceOrError && !isErrorLike(updatedServiceOrError)) {
            if (updatedServiceOrError.warning) {
                setExternalServiceOrError(updatedServiceOrError)
            } else {
                history.push(afterUpdateRoute)
            }
        }
    }, [updatedServiceOrError, history, afterUpdateRoute])

    const onSubmit = useCallback(
        (event?: React.FormEvent<HTMLFormElement>): void => {
            if (event) {
                event.preventDefault()
            }
            if (isExternalService(externalServiceOrError)) {
                nextSubmit(externalServiceOrError)
            }
        },
        [externalServiceOrError, nextSubmit]
    )
    let error: ErrorLike | undefined
    if (isErrorLike(updatedServiceOrError)) {
        error = updatedServiceOrError
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
            {isErrorLike(externalServiceOrError) && (
                <ErrorAlert className="mb-3" error={externalServiceOrError} history={history} />
            )}
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
                    loading={updatedServiceOrError === undefined}
                    onSubmit={onSubmit}
                    onChange={onChange}
                    history={history}
                    isLightTheme={isLightTheme}
                    telemetryService={telemetryService}
                />
            )}
            {externalService && <ExternalServiceWebhook externalService={externalService} />}
        </div>
    )
}
