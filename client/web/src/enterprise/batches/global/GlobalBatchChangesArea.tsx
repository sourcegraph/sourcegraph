import React from 'react'
import { RouteComponentProps, Switch, Route } from 'react-router'
import { withAuthenticatedUser } from '../../../auth/withAuthenticatedUser'
import { ThemeProps } from '../../../../../shared/src/theme'
import { ExtensionsControllerProps } from '../../../../../shared/src/extensions/controller'
import { PlatformContextProps } from '../../../../../shared/src/platform/context'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import { AuthenticatedUser } from '../../../auth'
import { Scalars } from '../../../../../shared/src/graphql-operations'
import { lazyComponent } from '../../../util/lazyComponent'
import { BatchChangeListPageProps, NamespaceBatchChangeListPageProps } from '../list/BatchChangeListPage'
import { BatchChangePreviewPageProps } from '../preview/BatchChangePreviewPage'
import { CreateBatchChangePageProps } from '../create/CreateBatchChangePage'
import { BatchChangeDetailsPageProps } from '../detail/BatchChangeDetailsPage'
import { BatchChangeClosePageProps } from '../close/BatchChangeClosePage'
import { BatchChangesDotComPageProps } from './marketing/BatchChangesDotComPage'
import { Page } from '../../../components/Page'

const BatchChangeListPage = lazyComponent<BatchChangeListPageProps, 'BatchChangeListPage'>(
    () => import('../list/BatchChangeListPage'),
    'BatchChangeListPage'
)
const NamespaceBatchChangeListPage = lazyComponent<NamespaceBatchChangeListPageProps, 'NamespaceBatchChangeListPage'>(
    () => import('../list/BatchChangeListPage'),
    'NamespaceBatchChangeListPage'
)
const BatchChangePreviewPage = lazyComponent<BatchChangePreviewPageProps, 'BatchChangePreviewPage'>(
    () => import('../preview/BatchChangePreviewPage'),
    'BatchChangePreviewPage'
)
const CreateBatchChangePage = lazyComponent<CreateBatchChangePageProps, 'CreateBatchChangePage'>(
    () => import('../create/CreateBatchChangePage'),
    'CreateBatchChangePage'
)
const BatchChangeDetailsPage = lazyComponent<BatchChangeDetailsPageProps, 'BatchChangeDetailsPage'>(
    () => import('../detail/BatchChangeDetailsPage'),
    'BatchChangeDetailsPage'
)
const BatchChangeClosePage = lazyComponent<BatchChangeClosePageProps, 'BatchChangeClosePage'>(
    () => import('../close/BatchChangeClosePage'),
    'BatchChangeClosePage'
)
const BatchChangesDotComPage = lazyComponent<BatchChangesDotComPageProps, 'BatchChangesDotComPage'>(
    () => import('./marketing/BatchChangesDotComPage'),
    'BatchChangesDotComPage'
)

interface Props
    extends RouteComponentProps<{}>,
        ThemeProps,
        ExtensionsControllerProps,
        TelemetryProps,
        PlatformContextProps {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
}

/**
 * The global batch changes area.
 */
export const GlobalBatchChangesArea: React.FunctionComponent<Props> = props => {
    if (props.isSourcegraphDotCom) {
        return (
            <Page>
                <BatchChangesDotComPage />
            </Page>
        )
    }
    return <AuthenticatedBatchChangesArea {...props} />
}

interface AuthenticatedProps extends Props {
    authenticatedUser: AuthenticatedUser
}

export const AuthenticatedBatchChangesArea = withAuthenticatedUser<AuthenticatedProps>(({ match, ...outerProps }) => (
    <Page>
        {/* eslint-disable react/jsx-no-bind */}
        <Switch>
            <Route render={props => <BatchChangeListPage {...outerProps} {...props} />} path={match.url} exact={true} />
            <Route
                path={`${match.url}/create`}
                render={props => <CreateBatchChangePage {...outerProps} {...props} />}
                exact={true}
            />
        </Switch>
        {/* eslint-enable react/jsx-no-bind */}
    </Page>
))

export interface NamespaceBatchChangesAreaProps extends Props {
    namespaceID: Scalars['ID']
}

export const NamespaceBatchChangesArea = withAuthenticatedUser<
    NamespaceBatchChangesAreaProps & { authenticatedUser: AuthenticatedUser }
>(({ match, namespaceID, ...outerProps }) => (
    <Page>
        {/* eslint-disable react/jsx-no-bind */}
        <Switch>
            <Route
                path={`${match.url}/apply/:specID`}
                render={({ match, ...props }: RouteComponentProps<{ specID: string }>) => (
                    <BatchChangePreviewPage {...outerProps} {...props} batchSpecID={match.params.specID} />
                )}
            />
            <Route
                path={`${match.url}/create`}
                render={props => <CreateBatchChangePage {...outerProps} {...props} />}
            />
            <Route
                path={`${match.url}/:batchChangeName/close`}
                render={({ match, ...props }: RouteComponentProps<{ batchChangeName: string }>) => (
                    <BatchChangeClosePage
                        {...outerProps}
                        {...props}
                        namespaceID={namespaceID}
                        batchChangeName={match.params.batchChangeName}
                    />
                )}
            />
            <Route
                path={`${match.url}/:batchChangeName`}
                render={({ match, ...props }: RouteComponentProps<{ batchChangeName: string }>) => (
                    <BatchChangeDetailsPage
                        {...outerProps}
                        {...props}
                        namespaceID={namespaceID}
                        batchChangeName={match.params.batchChangeName}
                    />
                )}
            />
            <Route
                path={match.url}
                render={props => <NamespaceBatchChangeListPage {...props} {...outerProps} namespaceID={namespaceID} />}
                exact={true}
            />
        </Switch>
        {/* eslint-enable react/jsx-no-bind */}
    </Page>
))
