import * as React from 'react'

import { Navigate } from 'react-router-dom'
import { concat, type Observable, Subject, Subscription } from 'rxjs'
import { catchError, concatMap, distinctUntilKeyChanged, map, tap, withLatestFrom } from 'rxjs/operators'

import { asError, type ErrorLike, isErrorLike, logger } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { OrganizationInvitationResponseType } from '@sourcegraph/shared/src/graphql-operations'
import { LoadingSpinner, Button, Link, Alert, H3, Text, ErrorAlert, Form } from '@sourcegraph/wildcard'

import { orgURL } from '..'
import { refreshAuthenticatedUser, type AuthenticatedUser } from '../../auth'
import { withAuthenticatedUser } from '../../auth/withAuthenticatedUser'
import { requestGraphQL } from '../../backend/graphql'
import { ModalPage } from '../../components/ModalPage'
import { PageTitle } from '../../components/PageTitle'
import type {
    RespondToOrganizationInvitationResult,
    RespondToOrganizationInvitationVariables,
} from '../../graphql-operations'
import { eventLogger } from '../../tracking/eventLogger'
import { userURL } from '../../user'
import { OrgAvatar } from '../OrgAvatar'

import type { OrgAreaRouteContext } from './OrgArea'

interface Props extends OrgAreaRouteContext {
    authenticatedUser: AuthenticatedUser

    /** Called when the viewer responds to the invitation. */
    onDidRespondToInvitation: (accepted: boolean) => void
}

interface State {
    /** The result of accepting the invitation. */
    submissionOrError?: 'loading' | null | ErrorLike

    lastResponse?: boolean
}

/**
 * Displays the organization invitation for the current user, if any.
 */
export const OrgInvitationPageLegacy = withAuthenticatedUser(
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
                                    tap(() =>
                                        this.props.onDidRespondToInvitation(
                                            responseType === OrganizationInvitationResponseType.ACCEPT
                                        )
                                    ),
                                    concatMap(() => concat(refreshAuthenticatedUser(), [{ submissionOrError: null }])),
                                    catchError(error => [{ submissionOrError: asError(error) }])
                                )
                            )
                        )
                    )
                    .subscribe(
                        stateUpdate => this.setState(stateUpdate as State),
                        error => logger.error(error)
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
                    <Navigate
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
                                <H3 className="my-0 font-weight-normal">
                                    You've been invited to the{' '}
                                    <Link to={orgURL(this.props.org.name)}>
                                        <strong>{this.props.org.name}</strong>
                                    </Link>{' '}
                                    organization.
                                </H3>
                                <Text>
                                    <small className="text-muted">
                                        Invited by{' '}
                                        <Link to={userURL(this.props.org.viewerPendingInvitation.sender.username)}>
                                            {this.props.org.viewerPendingInvitation.sender.username}
                                        </Link>
                                    </small>
                                </Text>
                                <div className="mt-3">
                                    <Button
                                        type="submit"
                                        className="mr-sm-2"
                                        disabled={this.state.submissionOrError === 'loading'}
                                        onClick={this.onAcceptInvitation}
                                        variant="primary"
                                    >
                                        Join {this.props.org.name}
                                    </Button>
                                    <Button to={orgURL(this.props.org.name)} variant="link" as={Link}>
                                        Go to {this.props.org.name}'s profile
                                    </Button>
                                </div>
                                <div>
                                    <Button
                                        disabled={this.state.submissionOrError === 'loading'}
                                        onClick={this.onDeclineInvitation}
                                        variant="link"
                                        size="sm"
                                    >
                                        Decline invitation
                                    </Button>
                                </div>
                                {isErrorLike(this.state.submissionOrError) && (
                                    <ErrorAlert className="my-2" error={this.state.submissionOrError} />
                                )}
                                {this.state.submissionOrError === 'loading' && <LoadingSpinner />}
                            </Form>
                        </ModalPage>
                    ) : (
                        <Alert className="align-self-start mt-4 mx-auto" variant="danger">
                            No pending invitation found.
                        </Alert>
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

        private respondToOrganizationInvitation = (args: RespondToOrganizationInvitationVariables): Observable<void> =>
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
            ).pipe(
                map(dataOrThrowErrors),
                map(() => undefined)
            )
    }
)
