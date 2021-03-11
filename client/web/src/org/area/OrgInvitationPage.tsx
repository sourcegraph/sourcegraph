import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import * as React from 'react'
import { Link, Redirect } from 'react-router-dom'
import { concat, Observable, Subject, Subscription } from 'rxjs'
import { catchError, concatMap, distinctUntilKeyChanged, map, mapTo, tap, withLatestFrom } from 'rxjs/operators'
import { orgURL } from '..'
import { dataOrThrowErrors, gql } from '../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../shared/src/graphql/schema'
import { asError, ErrorLike, isErrorLike } from '../../../../shared/src/util/errors'
import { refreshAuthenticatedUser, AuthenticatedUser } from '../../auth'
import { withAuthenticatedUser } from '../../auth/withAuthenticatedUser'
import { requestGraphQL } from '../../backend/graphql'
import { Form } from '../../../../branded/src/components/Form'
import { ModalPage } from '../../components/ModalPage'
import { PageTitle } from '../../components/PageTitle'
import { eventLogger } from '../../tracking/eventLogger'
import { userURL } from '../../user'
import { OrgAvatar } from '../OrgAvatar'
import { OrgAreaPageProps } from './OrgArea'
import { ErrorAlert } from '../../components/alerts'
import { OrganizationInvitationResponseType } from '../../../../shared/src/graphql-operations'
import {
    RespondToOrganizationInvitationResult,
    RespondToOrganizationInvitationVariables,
} from '../../graphql-operations'

interface Props extends OrgAreaPageProps {
    authenticatedUser: AuthenticatedUser

    /** Called when the viewer responds to the invitation. */
    onDidRespondToInvitation: () => void
}

interface State {
    /** The result of accepting the invitation. */
    submissionOrError?: 'loading' | null | ErrorLike

    lastResponse?: boolean
}

/**
 * Displays the organization invitation for the current user, if any.
 */
export const OrgInvitationPage = withAuthenticatedUser(
    class OrgInvitationPage extends React.PureComponent<Props, State> {
        public state: State = {}

        private componentUpdates = new Subject<Props>()
        private responses = new Subject<OrganizationInvitationResponseType>()
        private subscriptions = new Subscription()

        public componentDidMount(): void {
            eventLogger.logViewEvent('OrgInvitation')

            const orgChanges = this.componentUpdates.pipe(
                distinctUntilKeyChanged('org'),
                map(({ org }) => org)
            )

            this.subscriptions.add(
                this.responses
                    .pipe(
                        withLatestFrom(orgChanges),
                        concatMap(([responseType, org]) =>
                            concat(
                                [
                                    {
                                        submissionOrError: 'loading',
                                        lastResponse: responseType,
                                    },
                                ],
                                this.respondToOrganizationInvitation({
                                    organizationInvitation: org.viewerPendingInvitation!.id,
                                    responseType,
                                }).pipe(
                                    tap(() => eventLogger.log('OrgInvitationRespondedTo')),
                                    tap(() => this.props.onDidRespondToInvitation()),
                                    concatMap(() => [
                                        // Refresh current user's list of organizations.
                                        refreshAuthenticatedUser(),
                                        { submissionOrError: null },
                                    ]),
                                    catchError(error => [{ submissionOrError: asError(error) }])
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
            if (this.state.submissionOrError === null) {
                // Go to organization profile after accepting invitation, or user's own profile after declining
                // invitation.
                return (
                    <Redirect
                        to={
                            this.state.lastResponse
                                ? orgURL(this.props.org.name)
                                : userURL(this.props.authenticatedUser.username)
                        }
                    />
                )
            }

            return (
                <>
                    <PageTitle title={`Invitation - ${this.props.org.name}`} />
                    {this.props.org.viewerPendingInvitation ? (
                        <ModalPage icon={<OrgAvatar org={this.props.org.name} className="mt-2 mb-3" size="lg" />}>
                            <Form className="text-center">
                                <h3 className="my-0 font-weight-normal">
                                    You've been invited to the{' '}
                                    <Link to={orgURL(this.props.org.name)}>
                                        <strong>{this.props.org.name}</strong>
                                    </Link>{' '}
                                    organization.
                                </h3>
                                <p>
                                    <small className="text-muted">
                                        Invited by{' '}
                                        <Link to={userURL(this.props.org.viewerPendingInvitation.sender.username)}>
                                            {this.props.org.viewerPendingInvitation.sender.username}
                                        </Link>
                                    </small>
                                </p>
                                <div className="mt-3">
                                    <button
                                        type="submit"
                                        className="btn btn-primary mr-sm-2"
                                        disabled={this.state.submissionOrError === 'loading'}
                                        onClick={this.onAcceptInvitation}
                                    >
                                        Join {this.props.org.name}
                                    </button>
                                    <Link className="btn btn-link" to={orgURL(this.props.org.name)}>
                                        Go to {this.props.org.name}'s profile
                                    </Link>
                                </div>
                                <div>
                                    <button
                                        type="button"
                                        className="btn btn-link btn-sm"
                                        disabled={this.state.submissionOrError === 'loading'}
                                        onClick={this.onDeclineInvitation}
                                    >
                                        Decline invitation
                                    </button>
                                </div>
                                {isErrorLike(this.state.submissionOrError) && (
                                    <ErrorAlert className="my-2" error={this.state.submissionOrError} />
                                )}
                                {this.state.submissionOrError === 'loading' && (
                                    <LoadingSpinner className="icon-inline" />
                                )}
                            </Form>
                        </ModalPage>
                    ) : (
                        <div className="alert alert-danger">No pending invitation found.</div>
                    )}
                </>
            )
        }

        private onAcceptInvitation: React.MouseEventHandler<HTMLButtonElement> = event => {
            event.preventDefault()
            this.responses.next(OrganizationInvitationResponseType.ACCEPT)
        }

        private onDeclineInvitation: React.MouseEventHandler<HTMLButtonElement> = event => {
            event.preventDefault()
            this.responses.next(OrganizationInvitationResponseType.REJECT)
        }

        private respondToOrganizationInvitation = (
            args: GQL.IRespondToOrganizationInvitationOnMutationArguments
        ): Observable<void> =>
            requestGraphQL<RespondToOrganizationInvitationResult, RespondToOrganizationInvitationVariables>(
                gql`
                    mutation RespondToOrganizationInvitation(
                        $organizationInvitation: ID!
                        $responseType: OrganizationInvitationResponseType!
                    ) {
                        respondToOrganizationInvitation(
                            organizationInvitation: $organizationInvitation
                            responseType: $responseType
                        ) {
                            alwaysNil
                        }
                    }
                `,
                args
            ).pipe(map(dataOrThrowErrors), mapTo(undefined))
    }
)
