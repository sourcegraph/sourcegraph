import React, { useMemo } from 'react'

import { Route, Routes } from 'react-router-dom'

import type { Scalars } from '@sourcegraph/shared/src/graphql-operations'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { AuthenticatedUser } from '../../../auth'
import { withAuthenticatedUser } from '../../../auth/withAuthenticatedUser'
import { canWriteBatchChanges, NO_ACCESS_BATCH_CHANGES_WRITE, NO_ACCESS_SOURCEGRAPH_COM } from '../../../batches/utils'
import { NotFoundPage } from '../../../components/HeroPage'
import type { BatchChangeClosePageProps } from '../close/BatchChangeClosePage'
import type { CreateBatchChangePageProps } from '../create/CreateBatchChangePage'
import type { BatchChangeDetailsPageProps } from '../detail/BatchChangeDetailsPage'
import { TabName } from '../detail/BatchChangeDetailsTabs'
import type { BatchChangeListPageProps, NamespaceBatchChangeListPageProps } from '../list/BatchChangeListPage'
import type { BatchChangePreviewPageProps } from '../preview/BatchChangePreviewPage'

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

interface Props extends TelemetryProps, TelemetryV2Props, SettingsCascadeProps {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
}

/**
 * The global batch changes area.
 */
export const GlobalBatchChangesArea: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    authenticatedUser,
    isSourcegraphDotCom,
    ...props
}) => {
    const canCreate: true | string = useMemo(() => {
        if (isSourcegraphDotCom) {
            return NO_ACCESS_SOURCEGRAPH_COM
        }
        if (!canWriteBatchChanges(authenticatedUser)) {
            return NO_ACCESS_BATCH_CHANGES_WRITE
        }
        return true
    }, [isSourcegraphDotCom, authenticatedUser])

    return (
        <div className="w-100">
            <Routes>
                <Route
                    path=""
                    element={
                        <BatchChangeListPage
                            headingElement="h1"
                            canCreate={canCreate}
                            authenticatedUser={authenticatedUser}
                            isSourcegraphDotCom={isSourcegraphDotCom}
                            {...props}
                        />
                    }
                />
                {!isSourcegraphDotCom && canCreate === true && (
                    <Route
                        path="create"
                        element={
                            <AuthenticatedCreateBatchChangePage
                                {...props}
                                headingElement="h1"
                                authenticatedUser={authenticatedUser}
                            />
                        }
                    />
                )}
                <Route path="*" element={<NotFoundPage pageType="batch changes" />} key="hardcoded-key" />
            </Routes>
        </div>
    )
}

const AuthenticatedCreateBatchChangePage = withAuthenticatedUser<
    CreateBatchChangePageProps & { authenticatedUser: AuthenticatedUser }
>(props => <CreateBatchChangePage {...props} />)

export interface NamespaceBatchChangesAreaProps extends Props {
    namespaceID: Scalars['ID']
}

export const NamespaceBatchChangesArea = withAuthenticatedUser<
    NamespaceBatchChangesAreaProps & { authenticatedUser: AuthenticatedUser }
>(props => (
    <div className="pb-3">
        <Routes>
            <Route path="apply/:batchSpecID" element={<BatchChangePreviewPage {...props} />} />
            <Route path=":batchChangeName/close" element={<BatchChangeClosePage {...props} />} />
            <Route
                path=":batchChangeName/executions"
                element={<BatchChangeDetailsPage {...props} initialTab={TabName.Executions} />}
            />
            <Route path=":batchChangeName" element={<BatchChangeDetailsPage {...props} />} />
            <Route path="" element={<NamespaceBatchChangeListPage headingElement="h2" {...props} />} />
        </Routes>
    </div>
))
