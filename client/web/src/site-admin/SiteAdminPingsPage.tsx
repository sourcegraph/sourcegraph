import React, { useEffect, useMemo, useRef } from 'react'

import { json } from '@codemirror/lang-json'
import { foldGutter } from '@codemirror/language'
import { search, searchKeymap } from '@codemirror/search'
import { EditorState } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'
import { isEmpty } from 'lodash'
import { fromFetch } from 'rxjs/fetch'

import { checkOk } from '@sourcegraph/http-client'
import {
    defaultEditorTheme,
    editorHeight,
    jsonHighlighting,
    useCodeMirror,
} from '@sourcegraph/shared/src/components/CodeMirrorEditor'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Container, H3, Link, LoadingSpinner, PageHeader, Text, useObservable } from '@sourcegraph/wildcard'

import { PageTitle } from '../components/PageTitle'

// This seems to be necessary to have properly rounded corners on
// the right side.
const theme = EditorView.theme({
    '.cm-scroller': {
        borderTopRightRadius: 'var(--border-radius)',
        borderBottomRightRadius: 'var(--border-radius)',
    },
})

interface Props extends TelemetryV2Props {}

/**
 * A page displaying information about telemetry pings for the site.
 */
export const SiteAdminPingsPage: React.FunctionComponent<React.PropsWithChildren<Props>> = ({ telemetryRecorder }) => {
    const isLightTheme = useIsLightTheme()
    const latestPing = useObservable(
        useMemo(() => fromFetch<{}>('/site-admin/pings/latest', { selector: response => checkOk(response).json() }), [])
    )
    useEffect(() => {
        EVENT_LOGGER.logViewEvent('SiteAdminPings')
        telemetryRecorder.recordEvent('admin.pings', 'view')
    }, [telemetryRecorder])

    const updatesDisabled = window.context.site['update.channel'] !== 'release'
    const jsonEditorContainerRef = useRef<HTMLDivElement | null>(null)
    const editorRef = useRef<EditorView | null>(null)

    useCodeMirror(
        editorRef,
        jsonEditorContainerRef,
        useMemo(() => JSON.stringify(latestPing, undefined, 4), [latestPing]),
        useMemo(
            () => [
                EditorView.darkTheme.of(!isLightTheme),
                EditorState.readOnly.of(true),
                json(),
                foldGutter(),
                editorHeight({ height: '300px' }),
                theme,
                defaultEditorTheme,
                jsonHighlighting,
                search({ top: true }),
                keymap.of(searchKeymap),
            ],
            [isLightTheme]
        )
    )

    return (
        <div className="site-admin-pings-page">
            <PageTitle title="Pings - Admin" />
            <PageHeader
                path={[{ text: 'Pings' }]}
                headingElement="h2"
                description={
                    <>
                        Sourcegraph periodically sends a ping to Sourcegraph.com to help our product and customer teams.
                        It sends only the high-level data below. It never sends code, repository names, usernames, or
                        any other specific data.
                    </>
                }
                className="mb-3"
            />
            <Container>
                <H3>Most recent ping</H3>
                {latestPing === undefined ? (
                    <Text>
                        <LoadingSpinner />
                    </Text>
                ) : isEmpty(latestPing) ? (
                    <Text>No recent ping data to display.</Text>
                ) : (
                    <div ref={jsonEditorContainerRef} className="mb-1 border rounded" />
                )}
                <H3>Critical telemetry</H3>
                <Text>
                    Critical telemetry includes only the high-level data below required for billing, support, updates,
                    and security notices. This cannot be disabled.
                </Text>
                <ul>
                    <li>Randomly generated site identifier</li>
                    <li>
                        The email address of the initial site installer (or if deleted, the first active site admin), to
                        know who to contact regarding sales, product updates, security updates, and policy updates
                    </li>
                    <li>The external URL of the instance (e.g. "https://sourcegraph.example.com")</li>
                    <li>Sourcegraph version string (e.g. "vX.X.X")</li>
                    <li>Dependency versions (e.g. "6.0.9" for Redis, or "13.0" for Postgres)</li>
                    <li>
                        Deployment type (single Docker image, Docker Compose, Kubernetes cluster, Helm, or pure Docker
                        cluster)
                    </li>
                    <li>License key associated with your Sourcegraph subscription</li>
                    <li>Aggregate count of current monthly users</li>
                    <li>Total count of existing user accounts</li>
                    <li>Code Insights: total count of insights</li>
                </ul>
                <H3>Other telemetry</H3>
                <Text>
                    By default, Sourcegraph also aggregates usage and performance metrics for some product features. No
                    personal or specific information is ever included.
                </Text>
                <ul>
                    <li>Whether the instance is deployed on localhost (true/false)</li>
                    <li>
                        Which category of authentication provider is in use (built-in, OpenID Connect, an HTTP proxy,
                        SAML, GitHub, GitLab)
                    </li>
                    <li>
                        Which code hosts are in use (GitHub, Bitbucket Server, GitLab, Phabricator, Gitolite, AWS
                        CodeCommit, Other)
                        <ul>
                            <li>Which versions of the code hosts are used</li>
                        </ul>
                    </li>
                    <li>Whether new user signup is allowed (true/false)</li>
                    <li>Whether a repository has ever been added (true/false)</li>
                    <li>Whether a code search has ever been executed (true/false)</li>
                    <li>Whether code navigation has ever been used (true/false)</li>
                    <li>Aggregate counts of current daily, weekly, and monthly users</li>
                    <li>
                        Aggregate counts of current daily, weekly, and monthly users, by:
                        <ul>
                            <li>Whether they are using code host integrations</li>
                            <li>Search modes used (interactive search, plain-text search)</li>
                            <li>Search filters used (e.g. "type:", "repo:", "file:", "lang:", etc.)</li>
                        </ul>
                    </li>
                    <li>Aggregate daily, weekly, and monthly latencies (in ms) of search queries</li>
                    <li>
                        Aggregate daily, weekly, and monthly counts of:
                        <ul>
                            <li>Code navigation events (e.g., hover tooltips)</li>
                            <li>Searches using each search mode (interactive search, plain-text search)</li>
                            <li>Searches using each search filter (e.g. "type:", "repo:", "file:", "lang:", etc.)</li>
                        </ul>
                    </li>
                    <li>
                        Code navigation usage data
                        <ul>
                            <li>Total number of repositories with and without an uploaded LSIF index</li>
                            <li>
                                Total number of code navigation queries (e.g., hover tooltips) per week grouped by
                                language
                            </li>
                            <li>
                                Number of users performing code navigation queries (e.g., hover tooltips) per week
                                grouped by language
                            </li>
                        </ul>
                    </li>
                    <li>
                        Batch changes usage data
                        <ul>
                            <li>Total count of page views on the batch change apply page</li>
                            <li>
                                Total count of page views on the batch change details page after creating a batch change
                            </li>
                            <li>
                                Total count of page views on the batch change details page after updating a batch change
                            </li>
                            <li>Total count of created changeset specs</li>
                            <li>Total count of created batch specs</li>
                            <li>Total count of created batch changes</li>
                            <li>Total count of closed batch changes</li>
                            <li>Total count of changesets created by batch changes</li>
                            <li>Aggregate counts of lines added, deleted in changeset</li>
                            <li>Total count of changesets created by batch changes that have been merged</li>
                            <li>Aggregate counts of lines added, deleted in merged changeset</li>
                            <li>Total count of changesets manually added to a batch change</li>
                            <li>Total count of changesets manually added to a batch change that have been merged</li>
                            <li>
                                Aggregate counts of unique monthly users, by:
                                <ul>
                                    <li>Whether they have contributed to batch changes</li>
                                    <li>Whether they only viewed batch changes</li>
                                    <li>Whether they have performed a bulk operation</li>
                                </ul>
                            </li>
                            <li>
                                Weekly batch change (open, closed) and changesets counts (imported, published,
                                unpublished, open, draft, merged, closed) for batch change cohorts created in the last
                                12 months
                            </li>
                            <li>Weekly bulk operations count (grouped by operation)</li>
                            <li>Total count of executors connected</li>
                            <li>Cumulative executor runtime monthly</li>
                            <li>Total count of publish bulk operation</li>
                            <li>Total count of bulk operations (grouped by operation type)</li>
                            <li>
                                Changeset distribution for batch change (grouped by batch change source: local or
                                executor)
                            </li>
                            <li>Total count of users that ran a job on an executor monthly</li>
                            <li>
                                Total count of published changesets and batch changes created via:
                                <ul>
                                    <li>executor</li>
                                    <li>local (using src-cli)</li>
                                </ul>
                            </li>
                        </ul>
                    </li>
                    <li>
                        Monthly aggregated user state changes
                        <ul>
                            <li>Count of users created</li>
                            <li>Count of users deleted</li>
                            <li>Count of users retained</li>
                            <li>Count of users resurrected</li>
                            <li>Count of users churned</li>
                        </ul>
                    </li>
                    <li>
                        Monthly aggregated access requests changes
                        <ul>
                            <li>Count of pending access requests</li>
                            <li>Count of approved access requests</li>
                            <li>Count of rejected access requests</li>
                        </ul>
                    </li>
                    <li>
                        Saved searches usage data
                        <ul>
                            <li>Count of saved searches</li>
                            <li>Count of users using saved searches</li>
                            <li>Count of notifications triggered</li>
                            <li>Count of notifications clicked</li>
                            <li>Count of saved search views</li>
                        </ul>
                    </li>
                    <li>
                        Aggregated repository statistics
                        <ul>
                            <li>Total size of git repositories stored in bytes</li>
                            <li>Total number of lines of code stored in text search index</li>
                        </ul>
                    </li>
                    <li>
                        Homepage panel engagement
                        <ul>
                            <li>Percentage of panel clicks (out of total views)</li>
                            <li>Total count of unique users engaging with the panels</li>
                        </ul>
                    </li>
                    <li>Weekly retention rates for user cohorts created in the last 12 weeks</li>
                    <li>
                        Search onboarding engagement
                        <ul>
                            <li>Total number of views of the onboarding tour</li>
                            <li>Total number of views of each step in the onboarding tour</li>
                            <li>Total number of tours closed</li>
                        </ul>
                    </li>
                    <li>
                        Sourcegraph extension activation statistics
                        <ul>
                            <li>Total number of users that use a given non-default Sourcegraph extension</li>
                            <li>
                                Average number of activations for users that use a given non-default Sourcegraph
                                extension
                            </li>
                            <li>Total number of users that use non-default Sourcegraph extensions</li>
                            <li>
                                Average number of non-default extensions enabled for users that use non-default
                                Sourcegraph extensions
                            </li>
                        </ul>
                    </li>
                    <li>
                        Code insights usage data
                        <ul>
                            <li>
                                <Link to="/help/admin/pings#other-telemetry">
                                    See a full list of Code Insights pings.
                                </Link>
                            </li>
                        </ul>
                    </li>
                    <li>
                        Code monitoring usage data
                        <ul>
                            <li>Total number of views of the code monitoring page</li>
                            <li>Total number of views of the create code monitor page</li>
                            <li>
                                Total number of views of the create code monitor page with a pre-populated trigger query
                            </li>
                            <li>
                                Total number of views of the create code monitor page without a pre-populated trigger
                                query
                            </li>
                            <li>Total number of views of the manage code monitor page</li>
                            <li>Total number of clicks on the code monitor email search link</li>
                            <li>Total number of clicks on example monitors</li>
                            <li>Total number of views of the getting started page</li>
                            <li>Total number of submissions of the code monitor creation form</li>
                            <li>Total number of submissions of the manage code monitor form</li>
                            <li>Total number of deletions from the manage code monitor form</li>
                            <li>Total number of views of the logs page</li>
                            <li>Current number of Slack, webhook, and email actions enabled</li>
                            <li>Current number of unique users with Slack, webhook, and email actions enabled</li>
                            <li>Total number of Slack, webhook, and email actions triggered</li>
                            <li>Total number of Slack, webhook, and email action triggers that errored</li>
                            <li>
                                Total number of unique users that have had Slack, webhook, and email actions triggered
                            </li>
                            <li>Total number of search executions</li>
                            <li>Total number of search executions that errored</li>
                            <li>50th and 90th percentile runtimes for search executions</li>
                        </ul>
                    </li>
                    <li>
                        Notebooks usage data
                        <ul>
                            <li>Total number of views of the notebook page</li>
                            <li>Total number of views of the notebooks list page</li>
                            <li>Total number of views of the embedded notebook page</li>
                            <li>Total number of created notebooks</li>
                            <li>Total number of added notebook stars</li>
                            <li>Total number of added notebook markdown blocks</li>
                            <li>Total number of added notebook query blocks</li>
                            <li>Total number of added notebook file blocks</li>
                            <li>Total number of added notebook symbol blocks</li>
                            <li>Total number of added notebook compute blocks</li>
                        </ul>
                    </li>
                    <li>
                        Code Host integration usage data (Browser extension / Native Integration)
                        <ul>
                            <li>
                                Aggregate counts of current daily, weekly, and monthly unique users and total events
                            </li>
                            <li>
                                Aggregate counts of current daily, weekly, and monthly unique users and total events who
                                visited Sourcegraph instance from browser extension
                            </li>
                        </ul>
                    </li>
                    <li>
                        IDE extensions data
                        <ul>
                            <li>
                                Aggregate counts of current daily, weekly, and monthly searches performed:
                                <ul>
                                    <li>Count of unique users who performed searches</li>
                                    <li>Count of total searches performed</li>
                                </ul>
                            </li>
                        </ul>
                        <ul>
                            <li>Aggregate counts of daily user state:</li>
                            <ul>
                                <li>Count of unique users who installed the extension</li>
                                <li>Count of unique users who uninstalled the extension</li>
                            </ul>
                            <li>Aggregate count of daily redirects from extension to Sourcegraph instance</li>
                        </ul>
                    </li>
                    <li>
                        Migrated extensions data
                        <ul>
                            <li>Aggregate data of:</li>
                            <ul>
                                <li>Count interactions with the Git blame feature</li>
                                <li>Count of unique users who interacted with the Git blame feature</li>
                                <li>Count interactions with the open in editor feature</li>
                                <li>Count of unique users who interacted with the open in editor feature</li>
                                <li>Count interactions with the search exports feature</li>
                                <li>Count of unique users who interacted with the search exports feature</li>
                            </ul>
                        </ul>
                    </li>
                    <li>
                        Code ownership usage data
                        <ul>
                            <li>
                                Number and ratio of repositories for which ownership data is available via CODEOWNERS
                                file or the API.
                            </li>
                            <li>Total count of assigned owners.</li>
                            <li>Aggregate monthly weekly and daily active users for the following activities:</li>
                            <ul>
                                <li>Narrowing search results by owner using file:has.owner() predicate.</li>
                                <li>Selecting owner search result through select:file.owners.</li>
                                <li>Displaying ownership panel in file view.</li>
                            </ul>
                        </ul>
                    </li>
                    <li>Histogram of cloned repository sizes</li>
                    <li>Aggregate daily, weekly, monthly repository metadata usage statistics</li>
                    <li>
                        Cody providers data
                        <ul>
                            <li>
                                Completions
                                <ul>
                                    <li>
                                        Provider (e.g., "sourcegraph", "anthropic", "openai", "azure-openai",
                                        "fireworks", "aws-bedrock", "google", etc.)
                                    </li>
                                    <li>Chat model (included only for "sourcegraph" provider)</li>
                                    <li>Fast chat model (included only for "sourcegraph" provider)</li>
                                    <li>Completion model (included only for "sourcegraph" provider)</li>
                                </ul>
                            </li>
                        </ul>
                    </li>
                    <li>Whether Cody context filters are configured in the site config (true/false)</li>
                </ul>
                {updatesDisabled && <Text>All telemetry is disabled.</Text>}
            </Container>
        </div>
    )
}
