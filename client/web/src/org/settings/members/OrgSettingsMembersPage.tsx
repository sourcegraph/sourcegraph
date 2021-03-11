import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { Observable, Subject, Subscription } from 'rxjs'
import { catchError, distinctUntilChanged, filter, map, startWith, switchMap, tap } from 'rxjs/operators'
import { gql } from '../../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../../shared/src/graphql/schema'
import { asError, createAggregateError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { queryGraphQL } from '../../../backend/graphql'
import { FilteredConnection } from '../../../components/FilteredConnection'
import { PageTitle } from '../../../components/PageTitle'
import { eventLogger } from '../../../tracking/eventLogger'
import { userURL } from '../../../user'
import { removeUserFromOrganization } from '../../backend'
import { InviteForm } from './InviteForm'
import { OrgAreaPageProps } from '../../area/OrgArea'
import { ErrorAlert } from '../../../components/alerts'
import * as H from 'history'
import { AuthenticatedUser } from '../../../auth'
import { OrgAreaOrganizationFields } from '../../../graphql-operations'

interface UserNodeProps {
    /** The user to display in this list item. */
    node: GQL.IUser

    /** The organization being displayed. */
    org: OrgAreaOrganizationFields

    /** The currently authenticated user. */
    authenticatedUser: AuthenticatedUser | null

    /** Called when the user is updated by an action in this list item. */
    onDidUpdate?: () => void
    history: H.History
}

interface UserNodeState {
    /** Undefined means in progress, null means done or not started. */
    removalOrError?: null | ErrorLike
}

class UserNode extends React.PureComponent<UserNodeProps, UserNodeState> {
    public state: UserNodeState = {
        removalOrError: null,
    }

    private removes = new Subject<void>()
    private subscriptions = new Subscription()

    private get isSelf(): boolean {
        return this.props.authenticatedUser !== null && this.props.node.id === this.props.authenticatedUser.id
    }

    public componentDidMount(): void {
        this.subscriptions.add(
            this.removes
                .pipe(
                    filter(() =>
                        window.confirm(
                            this.isSelf ? 'Leave the organization?' : `Remove the user ${this.props.node.username}?`
                        )
                    ),
                    switchMap(() =>
                        removeUserFromOrganization({ user: this.props.node.id, organization: this.props.org.id }).pipe(
                            catchError(error => [asError(error)]),
                            map(removalOrError => ({ removalOrError: removalOrError || null })),
                            tap(() => {
                                if (this.props.onDidUpdate) {
                                    this.props.onDidUpdate()
                                }
                            }),
                            startWith<Pick<UserNodeState, 'removalOrError'>>({ removalOrError: undefined })
                        )
                    )
                )
                .subscribe(
                    stateUpdate => {
                        this.setState(stateUpdate)
                    },
                    error => console.error(error)
                )
        )
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        const loading = this.state.removalOrError === undefined
        return (
            <li className="list-group-item py-2" data-test-username={this.props.node.username}>
                <div className="d-flex align-items-center justify-content-between">
                    <div>
                        <Link to={userURL(this.props.node.username)}>
                            <strong>{this.props.node.username}</strong>
                        </Link>
                        {this.props.node.displayName && (
                            <>
                                <br />
                                <span className="text-muted">{this.props.node.displayName}</span>
                            </>
                        )}
                    </div>
                    <div className="site-admin-detail-list__actions">
                        {this.props.authenticatedUser && this.props.org.viewerCanAdminister && (
                            <button
                                type="button"
                                className="btn btn-secondary btn-sm site-admin-detail-list__action test-remove-org-member"
                                onClick={this.remove}
                                disabled={loading}
                            >
                                {this.isSelf ? 'Leave organization' : 'Remove from organization'}
                            </button>
                        )}
                    </div>
                </div>
                {isErrorLike(this.state.removalOrError) && (
                    <ErrorAlert className="mt-2" error={this.state.removalOrError} />
                )}
            </li>
        )
    }

    private remove = (): void => this.removes.next()
}

interface Props extends OrgAreaPageProps, RouteComponentProps<{}> {
    history: H.History
}

interface State {
    /**
     * Whether the viewer can administer this org. This is updated whenever a member is added or removed, so that
     * we can detect if the currently authenticated user is no longer able to administer the org (e.g., because
     * they removed themselves and they are not a site admin).
     */
    viewerCanAdminister: boolean
}

/**
 * The organizations members page
 */
export class OrgSettingsMembersPage extends React.PureComponent<Props, State> {
    private componentUpdates = new Subject<Props>()
    private userUpdates = new Subject<void>()
    private subscriptions = new Subscription()

    constructor(props: Props) {
        super(props)
        this.state = { viewerCanAdminister: props.org.viewerCanAdminister }
    }

    public componentDidMount(): void {
        eventLogger.logViewEvent('OrgMembers')

        this.subscriptions.add(
            this.componentUpdates
                .pipe(
                    map(props => props.org),
                    distinctUntilChanged((a, b) => a.id === b.id)
                )
                .subscribe(org => {
                    this.setState({ viewerCanAdminister: org.viewerCanAdminister })
                    this.userUpdates.next()
                })
        )
    }

    public componentDidUpdate(): void {
        this.componentUpdates.next(this.props)
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        const nodeProps: Omit<UserNodeProps, 'node'> = {
            org: { ...this.props.org, viewerCanAdminister: this.state.viewerCanAdminister },
            authenticatedUser: this.props.authenticatedUser,
            onDidUpdate: this.onDidUpdateUser,
            history: this.props.history,
        }

        return (
            <div className="org-settings-members-page">
                <PageTitle title={`Members - ${this.props.org.name}`} />
                {this.state.viewerCanAdminister && (
                    <InviteForm
                        orgID={this.props.org.id}
                        authenticatedUser={this.props.authenticatedUser}
                        onOrganizationUpdate={this.props.onOrganizationUpdate}
                        onDidUpdateOrganizationMembers={this.onDidUpdateOrganizationMembers}
                    />
                )}
                <FilteredConnection<GQL.IUser, Omit<UserNodeProps, 'node'>>
                    className="list-group list-group-flush mt-3 test-org-members"
                    noun="member"
                    pluralNoun="members"
                    queryConnection={this.fetchOrgMembers}
                    nodeComponent={UserNode}
                    nodeComponentProps={nodeProps}
                    noShowMore={true}
                    hideSearch={true}
                    updates={this.userUpdates}
                    history={this.props.history}
                    location={this.props.location}
                />
            </div>
        )
    }

    private onDidUpdateUser = (): void => this.userUpdates.next()

    private onDidUpdateOrganizationMembers = (): void => this.userUpdates.next()

    private fetchOrgMembers = (): Observable<GQL.IUserConnection> =>
        queryGraphQL(
            gql`
                query OrganizationMembers($id: ID!) {
                    node(id: $id) {
                        ... on Org {
                            viewerCanAdminister
                            members {
                                nodes {
                                    id
                                    username
                                    displayName
                                    avatarURL
                                }
                                totalCount
                            }
                        }
                    }
                }
            `,
            { id: this.props.org.id }
        ).pipe(
            map(({ data, errors }) => {
                if (!data || !data.node) {
                    this.setState({ viewerCanAdminister: false })
                    throw createAggregateError(errors)
                }
                const org = data.node as GQL.IOrg
                if (!org.members) {
                    this.setState({ viewerCanAdminister: false })
                    throw createAggregateError(errors)
                }
                this.setState({ viewerCanAdminister: org.viewerCanAdminister })
                return org.members
            })
        )
}
