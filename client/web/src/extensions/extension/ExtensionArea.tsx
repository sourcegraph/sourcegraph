import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import * as React from 'react'
import { Route, RouteComponentProps, Switch } from 'react-router'
import { combineLatest, merge, Observable, of, Subject, Subscription } from 'rxjs'
import { catchError, distinctUntilChanged, map, mapTo, startWith, switchMap } from 'rxjs/operators'
import {
    ConfiguredRegistryExtension,
    splitExtensionID,
    toConfiguredRegistryExtension,
} from '../../../../shared/src/extensions/extension'
import { gql } from '../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../shared/src/graphql/schema'
import { PlatformContextProps } from '../../../../shared/src/platform/context'
import { SettingsCascadeProps } from '../../../../shared/src/settings/settings'
import { createAggregateError, ErrorLike, isErrorLike, asError } from '../../../../shared/src/util/errors'
import { queryGraphQL } from '../../backend/graphql'
import { ErrorBoundary } from '../../components/ErrorBoundary'
import { HeroPage } from '../../components/HeroPage'
import { RouteDescriptor } from '../../util/contributions'
import { ExtensionsAreaRouteContext } from '../ExtensionsArea'
import { ExtensionAreaHeader, ExtensionAreaHeaderNavItem } from './ExtensionAreaHeader'
import { ThemeProps } from '../../../../shared/src/theme'
import { ErrorMessage } from '../../components/alerts'
import { TelemetryProps } from '../../../../shared/src/telemetry/telemetryService'
import { AuthenticatedUser } from '../../auth'
import { BreadcrumbSetters } from '../../components/Breadcrumbs'

export const registryExtensionFragment = gql`
    fragment RegistryExtensionFields on RegistryExtension {
        id
        publisher {
            __typename
            ... on User {
                id
                username
                displayName
                url
            }
            ... on Org {
                id
                name
                displayName
                url
            }
        }
        extensionID
        extensionIDWithoutRegistry
        name
        manifest {
            raw
            description
        }
        createdAt
        updatedAt
        publishedAt
        url
        remoteURL
        registryName
        isLocal
        isWorkInProgress
        viewerCanAdminister
    }
`

const NotFoundPage: React.FunctionComponent = () => <HeroPage icon={MapSearchIcon} title="404: Not Found" />

export interface ExtensionAreaRoute extends RouteDescriptor<ExtensionAreaRouteContext> {}

export interface ExtensionAreaProps
    extends ExtensionsAreaRouteContext,
        RouteComponentProps<{ extensionID: string }>,
        ThemeProps,
        TelemetryProps,
        BreadcrumbSetters {
    routes: readonly ExtensionAreaRoute[]
    extensionAreaHeaderNavItems: readonly ExtensionAreaHeaderNavItem[]
}

interface ExtensionAreaState {
    /** The registry extension, undefined while loading, or an error.  */
    extensionOrError?: ConfiguredRegistryExtension<GQL.IRegistryExtension> | ErrorLike
}

/**
 * Properties passed to all page components in the registry extension area.
 */
export interface ExtensionAreaRouteContext
    extends SettingsCascadeProps,
        PlatformContextProps,
        ThemeProps,
        TelemetryProps {
    /** The extension registry area main URL. */
    url: string

    /** The extension that is the subject of the page. */
    extension: ConfiguredRegistryExtension<GQL.IRegistryExtension>

    onDidUpdateExtension: () => void

    /** The currently authenticated user. */
    authenticatedUser: AuthenticatedUser | null
}

/**
 * An extension area.
 */
export class ExtensionArea extends React.Component<ExtensionAreaProps> {
    public state: ExtensionAreaState = {}

    private componentUpdates = new Subject<ExtensionAreaProps>()
    private refreshRequests = new Subject<void>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        const routeMatchChanges = this.componentUpdates.pipe(
            map(({ match }) => match.params),
            distinctUntilChanged()
        )

        // Changes to the route-matched extension ID.
        const extensionIDChanges = routeMatchChanges.pipe(
            map(({ extensionID }) => extensionID),
            distinctUntilChanged()
        )

        // Changes to the global extensions settings.
        const globalExtensionsSettingsChanges = this.componentUpdates.pipe(
            map(({ platformContext }) => platformContext),
            distinctUntilChanged()
        )

        // Set breadcrumb
        const { extensionID } = this.props.match.params

        const { publisher, name } = splitExtensionID(extensionID)

        this.subscriptions.add(
            this.props.setBreadcrumb({
                key: extensionID,
                element: `${name} by ${publisher}`,
            })
        )

        // Fetch extension.
        this.subscriptions.add(
            combineLatest([
                extensionIDChanges,
                merge(
                    this.refreshRequests.pipe(mapTo(false)),
                    globalExtensionsSettingsChanges.pipe(mapTo(false)),
                    of(false)
                ),
            ])
                .pipe(
                    switchMap(([extensionID, forceRefresh]) => {
                        type PartialStateUpdate = Pick<ExtensionAreaState, 'extensionOrError'>
                        return queryExtension(extensionID).pipe(
                            catchError((error): [ErrorLike] => [asError(error)]),
                            map((extensionOrError): PartialStateUpdate => ({ extensionOrError })),

                            // Don't clear old data while we reload, to avoid unmounting all components during
                            // loading.
                            startWith<PartialStateUpdate>(forceRefresh ? { extensionOrError: undefined } : {})
                        )
                    })
                )
                .subscribe(
                    stateUpdate => this.setState(stateUpdate),
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
        if (!this.state.extensionOrError) {
            return null // loading
        }
        if (isErrorLike(this.state.extensionOrError)) {
            return (
                <HeroPage
                    icon={AlertCircleIcon}
                    title="Error"
                    subtitle={<ErrorMessage error={this.state.extensionOrError} />}
                />
            )
        }

        // The URL, without the trailing "/-" that `this.props.match.url` includes on sub-pages.
        const url = this.props.match.url.replace(/\/-\/?$/, '')

        const context: ExtensionAreaRouteContext = {
            url,
            authenticatedUser: this.props.authenticatedUser,
            onDidUpdateExtension: this.onDidUpdateExtension,
            settingsCascade: this.props.settingsCascade,
            extension: this.state.extensionOrError,
            platformContext: this.props.platformContext,
            isLightTheme: this.props.isLightTheme,
            telemetryService: this.props.telemetryService,
        }

        return (
            <div className="registry-extension-area">
                <ExtensionAreaHeader
                    {...this.props}
                    {...context}
                    navItems={this.props.extensionAreaHeaderNavItems}
                    className="border-bottom"
                />
                <div className="container pt-3">
                    <ErrorBoundary location={this.props.location}>
                        <React.Suspense fallback={<LoadingSpinner className="icon-inline m-2" />}>
                            <Switch>
                                {this.props.routes.map(
                                    /* eslint-disable react/jsx-no-bind */
                                    ({ path, render, exact, condition = () => true }) =>
                                        condition(context) && (
                                            <Route
                                                path={url + path}
                                                exact={exact}
                                                key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                                                render={routeComponentProps =>
                                                    render({ ...context, ...routeComponentProps })
                                                }
                                            />
                                        )
                                    /* eslint-enable react/jsx-no-bind */
                                )}
                                <Route key="hardcoded-key" component={NotFoundPage} />
                            </Switch>
                        </React.Suspense>
                    </ErrorBoundary>
                </div>
            </div>
        )
    }

    private onDidUpdateExtension = (): void => this.refreshRequests.next()
}

function queryExtension(extensionID: string): Observable<ConfiguredRegistryExtension<GQL.IRegistryExtension>> {
    return queryGraphQL(
        gql`
            query RegistryExtension($extensionID: String!) {
                extensionRegistry {
                    extension(extensionID: $extensionID) {
                        ...RegistryExtensionFields
                    }
                }
            }
            ${registryExtensionFragment}
        `,
        { extensionID }
    ).pipe(
        map(({ data, errors }) => {
            if (!data || !data.extensionRegistry || !data.extensionRegistry.extension) {
                throw createAggregateError(errors)
            }
            return data.extensionRegistry.extension
        }),
        map(registryExtension => toConfiguredRegistryExtension(registryExtension))
    )
}
