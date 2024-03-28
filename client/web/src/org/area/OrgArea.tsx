import * as React from 'react'

import type * as H from 'history'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import { Route, Routes, type NavigateFunction } from 'react-router-dom'
import { combineLatest, merge, type Observable, of, Subject, Subscription } from 'rxjs'
import { catchError, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators'

import { type ErrorLike, isErrorLike, asError, logger } from '@sourcegraph/common'
import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { LoadingSpinner, ErrorMessage } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { requestGraphQL } from '../../backend/graphql'
import type { BatchChangesProps } from '../../batches'
import type { BreadcrumbsProps, BreadcrumbSetters } from '../../components/Breadcrumbs'
import { RouteError } from '../../components/ErrorBoundary'
import { HeroPage } from '../../components/HeroPage'
import { Page } from '../../components/Page'
import type { OrganizationResult, OrganizationVariables, OrgAreaOrganizationFields } from '../../graphql-operations'
import type { NamespaceProps } from '../../namespaces'
import type { RouteV6Descriptor } from '../../util/contributions'
import type { OrgSettingsAreaRoute } from '../settings/OrgSettingsArea'
import type { OrgSettingsSidebarItems } from '../settings/OrgSettingsSidebar'

import { type OrgAreaHeaderNavItem, OrgHeader } from './OrgHeader'
import { OrgInvitationPageLegacy } from './OrgInvitationPageLegacy'

function queryOrganization(args: { name: string }): Observable<OrgAreaOrganizationFields> {
    return requestGraphQL<OrganizationResult, OrganizationVariables>(
        gql`
            query Organization($name: String!) {
                organization(name: $name) {
                    ...OrgAreaOrganizationFields
                }
            }

            fragment OrgAreaOrganizationFields on Org {
                __typename
                id
                name
                displayName
                url
                settingsURL
                viewerPendingInvitation {
                    id
                    sender {
                        username
                        displayName
                        avatarURL
                        createdAt
                    }
                    respondURL
                }
                viewerIsMember
                viewerCanAdminister
                createdAt
            }
        `,
        args
    ).pipe(
        map(dataOrThrowErrors),
        map(data => {
            if (!data.organization) {
                throw new Error(`Organization not found: ${JSON.stringify(args.name)}`)
            }
            return data.organization
        })
    )
}

const NotFoundPage: React.FunctionComponent<React.PropsWithChildren<unknown>> = () => (
    <HeroPage icon={MapSearchIcon} title="404: Not Found" subtitle="Sorry, the requested organization was not found." />
)

export interface OrgAreaRoute extends RouteV6Descriptor<OrgAreaRouteContext> {
    /** When true, the header is not rendered and the component is not wrapped in a container. */
    fullPage?: boolean
}

export interface OrgAreaProps
    extends PlatformContextProps,
        SettingsCascadeProps,
        TelemetryProps,
        BreadcrumbsProps,
        BreadcrumbSetters,
        BatchChangesProps {
    orgAreaRoutes: readonly OrgAreaRoute[]
    orgAreaHeaderNavItems: readonly OrgAreaHeaderNavItem[]
    orgSettingsSideBarItems: OrgSettingsSidebarItems
    orgSettingsAreaRoutes: readonly OrgSettingsAreaRoute[]

    /**
     * The currently authenticated user.
     */
    authenticatedUser: AuthenticatedUser
    isSourcegraphDotCom: boolean

    location: H.Location
    navigate: NavigateFunction
    orgName: string
}

interface State extends BreadcrumbSetters {
    /**
     * The fetched org or an error if an error occurred; undefined while loading.
     */
    orgOrError?: OrgAreaOrganizationFields | ErrorLike
}

/**
 * Properties passed to all page components in the org area.
 */
export interface OrgAreaRouteContext
    extends PlatformContextProps,
        SettingsCascadeProps,
        TelemetryProps,
        NamespaceProps,
        BreadcrumbsProps,
        BreadcrumbSetters,
        BatchChangesProps {
    /** The org that is the subject of the page. */
    org: OrgAreaOrganizationFields

    /** Called when the organization is updated and must be reloaded. */
    onOrganizationUpdate: () => void

    /** The currently authenticated user. */
    authenticatedUser: AuthenticatedUser

    isSourcegraphDotCom: boolean

    orgSettingsSideBarItems: OrgSettingsSidebarItems
    orgSettingsAreaRoutes: readonly OrgSettingsAreaRoute[]

    license: {
        isCodeSearchEnabled: boolean
        isCodyEnabled: boolean
    }
}

/**
 * An organization's public profile area.
 */
export class OrgArea extends React.Component<OrgAreaProps> {
    public state: State

    private componentUpdates = new Subject<OrgAreaProps>()
    private refreshRequests = new Subject<void>()
    private subscriptions = new Subscription()

    constructor(props: OrgAreaProps) {
        super(props)
        this.state = {
            setBreadcrumb: props.setBreadcrumb,
            useBreadcrumb: props.useBreadcrumb,
        }
    }

    public componentDidMount(): void {
        // Changes to the route-matched org name.
        const nameChanges = this.componentUpdates.pipe(
            map(props => props.orgName),
            distinctUntilChanged()
        )

        // Fetch organization.
        this.subscriptions.add(
            combineLatest([nameChanges, merge(this.refreshRequests.pipe(map(() => false)), of(true))])
                .pipe(
                    switchMap(([name, forceRefresh]) => {
                        type PartialStateUpdate = Pick<State, 'orgOrError'>
                        return queryOrganization({ name }).pipe(
                            catchError((error): [ErrorLike] => [asError(error)]),
                            map((orgOrError): PartialStateUpdate => ({ orgOrError })),
                            // Don't clear old org data while we reload, to avoid unmounting all components during
                            // loading.
                            startWith<PartialStateUpdate>(forceRefresh ? { orgOrError: undefined } : {})
                        )
                    })
                )
                .subscribe(
                    stateUpdate => {
                        if (stateUpdate.orgOrError && !isErrorLike(stateUpdate.orgOrError)) {
                            const childBreadcrumbSetters = this.props.setBreadcrumb({
                                key: 'OrgArea',
                                link: { to: stateUpdate.orgOrError.url, label: stateUpdate.orgOrError.name },
                            })
                            this.subscriptions.add(childBreadcrumbSetters)
                            this.setState({
                                useBreadcrumb: childBreadcrumbSetters.useBreadcrumb,
                                setBreadcrumb: childBreadcrumbSetters.setBreadcrumb,
                                orgOrError: stateUpdate.orgOrError,
                            })
                        } else {
                            this.setState(stateUpdate)
                        }
                    },
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
        if (!this.state.orgOrError) {
            return null // loading
        }
        if (isErrorLike(this.state.orgOrError)) {
            return (
                <HeroPage
                    icon={AlertCircleIcon}
                    title="Error"
                    subtitle={<ErrorMessage error={this.state.orgOrError} />}
                />
            )
        }

        const context: OrgAreaRouteContext = {
            authenticatedUser: this.props.authenticatedUser,
            org: this.state.orgOrError,
            onOrganizationUpdate: this.onDidUpdateOrganization,
            platformContext: this.props.platformContext,
            settingsCascade: this.props.settingsCascade,
            namespace: this.state.orgOrError,
            telemetryService: this.props.telemetryService,
            isSourcegraphDotCom: this.props.isSourcegraphDotCom,
            batchChangesEnabled: this.props.batchChangesEnabled,
            batchChangesExecutionEnabled: this.props.batchChangesExecutionEnabled,
            batchChangesWebhookLogsEnabled: this.props.batchChangesWebhookLogsEnabled,
            breadcrumbs: this.props.breadcrumbs,
            setBreadcrumb: this.state.setBreadcrumb,
            useBreadcrumb: this.state.useBreadcrumb,
            orgSettingsAreaRoutes: this.props.orgSettingsAreaRoutes,
            orgSettingsSideBarItems: this.props.orgSettingsSideBarItems,
            license: {
                isCodeSearchEnabled: Boolean(window.context.licenseInfo?.features.codeSearch),
                isCodyEnabled: Boolean(window.context.licenseInfo?.features.cody),
            },
        }

        if (this.props.location.pathname === `/organizations/${this.props.orgName}/invitation`) {
            // The OrgInvitationPageLegacy is displayed without the OrgHeader because it is modal-like.
            return <OrgInvitationPageLegacy {...context} onDidRespondToInvitation={this.onDidRespondToInvitation} />
        }

        return (
            <React.Suspense fallback={<LoadingSpinner className="m-2" />}>
                <Routes>
                    {this.props.orgAreaRoutes.map(
                        ({ path, render, condition = () => true, fullPage }) =>
                            condition(context) && (
                                <Route
                                    path={path}
                                    key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                                    errorElement={<RouteError />}
                                    element={
                                        fullPage ? (
                                            render(context)
                                        ) : (
                                            <Page className="org-area">
                                                <OrgHeader
                                                    {...this.props}
                                                    {...context}
                                                    navItems={this.props.orgAreaHeaderNavItems}
                                                    className="mb-3"
                                                />
                                                <div className="container">{render(context)}</div>
                                            </Page>
                                        )
                                    }
                                />
                            )
                    )}
                    <Route path="*" element={<NotFoundPage />} />
                </Routes>
            </React.Suspense>
        )
    }

    private onDidRespondToInvitation = (accepted: boolean): void => {
        if (!accepted) {
            this.props.navigate('/user/settings')
            return
        }
        this.refreshRequests.next()
    }

    private onDidUpdateOrganization = (): void => this.refreshRequests.next()
}
