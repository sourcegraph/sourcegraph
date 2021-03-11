import React, { FormEvent, useCallback, useEffect, useState } from 'react'
import classNames from 'classnames'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import { RouteComponentProps } from 'react-router'
import { PageTitle } from '../../../components/PageTitle'
import { CheckboxRepositoryNode } from './RepositoryNode'
import { Form } from '../../../../../branded/src/components/Form'
import { Link } from '../../../../../shared/src/components/Link'
import { ExternalServiceKind, ExternalServicesResult, Maybe } from '../../../graphql-operations'
import {
    queryExternalServices,
    setExternalServiceRepos,
    listAffiliatedRepositories,
} from '../../../components/externalServices/backend'
import { ErrorAlert } from '../../../components/alerts'
import ChevronLeftIcon from 'mdi-react/ChevronLeftIcon'
import ChevronRightIcon from 'mdi-react/ChevronRightIcon'
import { repeatUntil } from '../../../../../shared/src/util/rxjs/repeatUntil'
import { LoaderButton } from '../../../components/LoaderButton'
import { UserRepositoriesUpdateProps } from '../../../util'

interface Props extends RouteComponentProps, TelemetryProps, UserRepositoriesUpdateProps {
    userID: string
    routingPrefix: string
}

interface Repo {
    name: string
    codeHost: Maybe<{ kind: ExternalServiceKind; id: string; displayName: string }>
    private: boolean
}

interface GitHubConfig {
    repos: string[]
    token: 'REDACTED'
    url: string
}
interface GitLabConfig {
    projectQuery: string[]
    projects: { name: string }[]
    token: 'REDACTED'
    url: string
}

const PER_PAGE = 25
const SIX_SECONDS = 6000
const EIGHT_SECONDS = 8000

// initial state constants
const emptyRepos: Repo[] = []
const initialRepoState = {
    repos: emptyRepos,
    loading: false,
    loaded: false,
    error: '',
}
const selectionMap = new Map<string, Repo>()
const emptyHosts: ExternalServicesResult['externalServices']['nodes'] = []
const emptyRepoNames: string[] = []
const initialCodeHostState = {
    hosts: emptyHosts,
    loaded: false,
    configuredRepos: emptyRepoNames,
}

type initialFetchingReposState = undefined | 'loading' | 'slow' | 'slower'
const isLoading = (status: initialFetchingReposState): boolean => {
    if (!status) {
        return false
    }

    return ['loading', 'slow', 'slower'].includes(status)
}

/**
 * A page to manage the repositories a user syncs from their connected code hosts.
 */
export const UserSettingsManageRepositoriesPage: React.FunctionComponent<Props> = ({
    history,
    userID,
    routingPrefix,
    telemetryService,
    onUserRepositoriesUpdate,
}) => {
    useEffect(() => {
        telemetryService.logViewEvent('UserSettingsRepositories')
    }, [telemetryService])

    // set up state hooks
    const [repoState, setRepoState] = useState(initialRepoState)
    const [selectionState, setSelectionState] = useState({ repos: selectionMap, loaded: false, radio: '' })
    const [currentPage, setPage] = useState(1)
    const [query, setQuery] = useState('')
    const [codeHostFilter, setCodeHostFilter] = useState('')
    const [codeHosts, setCodeHosts] = useState(initialCodeHostState)
    const [showTextArea, setShowTextArea] = useState(false)
    const [fetchingRepos, setFetchingRepos] = useState<initialFetchingReposState>()

    const toggleTextArea = useCallback(() => setShowTextArea(!showTextArea), [showTextArea])

    useCallback(() => {
        // first we should load code hosts
        if (!codeHosts.loaded) {
            const codeHostSubscription = queryExternalServices({
                first: null,
                after: null,
                namespace: userID,
            }).subscribe(result => {
                const selected: string[] = []
                for (const host of result.nodes) {
                    const cfg = JSON.parse(host.config) as GitHubConfig | GitLabConfig
                    switch (host.kind) {
                        case ExternalServiceKind.GITLAB: {
                            const gitLabCfg = cfg as GitLabConfig
                            if (gitLabCfg.projects !== undefined) {
                                gitLabCfg.projects.map(project => {
                                    selected.push(project.name)
                                })
                            }
                            break
                        }

                        case ExternalServiceKind.GITHUB: {
                            const gitHubCfg = cfg as GitHubConfig
                            if (gitHubCfg.repos !== undefined) {
                                selected.push(...gitHubCfg.repos)
                            }
                            break
                        }
                    }
                }
                setCodeHosts({
                    loaded: true,
                    hosts: result.nodes,
                    configuredRepos: selected,
                })
                if (selected.length !== 0) {
                    setSelectionState({
                        repos: selectionState.repos,
                        radio: 'selected',
                        loaded: selectionState.loaded,
                    })
                }
                codeHostSubscription.unsubscribe()
            })
        }
    }, [codeHosts, selectionState.loaded, selectionState.repos, userID])()

    // once we've loaded code hosts and the 'selected' panel is visible, load repos and set the loading state
    useCallback(() => {
        if (selectionState.radio === 'selected' && !repoState.loaded && !repoState.loading) {
            setRepoState({
                repos: emptyRepos,
                loading: true,
                loaded: false,
                error: '',
            })
            const listReposSubscription = listAffiliatedRepositories({
                user: userID,
                codeHost: null,
                query: null,
            }).subscribe(
                result => {
                    setRepoState({
                        repos: result.affiliatedRepositories.nodes,
                        loading: false,
                        loaded: true,
                        error: '',
                    })
                    listReposSubscription.unsubscribe()
                },
                error => {
                    setRepoState({
                        repos: emptyRepos,
                        loading: false,
                        loaded: true,
                        error: String(error),
                    })
                }
            )
        }
    }, [repoState.loaded, repoState.loading, selectionState.radio, userID])()

    // if we've loaded repos we should then populate our selection from
    // code host config.
    if (repoState.loaded && codeHosts.loaded && !selectionState.loaded) {
        const selectedRepos = new Map<string, Repo>()

        for (const repo of codeHosts.configuredRepos) {
            const foundInState = repoState.repos.find(fullRepo => fullRepo.name === repo)
            if (foundInState) {
                selectedRepos.set(repo, foundInState)
            }
        }

        let radioState = selectionState.radio
        if (selectionState.radio === 'all' && selectedRepos.size > 0) {
            radioState = 'selected'
        }
        setSelectionState({
            repos: selectedRepos,
            loaded: true,
            radio: radioState,
        })
    }

    // filter our set of repos based on query & code host selection
    const filteredRepos: Repo[] = []
    for (const repo of repoState.repos) {
        if (!repo.name.toLowerCase().includes(query)) {
            continue
        }
        if (codeHostFilter !== '' && repo.codeHost?.id !== codeHostFilter) {
            continue
        }
        filteredRepos.push(repo)
    }

    // create elements for pagination
    const pages: JSX.Element[] = []
    for (let page = 1; page <= Math.ceil(filteredRepos.length / PER_PAGE); page++) {
        if (page === 1) {
            pages.push(
                (page !== currentPage && (
                    <button
                        type="button"
                        key="prev"
                        className="btn btn-link px-0 text-primary user-settings-repos__pageend"
                        onClick={() => setPage(currentPage - 1)}
                    >
                        <ChevronLeftIcon className="icon-inline fill-primary" />
                        Previous
                    </button>
                )) || (
                    <button
                        type="button"
                        key="prev"
                        className="btn btn-link px-0 text-muted user-settings-repos__pageend"
                    >
                        <ChevronLeftIcon className="icon-inline fill-border-color-2" />
                        Previous
                    </button>
                )
            )
        }
        pages.push(
            <button
                type="button"
                key={page}
                className={classNames({
                    'btn user-settings-repos__page': true,
                    'user-settings-repos__page--active': currentPage === page,
                })}
                onClick={() => setPage(page)}
            >
                <p
                    className={classNames({
                        'mb-0': true,
                        'text-muted': currentPage === page,
                        'text-primary': currentPage !== page,
                    })}
                >
                    {page}
                </p>
            </button>
        )
        if (page === Math.ceil(filteredRepos.length / PER_PAGE)) {
            pages.push(
                (page !== currentPage && (
                    <button
                        type="button"
                        key="next"
                        className="btn btn-link px-0 text-primary user-settings-repos__pageend"
                        onClick={() => setPage(currentPage + 1)}
                    >
                        Next
                        <ChevronRightIcon className="icon-inline fill-primary" />
                    </button>
                )) || (
                    <button
                        type="button"
                        key="next"
                        className="btn btn-link px-0 text-muted user-settings-repos__pageend"
                    >
                        Next
                        <ChevronRightIcon className="icon-inline user-settings-repos__chevron--inactive" />
                    </button>
                )
            )
        }
    }

    // save changes and update code hosts
    const submit = useCallback(
        async (event: FormEvent<HTMLFormElement>): Promise<void> => {
            event.preventDefault()
            const syncTimes = new Map<string, string>()
            const codeHostRepoPromises = []

            setFetchingRepos('loading')

            for (const host of codeHosts.hosts) {
                const repos: string[] = []
                syncTimes.set(host.id, host.lastSyncAt)
                for (const repo of selectionState.repos.values()) {
                    if (repo.codeHost?.id !== host.id) {
                        continue
                    }
                    repos.push(repo.name)
                }

                codeHostRepoPromises.push(
                    setExternalServiceRepos({
                        id: host.id,
                        allRepos: selectionState.radio === 'all',
                        repos: (selectionState.radio === 'selected' && repos) || null,
                    })
                )
            }

            try {
                await Promise.all(codeHostRepoPromises)
            } catch (error) {
                setRepoState({ ...repoState, error: String(error) })
            }

            const started = Date.now()
            const externalServiceSubscription = queryExternalServices({
                first: null,
                after: null,
                namespace: userID,
            })
                .pipe(
                    repeatUntil(
                        result => {
                            // if the background job takes too long we should update the button
                            // text to indicate we're still working on it.

                            const now = Date.now()
                            const timeDiff = now - started

                            // setting the same state multiple times won't cause
                            // re-renders in Function components
                            if (timeDiff >= SIX_SECONDS + EIGHT_SECONDS) {
                                setFetchingRepos('slower')
                            } else if (timeDiff >= SIX_SECONDS) {
                                setFetchingRepos('slow')
                            }

                            // if the lastSyncAt has changed for all hosts then we're done
                            if (result.nodes.every(codeHost => codeHost.lastSyncAt !== syncTimes.get(codeHost.id))) {
                                const repoCount = result.nodes.reduce((sum, codeHost) => sum + codeHost.repoCount, 0)
                                onUserRepositoriesUpdate(repoCount)
                                // push the user back to the repo list page
                                history.push(routingPrefix + '/repositories')
                                // cancel the repeatUntil
                                return true
                            }
                            // keep repeating
                            return false
                        },
                        { delay: 2000 }
                    )
                )
                .subscribe(
                    () => {},
                    error => {
                        setRepoState({ ...repoState, error: String(error) })
                    },
                    () => {
                        externalServiceSubscription.unsubscribe()
                    }
                )
        },
        [
            codeHosts.hosts,
            history,
            repoState,
            routingPrefix,
            selectionState.radio,
            selectionState.repos,
            userID,
            onUserRepositoriesUpdate,
        ]
    )

    const handleRadioSelect = (changeEvent: React.ChangeEvent<HTMLInputElement>): void => {
        setSelectionState({
            repos: selectionState.repos,
            radio: changeEvent.currentTarget.value,
            loaded: selectionState.loaded,
        })
    }

    const modeSelect: JSX.Element = (
        <Form className="mt-4">
            <label className="d-flex flex-row align-items-baseline">
                <input
                    type="radio"
                    value="all"
                    disabled={true}
                    checked={selectionState.radio === 'all'}
                    onChange={handleRadioSelect}
                />
                <div className="d-flex flex-column ml-2">
                    <p
                        className="mb-0 user-settings-repos__text-coming-soon
"
                    >
                        Sync all repositories (coming soon)
                    </p>
                    <p
                        className="user-settings-repos__text-coming-soon
"
                    >
                        Will sync all current and future public and private repositories
                    </p>
                </div>
            </label>
            <label className="d-flex flex-row align-items-baseline">
                {/* TODO: @artem add this functionality after pe-GA */}
                <input
                    type="radio"
                    value="all"
                    disabled={true}
                    checked={selectionState.radio === 'org'}
                    onChange={handleRadioSelect}
                />
                <div className="d-flex flex-column ml-2">
                    <p
                        className="mb-0 user-settings-repos__text-coming-soon
"
                    >
                        Sync all repositories from selected organizations or users (coming soon)
                    </p>
                    <p
                        className="user-settings-repos__text-coming-soon
"
                    >
                        Will sync all current and future public and private repositories
                    </p>
                </div>
            </label>
            <label className="d-flex flex-row align-items-baseline">
                <input
                    type="radio"
                    value="selected"
                    checked={selectionState.radio === 'selected'}
                    onChange={handleRadioSelect}
                />
                <div className="d-flex flex-column ml-2">
                    <p className="mb-0">Sync selected public repositories</p>
                </div>
            </label>
        </Form>
    )

    const filterControls: JSX.Element = (
        <Form className="w-100 d-inline-flex justify-content-between flex-row mt-3">
            <div className="d-inline-flex flex-row mr-3 align-items-baseline">
                <p className="text-xl-center text-nowrap mr-2">Code Host:</p>
                <select
                    className="form-control"
                    name="code-host"
                    aria-label="select code host type"
                    onBlur={event => {
                        setCodeHostFilter(event.target.value)
                    }}
                >
                    <option key="any" value="" label="Any" />
                    {codeHosts.hosts.map(value => (
                        <option key={value.id} value={value.id} label={value.displayName} />
                    ))}
                </select>
            </div>
            <input
                className="form-control user-settings-repos__filter-input"
                type="search"
                placeholder="Search repositories..."
                name="query"
                autoComplete="off"
                autoCorrect="off"
                autoCapitalize="off"
                spellCheck={false}
                onChange={event => {
                    setQuery(event.target.value)
                }}
            />
        </Form>
    )

    const onRepoClicked = useCallback(
        (repo: Repo) => (): void => {
            const newMap = new Map(selectionState.repos)
            if (newMap.has(repo.name)) {
                newMap.delete(repo.name)
            } else {
                newMap.set(repo.name, repo)
            }
            setSelectionState({
                repos: newMap,
                radio: selectionState.radio,
                loaded: selectionState.loaded,
            })
        },
        [selectionState, setSelectionState]
    )

    const selectAll = (): void => {
        const newMap = new Map<string, Repo>()
        // if not all repos are selected, we should select all, otherwise empty the selection
        if (selectionState.repos.size !== filteredRepos.length) {
            for (const repo of filteredRepos) {
                newMap.set(repo.name, repo)
            }
        }
        setSelectionState({
            repos: newMap,
            loaded: selectionState.loaded,
            radio: selectionState.radio,
        })
    }

    const rows: JSX.Element = (
        <tbody>
            <tr className="align-items-baseline d-flex" key="header">
                <td className="user-settings-repos__repositorynode p-2 w-100 d-flex align-items-center border-top-0 border-bottom">
                    <input
                        id="select-all-repos"
                        className="mr-3"
                        type="checkbox"
                        checked={selectionState.repos.size !== 0 && selectionState.repos.size === filteredRepos.length}
                        onClick={selectAll}
                    />
                    <label
                        htmlFor="select-all-repos"
                        className={classNames({
                            'text-muted': selectionState.repos.size === 0,
                            'text-body': selectionState.repos.size !== 0,
                            'mb-0': true,
                        })}
                    >
                        {(selectionState.repos.size > 0 && (
                            <small>{`${selectionState.repos.size} repositories selected`}</small>
                        )) || <small>Select all</small>}
                    </label>
                </td>
            </tr>
            {filteredRepos.map((repo, index) => {
                if (index < (currentPage - 1) * PER_PAGE || index >= currentPage * PER_PAGE) {
                    return
                }
                return (
                    <CheckboxRepositoryNode
                        name={repo.name}
                        key={repo.name}
                        onClick={onRepoClicked(repo)}
                        checked={selectionState.repos.has(repo.name)}
                        serviceType={repo.codeHost?.kind || ''}
                        isPrivate={repo.private}
                    />
                )
            })}
        </tbody>
    )

    const loadingAnimation: JSX.Element = (
        <tbody className="container">
            <tr className="mt-2 align-items-baseline row">
                <td className="user-settings-repos__shimmer p-3 border-top-0 col-sm-9" />
            </tr>
            <tr className="mt-2 align-items-baseline row">
                <td className="user-settings-repos__shimmer p-3 border-top-0 col-sm-4" />
            </tr>
            <tr className="mt-2 align-items-baseline row">
                <td className="user-settings-repos__shimmer p-3 border-top-0 col-sm-7" />
            </tr>
        </tbody>
    )
    return (
        <div className="user-settings-repos">
            <PageTitle title="Manage Repositories" />
            <h2 className="mb-2">Manage Repositories</h2>
            <p className="text-muted">
                Choose repositories to sync with Sourcegraph to search code you care about all in one place
            </p>
            <ul className="list-group">
                <li className="list-group-item p-0 user-settings-repos__container" key="from-code-hosts">
                    <div className="p-4">
                        <h3>Your repositories</h3>
                        <p className="text-muted">
                            Repositories you own or collaborate on from your{' '}
                            <Link className="text-primary" to={`${routingPrefix}/code-hosts`}>
                                connected code hosts
                            </Link>
                        </p>
                        <div className="alert alert-primary">
                            Coming soon: search private repositories with Sourcegraph Cloud.{' '}
                            <Link
                                to="https://share.hsforms.com/1copeCYh-R8uVYGCpq3s4nw1n7ku"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                Get updated when this feature launches
                            </Link>
                        </div>
                        {
                            // display radio button for 'all' or 'selected' repos
                            modeSelect
                        }
                        {
                            // if we're in 'selected' mode, show a list of all the repos on the code hosts to select from
                            selectionState.radio === 'selected' && (
                                <div className="ml-4">
                                    {filterControls}
                                    {repoState.error !== '' && <ErrorAlert error={repoState.error} />}
                                    <table role="grid" className="table">
                                        {
                                            // if we're selecting repos, and the repos are still loading, display the loading animation
                                            selectionState.radio === 'selected' &&
                                                repoState.loading &&
                                                !repoState.loaded &&
                                                loadingAnimation
                                        }
                                        {
                                            // if the repos are loaded display the rows of repos
                                            repoState.loaded && rows
                                        }
                                    </table>
                                    <div className="user-settings-repos__pages">
                                        {
                                            // pagination control
                                            pages
                                        }
                                    </div>
                                </div>
                            )
                        }
                    </div>
                </li>
                <li className="list-group-item p-0 user-settings-repos__container" key="add-textarea">
                    <div className="p-4 text-muted">
                        <h3 className="text-muted">Other public repositories (coming soon)</h3>
                        <p className="text-muted">Public repositories on GitHub and GitLab</p>
                        <input
                            disabled={true}
                            id="add-public-repos"
                            className="mr-2 mt-2"
                            type="checkbox"
                            onChange={toggleTextArea}
                            checked={showTextArea}
                        />
                        <label htmlFor="add-public-repos">Sync specific public repositories by URL</label>

                        {showTextArea && (
                            <div className="form-group ml-4 mt-3">
                                <p className="mb-2">Repositories to sync</p>
                                <textarea className="form-control" rows={5} />
                                <p className="text-muted mt-2">Specify with complete URLs. One repository per line.</p>
                            </div>
                        )}
                    </div>
                </li>
            </ul>
            <Form className="mt-4 d-flex" onSubmit={submit}>
                <LoaderButton
                    loading={isLoading(fetchingRepos)}
                    className="btn btn-primary test-goto-add-external-service-page mr-2"
                    alwaysShowLabel={true}
                    type="submit"
                    label={
                        (!fetchingRepos && 'Save changes') ||
                        (fetchingRepos === 'loading' && 'Saving changes...') ||
                        (fetchingRepos === 'slow' && 'Still working...') ||
                        "That's a lot of code..."
                    }
                    disabled={!selectionState.radio || isLoading(fetchingRepos)}
                />

                <Link
                    className="btn btn-secondary test-goto-add-external-service-page"
                    to={`${routingPrefix}/repositories`}
                >
                    Cancel
                </Link>
            </Form>
        </div>
    )
}
