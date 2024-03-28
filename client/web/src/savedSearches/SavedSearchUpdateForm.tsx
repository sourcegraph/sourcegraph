import React, { type FC } from 'react'

import { useParams } from 'react-router-dom'
import { concat, of, Subject, Subscription } from 'rxjs'
import { catchError, delay, distinctUntilChanged, map, mergeMap, startWith, switchMap, tap } from 'rxjs/operators'

import { asError, type ErrorLike, isErrorLike } from '@sourcegraph/common'
import { Alert, LoadingSpinner } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import type { SavedSearchFields } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { fetchSavedSearch, updateSavedSearch } from '../search/backend'
import { eventLogger } from '../tracking/eventLogger'

import { type SavedQueryFields, SavedSearchForm } from './SavedSearchForm'

interface Props extends NamespaceProps {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    searchId: string
}

const LOADING = 'loading' as const

interface State {
    savedSearchOrError: typeof LOADING | SavedSearchFields | ErrorLike
    updatedOrError: null | true | typeof LOADING | ErrorLike
}

export const SavedSearchUpdateForm: FC<Omit<Props, 'searchId'>> = props => {
    const { id } = useParams<{ id: string }>()

    return <InnerSavedSearchUpdateForm {...props} searchId={id!} />
}

class InnerSavedSearchUpdateForm extends React.Component<Props, State> {
    constructor(props: Props) {
        super(props)
        this.state = {
            savedSearchOrError: LOADING,
            updatedOrError: null,
        }
    }

    private componentUpdates = new Subject<Props>()
    private subscriptions = new Subscription()
    private submits = new Subject<SavedQueryFields>()

    public componentDidMount(): void {
        this.subscriptions.add(
            this.componentUpdates
                .pipe(
                    map(props => props.searchId),
                    distinctUntilChanged(),
                    switchMap(id =>
                        fetchSavedSearch(id).pipe(
                            startWith(LOADING),
                            catchError(error => [asError(error)])
                        )
                    ),
                    map(result => ({ savedSearchOrError: result }))
                )
                .subscribe(stateUpdate => this.setState(stateUpdate))
        )

        this.subscriptions.add(
            this.submits
                .pipe(
                    switchMap(input =>
                        concat(
                            [{ updatedOrError: LOADING }],
                            updateSavedSearch(
                                input.id,
                                input.description,
                                input.query,
                                input.notify,
                                input.notifySlack,
                                this.props.namespace.__typename === 'User' ? this.props.namespace.id : null,
                                this.props.namespace.__typename === 'Org' ? this.props.namespace.id : null
                            ).pipe(
                                map(() => null),
                                tap(() => eventLogger.log('SavedSearchUpdated')),
                                mergeMap(() =>
                                    concat(
                                        // Flash "updated" text
                                        of({ updatedOrError: true }),
                                        // Hide "updated" text again after 1s
                                        of({ updatedOrError: null }).pipe(delay(1000))
                                    )
                                ),
                                catchError((error: Error) => [{ updatedOrError: asError(error) }])
                            )
                        )
                    )
                )
                .subscribe(stateUpdate => this.setState(stateUpdate as State))
        )

        this.componentUpdates.next(this.props)

        eventLogger.logViewEvent('UpdateSavedSearchPage')
    }

    public render(): JSX.Element | null {
        const savedSearch =
            (!isErrorLike(this.state.savedSearchOrError) &&
                this.state.savedSearchOrError !== LOADING &&
                this.state.savedSearchOrError) ||
            undefined

        return (
            <div>
                {this.state.savedSearchOrError === LOADING && <LoadingSpinner />}
                {this.props.authenticatedUser && savedSearch && (
                    <SavedSearchForm
                        {...this.props}
                        submitLabel="Update saved search"
                        title="Manage saved search"
                        defaultValues={{
                            id: savedSearch.id,
                            description: savedSearch.description,
                            query: savedSearch.query,
                            notify: savedSearch.notify,
                            notifySlack: savedSearch.notifySlack,
                            slackWebhookURL: savedSearch.slackWebhookURL,
                        }}
                        loading={this.state.updatedOrError === LOADING}
                        onSubmit={(fields: Pick<SavedQueryFields, Exclude<keyof SavedQueryFields, 'id'>>): void =>
                            this.onSubmit({ id: savedSearch.id, ...fields })
                        }
                        error={isErrorLike(this.state.updatedOrError) ? this.state.updatedOrError : undefined}
                    />
                )}
                {this.state.updatedOrError === true && (
                    <Alert variant="success" as="p">
                        Updated!
                    </Alert>
                )}
            </div>
        )
    }

    private onSubmit = (fields: SavedQueryFields): void => {
        this.submits.next(fields)
    }
}
