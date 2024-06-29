import type { FunctionComponent, PropsWithChildren } from 'react'

import { Route, Routes } from 'react-router-dom'

import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { AuthenticatedUser } from '../auth'
import { withAuthenticatedUser } from '../auth/withAuthenticatedUser'
import { NotFoundPage } from '../components/HeroPage'
import type { NamespaceAreaContext } from '../namespaces/NamespaceArea'

const SavedSearchListPage = lazyComponent(() => import('./SavedSearchListPage'), 'SavedSearchListPage')
const SavedSearchCreateForm = lazyComponent(() => import('./SavedSearchCreateForm'), 'SavedSearchCreateForm')
const SavedSearchUpdateForm = lazyComponent(() => import('./SavedSearchUpdateForm'), 'SavedSearchUpdateForm')

interface Props extends NamespaceAreaContext {
    authenticatedUser: AuthenticatedUser
}

const AuthenticatedSavedSearchArea: FunctionComponent<PropsWithChildren<Props>> = ({
    namespace,
    platformContext: { telemetryRecorder },
    authenticatedUser,
    isSourcegraphDotCom,
}) => (
    <Routes>
        <Route path="" element={<SavedSearchListPage namespace={namespace} telemetryRecorder={telemetryRecorder} />} />
        <Route
            path="new"
            element={
                <SavedSearchCreateForm
                    namespace={namespace}
                    authenticatedUser={authenticatedUser}
                    isSourcegraphDotCom={isSourcegraphDotCom}
                    telemetryRecorder={telemetryRecorder}
                />
            }
        />
        <Route
            path=":id"
            element={
                <SavedSearchUpdateForm
                    namespace={namespace}
                    authenticatedUser={authenticatedUser}
                    isSourcegraphDotCom={isSourcegraphDotCom}
                    telemetryRecorder={telemetryRecorder}
                />
            }
        />
        <Route path="*" element={<NotFoundPage pageType="saved search" />} />
    </Routes>
)

export const SavedSearchArea = withAuthenticatedUser(AuthenticatedSavedSearchArea)
