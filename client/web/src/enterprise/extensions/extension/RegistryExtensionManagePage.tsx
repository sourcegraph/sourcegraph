import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { concat, Observable, Subject, Subscription } from 'rxjs'
import { catchError, concatMap, map, tap } from 'rxjs/operators'
import { gql } from '../../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../../shared/src/graphql/schema'
import { asError, createAggregateError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { withAuthenticatedUser } from '../../../auth/withAuthenticatedUser'
import { mutateGraphQL } from '../../../backend/graphql'
import { Form } from '../../../../../branded/src/components/Form'
import { HeroPage } from '../../../components/HeroPage'
import { PageTitle } from '../../../components/PageTitle'
import { toExtensionID } from '../../../extensions/extension/extension'
import { ExtensionAreaRouteContext } from '../../../extensions/extension/ExtensionArea'
import { eventLogger } from '../../../tracking/eventLogger'
import { RegistryExtensionDeleteButton } from './RegistryExtensionDeleteButton'
import { RegistryExtensionNameFormGroup, RegistryPublisherFormGroup } from './RegistryExtensionForm'
import { ErrorAlert } from '../../../components/alerts'
import * as H from 'history'
import { AuthenticatedUser } from '../../../auth'

function updateExtension(
    args: Pick<
        GQL.IUpdateExtensionOnExtensionRegistryMutationArguments,
        Exclude<keyof GQL.IUpdateExtensionOnExtensionRegistryMutationArguments, 'manifest'>
    >
): Observable<GQL.IExtensionRegistryUpdateExtensionResult> {
    return mutateGraphQL(
        gql`
            mutation UpdateRegistryExtension($extension: ID!, $name: String) {
                extensionRegistry {
                    updateExtension(extension: $extension, name: $name) {
                        extension {
                            url
                        }
                    }
                }
            }
        `,
        args
    ).pipe(
        map(({ data, errors }) => {
            if (
                !data ||
                !data.extensionRegistry ||
                !data.extensionRegistry.updateExtension ||
                (errors && errors.length > 0)
            ) {
                throw createAggregateError(errors)
            }
            return data.extensionRegistry.updateExtension
        })
    )
}

interface Props extends ExtensionAreaRouteContext, RouteComponentProps<{}> {
    authenticatedUser: AuthenticatedUser
    history: H.History
}

interface State {
    name?: string

    /** The update result, undefined if not triggered, 'loading', or an error. */
    updateOrError?: 'loading' | GQL.IExtensionRegistryUpdateExtensionResult | ErrorLike
}

/** A page for managing an extension in the extension registry. */
export const RegistryExtensionManagePage = withAuthenticatedUser(
    class RegistryExtensionManagePage extends React.PureComponent<Props, State> {
        public state: State = {}

        private submits = new Subject<React.FormEvent<HTMLFormElement>>()
        private componentUpdates = new Subject<Props>()
        private subscriptions = new Subscription()

        public componentDidMount(): void {
            eventLogger.logViewEvent('RegistryExtensionManage')

            this.subscriptions.add(
                this.submits
                    .pipe(
                        tap(event => event.preventDefault()),
                        concatMap(() =>
                            concat(
                                [{ updateOrError: 'loading' }],
                                updateExtension({
                                    extension: this.props.extension.registryExtension!.id,
                                    name: this.state.name,
                                }).pipe(
                                    tap(result => {
                                        // Redirect to the extension's new URL (if it changed).
                                        if (this.props.extension.registryExtension!.url !== result.extension.url) {
                                            this.props.history.push(result.extension.url + '/-/manage')
                                        }
                                        this.props.onDidUpdateExtension()
                                    }),
                                    map(result => ({ updateOrError: result })),
                                    catchError(error => [{ updateOrError: asError(error) }])
                                )
                            )
                        )
                    )
                    .subscribe(
                        stateUpdate => this.setState(stateUpdate as State),
                        error => console.error(error)
                    )
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
            if (
                !this.props.extension.registryExtension ||
                !this.props.extension.registryExtension.viewerCanAdminister
            ) {
                return (
                    <HeroPage
                        icon={AlertCircleIcon}
                        title="Unauthorized"
                        subtitle="You are not authorized to adminster this extension."
                    />
                )
            }

            const publisher = this.props.extension.registryExtension.publisher
            if (!publisher) {
                return <HeroPage icon={AlertCircleIcon} title="Publisher not found" />
            }

            const extensionName =
                this.state.name === undefined ? this.props.extension.registryExtension.name : this.state.name

            const extensionID = toExtensionID(publisher, extensionName)

            return (
                <div className="registry-extension-manage-page">
                    <PageTitle title="Manage extension" />
                    <h2>Manage extension</h2>
                    <Form onSubmit={this.onSubmit}>
                        <RegistryPublisherFormGroup
                            className="registry-extension-manage-page__input"
                            value={publisher.id}
                            publishersOrError={[publisher]}
                            disabled={true}
                        />
                        <RegistryExtensionNameFormGroup
                            className="registry-extension-manage-page__input"
                            value={extensionName}
                            onChange={this.onNameChange}
                            disabled={this.state.updateOrError === 'loading'}
                        />
                        {extensionID &&
                            this.state.name &&
                            this.state.name !== this.props.extension.registryExtension.name && (
                                <div className="alert alert-primary">
                                    Extension will be renamed. New extension ID:{' '}
                                    <code id="registry-extension__extensionID">
                                        <strong>{extensionID}</strong>
                                    </code>
                                </div>
                            )}
                        <button
                            type="submit"
                            disabled={this.state.updateOrError === 'loading'}
                            className="btn btn-primary"
                        >
                            {this.state.updateOrError === 'loading' ? (
                                <LoadingSpinner className="icon-inline" />
                            ) : (
                                'Update extension'
                            )}
                        </button>
                    </Form>
                    {isErrorLike(this.state.updateOrError) && <ErrorAlert error={this.state.updateOrError} />}
                    <div className="card mt-5 registry-extension-manage-page__other-actions">
                        <div className="card-header">Other actions</div>
                        <div className="card-body">
                            <Link
                                to={`${this.props.extension.registryExtension.url}/-/releases/new`}
                                className="btn btn-success mr-2"
                            >
                                Publish new release
                            </Link>
                            <RegistryExtensionDeleteButton
                                extension={this.props.extension.registryExtension}
                                onDidUpdate={this.onDidDelete}
                            />
                        </div>
                    </div>
                </div>
            )
        }

        private onNameChange: React.ChangeEventHandler<HTMLInputElement> = event =>
            this.setState({ name: event.currentTarget.value })

        private onSubmit: React.FormEventHandler<HTMLFormElement> = event => this.submits.next(event)

        private onDidDelete = (): void => {
            this.props.history.push('/extensions')
            this.props.onDidUpdateExtension()
        }
    }
)
