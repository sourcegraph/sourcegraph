import type { FunctionComponent, PropsWithChildren } from 'react'

import { Route, Routes } from 'react-router-dom'

import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { AuthenticatedUser } from '../auth'
import { withAuthenticatedUser } from '../auth/withAuthenticatedUser'
import { NotFoundPage } from '../components/HeroPage'
import type { NamespaceAreaContext } from '../namespaces/NamespaceArea'

const WorkflowListPage = lazyComponent(() => import('./WorkflowListPage'), 'WorkflowListPage')
const WorkflowCreateForm = lazyComponent(() => import('./WorkflowCreateForm'), 'WorkflowCreateForm')
const WorkflowUpdateForm = lazyComponent(() => import('./WorkflowUpdateForm'), 'WorkflowUpdateForm')

interface Props extends NamespaceAreaContext {
    authenticatedUser: AuthenticatedUser
}

const AuthenticatedWorkflowArea: FunctionComponent<PropsWithChildren<Props>> = ({
    namespace,
    platformContext: { telemetryRecorder },
}) => (
    <Routes>
        <Route path="" element={<WorkflowListPage namespace={namespace} telemetryRecorder={telemetryRecorder} />} />
        <Route
            path="new"
            element={<WorkflowCreateForm namespace={namespace} telemetryRecorder={telemetryRecorder} />}
        />
        <Route
            path=":id"
            element={<WorkflowUpdateForm namespace={namespace} telemetryRecorder={telemetryRecorder} />}
        />
        <Route path="*" element={<NotFoundPage pageType="workflow" />} />
    </Routes>
)

export const WorkflowArea = withAuthenticatedUser(AuthenticatedWorkflowArea)
