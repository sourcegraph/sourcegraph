import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import AddIcon from 'mdi-react/AddIcon'
import PuzzleIcon from 'mdi-react/PuzzleIcon'
import HelpCircleOutline from 'mdi-react/HelpCircleOutlineIcon'
import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { concat, Observable, Subject, Subscription } from 'rxjs'
import { catchError, concatMap, map, tap } from 'rxjs/operators'
import { gql } from '../../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../../shared/src/graphql/schema'
import { asError, createAggregateError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { withAuthenticatedUser } from '../../../auth/withAuthenticatedUser'
import { mutateGraphQL } from '../../../backend/graphql'
import { Form } from '../../../../../branded/src/components/Form'
import { ModalPage } from '../../../components/ModalPage'
import { PageTitle } from '../../../components/PageTitle'
import { RegistryPublisher, toExtensionID } from '../../../extensions/extension/extension'
import { eventLogger } from '../../../tracking/eventLogger'
import { RegistryExtensionNameFormGroup, RegistryPublisherFormGroup } from '../extension/RegistryExtensionForm'
import { queryViewerRegistryPublishers } from './backend'
import { RegistryAreaPageProps } from './RegistryArea'
import { ErrorAlert } from '../../../components/alerts'
import * as H from 'history'
import { AuthenticatedUser } from '../../../auth'
import { BreadcrumbSetters } from '../../../components/Breadcrumbs'
import { Scalars } from '../../../../../shared/src/graphql-operations'

function createExtension(
    publisher: Scalars['ID'],
    name: string
): Observable<GQL.IExtensionRegistryCreateExtensionResult> {
    return mutateGraphQL(
        gql`
            mutation CreateRegistryExtension($publisher: ID!, $name: String!) {
                extensionRegistry {
                    createExtension(publisher: $publisher, name: $name) {
                        extension {
                            id
                            extensionID
                            url
                        }
                    }
                }
            }
        `,
        { publisher, name }
    ).pipe(
        map(({ data, errors }) => {
            if (
                !data ||
                !data.extensionRegistry ||
                !data.extensionRegistry.createExtension ||
                (errors && errors.length > 0)
            ) {
                throw createAggregateError(errors)
            }
            return data.extensionRegistry.createExtension
        })
    )
}

interface Props extends RegistryAreaPageProps, RouteComponentProps<{}>, BreadcrumbSetters {
    authenticatedUser: AuthenticatedUser
    history: H.History
}

interface State {
    /** The viewer's authorized publishers, undefined while loading, or an error. */
    publishersOrError: 'loading' | RegistryPublisher[] | ErrorLike

    name: string
    publisher?: Scalars['ID']

    /** The creation result, undefined while loading, or an error. */
    creationOrError?: 'loading' | GQL.IExtensionRegistryCreateExtensionResult | ErrorLike
}

/** A page with a form to create a new extension in the extension registry. */
export const RegistryNewExtensionPage = withAuthenticatedUser(
    class RegistryNewExtensionPage extends React.PureComponent<Props, State> {
        public state: State = {
            publishersOrError: 'loading',
            name: '',
        }

        private submits = new Subject<React.FormEvent<HTMLFormElement>>()
        private componentUpdates = new Subject<Props>()
        private subscriptions = new Subscription()

        public componentDidMount(): void {
            eventLogger.logViewEvent('ExtensionRegistryCreateExtension')

            this.subscriptions.add(
                concat(
                    [{ publishersOrError: 'loading' }],
                    queryViewerRegistryPublishers().pipe(
                        map(result => ({ publishersOrError: result, publisher: result[0]?.id })),
                        catchError(error => [{ publishersOrError: asError(error) }])
                    )
                ).subscribe(
                    stateUpdate => this.setState(stateUpdate as State),
                    error => console.error(error)
                )
            )

            this.subscriptions.add(
                this.submits
                    .pipe(
                        tap(event => event.preventDefault()),
                        concatMap(() =>
                            concat(
                                [{ creationOrError: 'loading' }],
                                createExtension(this.state.publisher!, this.state.name).pipe(
                                    tap(result => {
                                        // Go to the page for the newly created extension.
                                        this.props.history.push(result.extension.url)
                                    }),
                                    map(result => ({ creationOrError: result })),
                                    catchError(error => [{ creationOrError: asError(error) }])
                                )
                            )
                        )
                    )
                    .subscribe(
                        stateUpdate => this.setState(stateUpdate as State),
                        error => console.error(error)
                    )
            )

            this.subscriptions.add(
                this.props.setBreadcrumb({ key: 'create-new-extension', element: <>Create extension</> })
            )

            this.componentUpdates.next(this.props)
        }

        public componentDidUpdate(): void {
            this.componentUpdates.next(this.props)
        }

        public componentWillUnmount(): void {
            this.subscriptions.unsubscribe()
        }

        public render(): JSX.Element | null {
            let extensionID: string | undefined
            if (
                this.state.publishersOrError !== 'loading' &&
                !isErrorLike(this.state.publishersOrError) &&
                this.state.publisher
            ) {
                const publisher = this.state.publishersOrError.find(publisher => publisher.id === this.state.publisher)
                if (publisher) {
                    extensionID = toExtensionID(publisher, this.state.name)
                }
            }

            return (
                <>
                    <PageTitle title="New extension" />
                    <ModalPage className="registry-new-extension-page mt-4 overflow-hidden">
                        <h2 className="mb-4">
                            <PuzzleIcon className="icon-inline" /> New extension
                        </h2>
                        <div className="mb-3">
                            <a
                                target="_blank"
                                rel="noopener"
                                href="https://docs.sourcegraph.com/extensions/authoring"
                                className="registry-new-extension-page__docs-link"
                            >
                                Learn more
                            </a>{' '}
                            about authoring Sourcegraph extensions{' '}
                            <a target="_blank" rel="noopener" href="https://docs.sourcegraph.com/extensions/authoring">
                                <HelpCircleOutline className="icon-inline" />
                            </a>
                        </div>
                        <Form onSubmit={this.onSubmit}>
                            <RegistryPublisherFormGroup
                                value={this.state.publisher}
                                publishersOrError={this.state.publishersOrError}
                                onChange={this.onPublisherChange}
                                disabled={this.state.creationOrError === 'loading'}
                            />
                            <RegistryExtensionNameFormGroup
                                value={this.state.name}
                                disabled={this.state.creationOrError === 'loading'}
                                onChange={this.onNameChange}
                            />
                            {extensionID && (
                                <div className="form-group d-flex flex-wrap align-items-baseline">
                                    <label
                                        htmlFor="extension-registry-create-extension-page__extensionID"
                                        className="mr-1 mb-0 mt-1"
                                    >
                                        Extension ID:
                                    </label>
                                    <code
                                        id="extension-registry-create-extension-page__extensionID"
                                        className="registry-new-extension-page__extension-id mt-1"
                                    >
                                        <strong>{extensionID}</strong>
                                    </code>
                                </div>
                            )}
                            <button
                                type="submit"
                                disabled={
                                    isErrorLike(this.state.publishersOrError) ||
                                    this.state.publishersOrError === 'loading' ||
                                    this.state.creationOrError === 'loading'
                                }
                                className="btn btn-primary"
                            >
                                {this.state.creationOrError === 'loading' ? (
                                    <LoadingSpinner className="icon-inline" />
                                ) : (
                                    <AddIcon className="icon-inline" />
                                )}{' '}
                                Create extension
                            </button>
                        </Form>
                        {isErrorLike(this.state.creationOrError) && (
                            <ErrorAlert className="mt-3" error={this.state.creationOrError} />
                        )}
                    </ModalPage>
                </>
            )
        }

        private onPublisherChange: React.ChangeEventHandler<HTMLSelectElement> = event =>
            this.setState({ publisher: event.currentTarget.value })

        private onNameChange: React.ChangeEventHandler<HTMLInputElement> = event =>
            this.setState({ name: event.currentTarget.value })

        private onSubmit: React.FormEventHandler<HTMLFormElement> = event => this.submits.next(event)
    }
)
