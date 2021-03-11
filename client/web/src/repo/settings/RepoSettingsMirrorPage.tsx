import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import CheckIcon from 'mdi-react/CheckIcon'
import LockIcon from 'mdi-react/LockIcon'
import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { interval, Subject, Subscription } from 'rxjs'
import { catchError, switchMap, tap } from 'rxjs/operators'
import * as GQL from '../../../../shared/src/graphql/schema'
import { FeedbackText } from '../../components/FeedbackText'
import { PageTitle } from '../../components/PageTitle'
import { Timestamp } from '../../components/time/Timestamp'
import { checkMirrorRepositoryConnection, updateMirrorRepository } from '../../site-admin/backend'
import { eventLogger } from '../../tracking/eventLogger'
import { DirectImportRepoAlert } from '../DirectImportRepoAlert'
import { fetchSettingsAreaRepository } from './backend'
import { ActionContainer, BaseActionContainer } from './components/ActionContainer'
import { ErrorAlert } from '../../components/alerts'
import { asError } from '../../../../shared/src/util/errors'
import * as H from 'history'
import { SettingsAreaRepositoryFields } from '../../graphql-operations'

interface UpdateMirrorRepositoryActionContainerProps {
    repo: SettingsAreaRepositoryFields
    onDidUpdateRepository: () => void
    disabled: boolean
    disabledReason: string | undefined
    history: H.History
}

class UpdateMirrorRepositoryActionContainer extends React.PureComponent<UpdateMirrorRepositoryActionContainerProps> {
    private componentUpdates = new Subject<UpdateMirrorRepositoryActionContainerProps>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        this.subscriptions.add(
            interval(3000).subscribe(() => {
                this.props.onDidUpdateRepository()
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
        let title: React.ReactFragment
        let description: React.ReactFragment
        let buttonLabel: React.ReactFragment
        let buttonDisabled = false
        let info: React.ReactNode
        if (this.props.repo.mirrorInfo.cloneInProgress) {
            title = 'Cloning in progress...'
            description =
                <code>{this.props.repo.mirrorInfo.cloneProgress}</code> ||
                'This repository is currently being cloned from its remote repository.'
            buttonLabel = (
                <span>
                    <LoadingSpinner className="icon-inline" /> Cloning...
                </span>
            )
            buttonDisabled = true
            info = <DirectImportRepoAlert className="action-container__alert" />
        } else if (this.props.repo.mirrorInfo.cloned) {
            const updateSchedule = this.props.repo.mirrorInfo.updateSchedule
            title = (
                <>
                    <div>
                        Last refreshed:{' '}
                        {this.props.repo.mirrorInfo.updatedAt ? (
                            <Timestamp date={this.props.repo.mirrorInfo.updatedAt} />
                        ) : (
                            'unknown'
                        )}{' '}
                    </div>
                    {updateSchedule && (
                        <div>
                            Next scheduled update <Timestamp date={updateSchedule.due} /> (position{' '}
                            {updateSchedule.index + 1} out of {updateSchedule.total} in the schedule)
                        </div>
                    )}
                    {this.props.repo.mirrorInfo.updateQueue && !this.props.repo.mirrorInfo.updateQueue.updating && (
                        <div>
                            Queued for update (position {this.props.repo.mirrorInfo.updateQueue.index + 1} out of{' '}
                            {this.props.repo.mirrorInfo.updateQueue.total} in the queue)
                        </div>
                    )}
                </>
            )
            if (!updateSchedule) {
                description = 'This repository is automatically updated when accessed by a user.'
            } else {
                description =
                    'This repository is automatically updated from its remote repository periodically and when accessed by a user.'
            }
            buttonLabel = 'Refresh now'
        } else {
            title = 'Clone this repository'
            description = 'This repository has not yet been cloned from its remote repository.'
            buttonLabel = 'Clone now'
        }

        return (
            <ActionContainer
                title={title}
                description={<div>{description}</div>}
                buttonLabel={buttonLabel}
                buttonDisabled={buttonDisabled || this.props.disabled}
                buttonSubtitle={this.props.disabledReason}
                flashText="Added to queue"
                info={info}
                run={this.updateMirrorRepository}
                history={this.props.history}
            />
        )
    }

    private updateMirrorRepository = async (): Promise<void> => {
        await updateMirrorRepository({ repository: this.props.repo.id }).toPromise()
        this.props.onDidUpdateRepository()
    }
}

interface CheckMirrorRepositoryConnectionActionContainerProps {
    repo: SettingsAreaRepositoryFields
    onDidUpdateReachability: (reachable: boolean | undefined) => void
    history: H.History
}

interface CheckMirrorRepositoryConnectionActionContainerState {
    loading: boolean
    result?: GQL.ICheckMirrorRepositoryConnectionResult | null
    errorDescription?: string
}

class CheckMirrorRepositoryConnectionActionContainer extends React.PureComponent<
    CheckMirrorRepositoryConnectionActionContainerProps,
    CheckMirrorRepositoryConnectionActionContainerState
> {
    public state: CheckMirrorRepositoryConnectionActionContainerState = { loading: false }

    private checkRequests = new Subject<void>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        this.subscriptions.add(
            this.checkRequests
                .pipe(
                    tap(() => {
                        this.setState({ errorDescription: undefined, result: undefined, loading: true })
                        this.props.onDidUpdateReachability(undefined)
                    }),
                    switchMap(() =>
                        checkMirrorRepositoryConnection({ repository: this.props.repo.id }).pipe(
                            catchError(error => {
                                this.setState({
                                    errorDescription: asError(error).message,
                                    result: undefined,
                                    loading: false,
                                })
                                this.props.onDidUpdateReachability(false)
                                return []
                            })
                        )
                    )
                )
                .subscribe(
                    result => {
                        this.setState({ result, loading: false })
                        this.props.onDidUpdateReachability(result.error === null)
                    },
                    error => console.log(error)
                )
        )

        // Run the check upon initial mount, so the user sees the information without needing to click.
        this.checkRequests.next()
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <BaseActionContainer
                title="Check connection to remote repository"
                description={<span>Diagnose problems cloning or updating from the remote repository.</span>}
                action={
                    <button
                        type="button"
                        className="btn btn-primary"
                        disabled={this.state.loading}
                        onClick={this.checkMirrorRepositoryConnection}
                    >
                        Check connection
                    </button>
                }
                details={
                    <>
                        {this.state.errorDescription && (
                            <ErrorAlert className="action-container__alert" error={this.state.errorDescription} />
                        )}
                        {this.state.loading && (
                            <div className="alert alert-primary action-container__alert">
                                <LoadingSpinner className="icon-inline" /> Checking connection...
                            </div>
                        )}
                        {this.state.result &&
                            (this.state.result.error === null ? (
                                <div className="alert alert-success action-container__alert">
                                    <CheckIcon className="icon-inline" /> The remote repository is reachable.
                                </div>
                            ) : (
                                <div className="alert alert-danger action-container__alert">
                                    <p>The remote repository is unreachable. Logs follow.</p>
                                    <div>
                                        <pre className="check-mirror-repository-connection-action-container__log">
                                            <code>{this.state.result.error}</code>
                                        </pre>
                                    </div>
                                </div>
                            ))}
                    </>
                }
            />
        )
    }

    private checkMirrorRepositoryConnection = (): void => this.checkRequests.next()
}

interface RepoSettingsMirrorPageProps extends RouteComponentProps<{}> {
    repo: SettingsAreaRepositoryFields
    history: H.History
}

interface RepoSettingsMirrorPageState {
    /**
     * The repository object, refreshed after we make changes that modify it.
     */
    repo: SettingsAreaRepositoryFields

    /**
     * Whether the repository connection check reports that the repository is reachable.
     */
    reachable?: boolean

    loading: boolean
    error?: string
}

/**
 * The repository settings mirror page.
 */
export class RepoSettingsMirrorPage extends React.PureComponent<
    RepoSettingsMirrorPageProps,
    RepoSettingsMirrorPageState
> {
    private repoUpdates = new Subject<void>()
    private subscriptions = new Subscription()

    constructor(props: RepoSettingsMirrorPageProps) {
        super(props)

        this.state = {
            loading: false,
            repo: props.repo,
        }
    }

    public componentDidMount(): void {
        eventLogger.logViewEvent('RepoSettingsMirror')

        this.subscriptions.add(
            this.repoUpdates.pipe(switchMap(() => fetchSettingsAreaRepository(this.props.repo.name))).subscribe(
                repo => this.setState({ repo }),
                error => this.setState({ error: asError(error).message })
            )
        )
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <div className="repo-settings-mirror-page">
                <PageTitle title="Mirror settings" />
                <h2>Mirroring and cloning</h2>
                {this.state.loading && <LoadingSpinner className="icon-inline" />}
                {this.state.error && <ErrorAlert error={this.state.error} />}
                <div className="form-group">
                    <label>
                        Remote repository URL{' '}
                        <small className="text-info">
                            <LockIcon className="icon-inline" /> Only visible to site admins
                        </small>
                    </label>
                    <input
                        className="form-control"
                        value={this.props.repo.mirrorInfo.remoteURL || '(unknown)'}
                        readOnly={true}
                    />
                    {this.state.repo.viewerCanAdminister && (
                        <small className="form-text text-muted">
                            Configure repository mirroring in{' '}
                            <Link to="/site-admin/external-services">external services</Link>.
                        </small>
                    )}
                </div>
                <UpdateMirrorRepositoryActionContainer
                    repo={this.state.repo}
                    onDidUpdateRepository={this.onDidUpdateRepository}
                    disabled={typeof this.state.reachable === 'boolean' && !this.state.reachable}
                    disabledReason={
                        typeof this.state.reachable === 'boolean' && !this.state.reachable ? 'Not reachable' : undefined
                    }
                    history={this.props.history}
                />
                <CheckMirrorRepositoryConnectionActionContainer
                    repo={this.state.repo}
                    onDidUpdateReachability={this.onDidUpdateReachability}
                    history={this.props.history}
                />
                {typeof this.state.reachable === 'boolean' && !this.state.reachable && (
                    <div className="alert alert-info repo-settings-mirror-page__troubleshooting">
                        Problems cloning or updating this repository?
                        <ul className="repo-settings-mirror-page__steps">
                            <li className="repo-settings-mirror-page__step">
                                Inspect the <strong>Check connection</strong> error log output to see why the remote
                                repository is not reachable.
                            </li>
                            <li className="repo-settings-mirror-page__step">
                                <code>
                                    <strong>No ECDSA host key is known ... Host key verification failed?</strong>
                                </code>{' '}
                                See{' '}
                                <Link to="/help/admin/repo/auth#ssh-authentication-config-keys-known-hosts">
                                    SSH repository authentication documentation
                                </Link>{' '}
                                for how to provide an SSH <code>known_hosts</code> file with the remote host's SSH host
                                key.
                            </li>
                            <li className="repo-settings-mirror-page__step">
                                Consult <Link to="/help/admin/repo/add">Sourcegraph repositories documentation</Link>{' '}
                                for resolving other authentication issues (such as HTTPS certificates and SSH keys).
                            </li>
                            <li className="repo-settings-mirror-page__step">
                                <FeedbackText headerText="Questions?" />
                            </li>
                        </ul>
                    </div>
                )}
            </div>
        )
    }

    private onDidUpdateRepository = (): void => {
        this.repoUpdates.next()
    }

    private onDidUpdateReachability = (reachable: boolean | undefined): void => this.setState({ reachable })
}
