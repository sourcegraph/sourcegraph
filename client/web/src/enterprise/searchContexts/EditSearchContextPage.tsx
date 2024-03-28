import React, { useCallback, useEffect, useMemo } from 'react'

import { mdiMagnify } from '@mdi/js'
import { useParams } from 'react-router-dom'
import { type Observable, of, throwError } from 'rxjs'
import { catchError, startWith, switchMap } from 'rxjs/operators'

import { asError, isErrorLike } from '@sourcegraph/common'
import type {
    Scalars,
    SearchContextEditInput,
    SearchContextRepositoryRevisionsInput,
    SearchContextFields,
} from '@sourcegraph/shared/src/graphql-operations'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SearchContextProps } from '@sourcegraph/shared/src/search'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { PageHeader, LoadingSpinner, useObservable, Alert } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { withAuthenticatedUser } from '../../auth/withAuthenticatedUser'
import { Page } from '../../components/Page'
import { PageTitle } from '../../components/PageTitle'

import { SearchContextForm } from './SearchContextForm'

export interface EditSearchContextPageProps
    extends TelemetryProps,
        Pick<SearchContextProps, 'updateSearchContext' | 'fetchSearchContextBySpec' | 'deleteSearchContext'>,
        PlatformContextProps<'requestGraphQL' | 'telemetryRecorder'> {
    authenticatedUser: AuthenticatedUser
    isSourcegraphDotCom: boolean
}

export const AuthenticatedEditSearchContextPage: React.FunctionComponent<
    React.PropsWithChildren<EditSearchContextPageProps>
> = props => {
    const LOADING = 'loading' as const

    const params = useParams()
    const spec: string = params.spec ? `${params.specOrOrg}/${params.spec}` : params.specOrOrg!

    const { updateSearchContext, fetchSearchContextBySpec, platformContext } = props

    useEffect(() => {
        platformContext.telemetryRecorder.recordEvent('searchContext.edit', 'view')
    }, [platformContext.telemetryRecorder])

    const onSubmit = useCallback(
        (
            id: Scalars['ID'] | undefined,
            searchContext: SearchContextEditInput,
            repositories: SearchContextRepositoryRevisionsInput[]
        ): Observable<SearchContextFields> => {
            if (!id) {
                return throwError(new Error('Cannot update search context with undefined ID'))
            }
            platformContext.telemetryRecorder.recordEvent('searchContext', 'update')
            return updateSearchContext({ id, searchContext, repositories }, platformContext)
        },
        [updateSearchContext, platformContext]
    )

    const searchContextOrError = useObservable(
        useMemo(
            () =>
                fetchSearchContextBySpec(spec, platformContext).pipe(
                    switchMap(searchContext => {
                        if (!searchContext.viewerCanManage) {
                            return throwError(new Error('You do not have sufficient permissions to edit this context.'))
                        }
                        return of(searchContext)
                    }),
                    startWith(LOADING),
                    catchError(error => [asError(error)])
                ),
            [spec, fetchSearchContextBySpec, platformContext]
        )
    )

    return (
        <div className="w-100">
            <Page>
                <div className="container col-sm-8">
                    <PageTitle title="Edit context" />
                    <PageHeader
                        className="mb-3"
                        path={[
                            {
                                icon: mdiMagnify,
                                to: '/search',
                                ariaLabel: 'Code Search',
                            },
                            {
                                to: '/contexts',
                                text: 'Contexts',
                            },
                            {
                                text: 'Edit context',
                            },
                        ]}
                    />
                    {searchContextOrError === LOADING && (
                        <div className="d-flex justify-content-center">
                            <LoadingSpinner inline={false} />
                        </div>
                    )}
                    {searchContextOrError && searchContextOrError !== LOADING && !isErrorLike(searchContextOrError) && (
                        <SearchContextForm {...props} searchContext={searchContextOrError} onSubmit={onSubmit} />
                    )}
                    {isErrorLike(searchContextOrError) && (
                        <Alert data-testid="search-contexts-alert-danger" variant="danger">
                            Error while loading the search context: <strong>{searchContextOrError.message}</strong>
                        </Alert>
                    )}
                </div>
            </Page>
        </div>
    )
}

export const EditSearchContextPage = withAuthenticatedUser(AuthenticatedEditSearchContextPage)
