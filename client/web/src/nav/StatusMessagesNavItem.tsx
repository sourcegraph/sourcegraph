import CloudAlertIcon from 'mdi-react/CloudAlertIcon'
import CloudCheckIcon from 'mdi-react/CloudCheckIcon'
import CloudSyncIcon from 'mdi-react/CloudSyncIcon'
import React from 'react'
import { ButtonDropdown, DropdownMenu, DropdownToggle } from 'reactstrap'
import { Observable, Subscription } from 'rxjs'
import { catchError, map, repeatWhen, delay, distinctUntilChanged } from 'rxjs/operators'
import { Link } from '../../../shared/src/components/Link'
import { dataOrThrowErrors, gql } from '../../../shared/src/graphql/graphql'
import { asError, ErrorLike, isErrorLike } from '../../../shared/src/util/errors'
import { requestGraphQL } from '../backend/graphql'
import classNames from 'classnames'
import { ErrorAlert } from '../components/alerts'
import * as H from 'history'
import { repeatUntil } from '../../../shared/src/util/rxjs/repeatUntil'
import { StatusMessagesResult, StatusMessageFields } from '../graphql-operations'
import { isEqual } from 'lodash'

export function fetchAllStatusMessages(): Observable<StatusMessagesResult['statusMessages']> {
    return requestGraphQL<StatusMessagesResult>(
        gql`
            query StatusMessages {
                statusMessages {
                    ...StatusMessageFields
                }
            }

            fragment StatusMessageFields on StatusMessage {
                __typename

                ... on CloningProgress {
                    message
                }

                ... on SyncError {
                    message
                }

                ... on ExternalServiceSyncError {
                    message
                    externalService {
                        id
                        displayName
                    }
                }
            }
        `
    ).pipe(
        map(dataOrThrowErrors),
        map(data => data.statusMessages)
    )
}

type EntryType = 'warning' | 'success' | 'progress'

interface StatusMessageEntryProps {
    title: string
    text: string
    showLink?: boolean
    linkTo: string
    linkText: string
    entryType: EntryType
    linkOnClick: (event: React.MouseEvent<HTMLAnchorElement, MouseEvent>) => void
}

function entryIcon(entryType: EntryType): JSX.Element {
    switch (entryType) {
        case 'warning':
            return <CloudAlertIcon className="icon-inline mr-1" />
        case 'success':
            return <CloudCheckIcon className="icon-inline mr-1" />
        case 'progress':
            return <CloudSyncIcon className="icon-inline mr-1" />
    }
}

const StatusMessagesNavItemEntry: React.FunctionComponent<StatusMessageEntryProps> = props => (
    <div
        key={props.text}
        className={classNames(
            'status-messages-nav-item__entry mb-3',
            props.entryType && `status-messages-nav-item__entry--border-${props.entryType}`
        )}
    >
        <h4>
            {entryIcon(props.entryType)}
            {props.title}
        </h4>
        <p>{props.text}</p>
        {props.showLink && (
            <p className="status-messages-nav-item__entry-link">
                <Link to={props.linkTo} onClick={props.linkOnClick}>
                    {props.linkText}
                </Link>
            </p>
        )}
    </div>
)

interface Props {
    fetchMessages?: () => Observable<StatusMessagesResult['statusMessages']>
    isSiteAdmin: boolean
    history: H.History
}

interface State {
    messagesOrError: StatusMessagesResult['statusMessages'] | ErrorLike
    isOpen: boolean
}

const REFRESH_INTERVAL_AFTER_ERROR_MS = 3000
const REFRESH_INTERVAL_MS = 10000

/**
 * Displays a status icon in the navbar reflecting the completion of backend
 * tasks such as repository cloning, and exposes a dropdown menu containing
 * more information on these tasks.
 */
export class StatusMessagesNavItem extends React.PureComponent<Props, State> {
    private subscriptions = new Subscription()

    public state: State = { isOpen: false, messagesOrError: [] }

    private toggleIsOpen = (): void => this.setState(previousState => ({ isOpen: !previousState.isOpen }))

    public componentDidMount(): void {
        this.subscriptions.add(
            (this.props.fetchMessages ?? fetchAllStatusMessages)()
                .pipe(
                    catchError(error => [asError(error) as ErrorLike]),
                    // Poll on REFRESH_INTERVAL_MS, or REFRESH_INTERVAL_AFTER_ERROR_MS if there is an error.
                    repeatUntil(messagesOrError => isErrorLike(messagesOrError), { delay: REFRESH_INTERVAL_MS }),
                    repeatWhen(completions => completions.pipe(delay(REFRESH_INTERVAL_AFTER_ERROR_MS))),
                    distinctUntilChanged((a, b) => isEqual(a, b))
                )
                .subscribe(messagesOrError => this.setState({ messagesOrError }))
        )
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    private renderMessage(message: StatusMessageFields, key: number): JSX.Element | null {
        switch (message.__typename) {
            case 'CloningProgress':
                return (
                    <StatusMessagesNavItemEntry
                        key={key}
                        title="Repositories cloning"
                        text={message.message}
                        showLink={this.props.isSiteAdmin}
                        linkTo="/site-admin/external-services"
                        linkText="Configure synced repositories"
                        linkOnClick={this.toggleIsOpen}
                        entryType="progress"
                    />
                )
            case 'ExternalServiceSyncError':
                return (
                    <StatusMessagesNavItemEntry
                        key={key}
                        title={`Syncing repositories from external service "${message.externalService.displayName}" failed:`}
                        text={message.message}
                        showLink={this.props.isSiteAdmin}
                        linkTo={`/site-admin/external-services/${message.externalService.id}`}
                        linkText={`Edit "${message.externalService.displayName}"`}
                        linkOnClick={this.toggleIsOpen}
                        entryType="warning"
                    />
                )
            case 'SyncError':
                return (
                    <StatusMessagesNavItemEntry
                        key={key}
                        title="Syncing repositories failed:"
                        text={message.message}
                        showLink={this.props.isSiteAdmin}
                        linkTo="/site-admin/external-services"
                        linkText="Configure synced repositories"
                        linkOnClick={this.toggleIsOpen}
                        entryType="warning"
                    />
                )
        }
    }

    private renderIcon(): JSX.Element | null {
        if (isErrorLike(this.state.messagesOrError)) {
            return <CloudAlertIcon className="icon-inline" />
        }
        if (this.state.messagesOrError.some(({ __typename }) => __typename === 'ExternalServiceSyncError')) {
            return (
                <CloudAlertIcon
                    className="icon-inline"
                    data-tooltip={this.state.isOpen ? undefined : 'Syncing repositories failed!'}
                />
            )
        }
        if (this.state.messagesOrError.some(({ __typename }) => __typename === 'CloningProgress')) {
            return (
                <CloudSyncIcon
                    className="icon-inline"
                    data-tooltip={this.state.isOpen ? undefined : 'Cloning repositories...'}
                />
            )
        }
        return (
            <CloudCheckIcon
                className="icon-inline"
                data-tooltip={this.state.isOpen ? undefined : 'Repositories up to date'}
            />
        )
    }

    public render(): JSX.Element | null {
        return (
            <ButtonDropdown
                isOpen={this.state.isOpen}
                toggle={this.toggleIsOpen}
                className="nav-link py-0 px-0 percy-hide chromatic-ignore"
            >
                <DropdownToggle caret={false} className="btn btn-link" nav={true}>
                    {this.renderIcon()}
                </DropdownToggle>

                <DropdownMenu right={true} className="status-messages-nav-item__dropdown-menu">
                    <h3>Code host status</h3>
                    <div className="status-messages-nav-item__dropdown-menu-content">
                        {isErrorLike(this.state.messagesOrError) ? (
                            <ErrorAlert
                                className="status-messages-nav-item__entry"
                                prefix="Failed to load status messages"
                                error={this.state.messagesOrError}
                            />
                        ) : this.state.messagesOrError.length > 0 ? (
                            this.state.messagesOrError.map((message, index) => this.renderMessage(message, index))
                        ) : (
                            <StatusMessagesNavItemEntry
                                title="Repositories up to date"
                                text="All repositories hosted on the configured code hosts are synced."
                                showLink={this.props.isSiteAdmin}
                                linkTo="/site-admin/external-services"
                                linkText="Manage repositories"
                                linkOnClick={this.toggleIsOpen}
                                entryType="success"
                            />
                        )}
                    </div>
                </DropdownMenu>
            </ButtonDropdown>
        )
    }
}
