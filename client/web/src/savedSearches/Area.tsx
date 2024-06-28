import type { FunctionComponent, PropsWithChildren } from 'react'

import { mdiPlus } from '@mdi/js'
import { Route, Routes } from 'react-router-dom'

import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { Button, Icon, Link, PageHeader } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { withAuthenticatedUser } from '../auth/withAuthenticatedUser'
import { NotFoundPage } from '../components/HeroPage'
import { PageTitle } from '../components/PageTitle'
import type { NamespaceAreaContext } from '../namespaces/NamespaceArea'

import { SavedSearchIcon } from './SavedSearchIcon'

const ListPage = lazyComponent(() => import('./ListPage'), 'ListPage')
const CreateForm = lazyComponent(() => import('./CreateForm'), 'CreateForm')
const UpdateForm = lazyComponent(() => import('./UpdateForm'), 'UpdateForm')

interface Props extends NamespaceAreaContext {
    authenticatedUser: AuthenticatedUser
}

const AuthenticatedArea: FunctionComponent<PropsWithChildren<Props>> = ({
    namespace,
    authenticatedUser,
    platformContext: { telemetryRecorder },
    isSourcegraphDotCom,
}) => (
    <Routes>
        <Route
            path=""
            element={
                <>
                    <PageTitle title="Saved searches" />
                    <PageHeader
                        actions={
                            <Button to="new" variant="primary" as={Link}>
                                <Icon aria-hidden={true} svgPath={mdiPlus} /> New saved search
                            </Button>
                        }
                        className="mb-3"
                        data-testid="saved-searches-list-page"
                    >
                        <PageHeader.Heading as="h3" styleAs="h1" className="mb-1">
                            <PageHeader.Breadcrumb icon={SavedSearchIcon}>Saved Searches</PageHeader.Breadcrumb>
                        </PageHeader.Heading>
                    </PageHeader>
                    <ListPage
                        namespace={namespace}
                        authenticatedUser={authenticatedUser}
                        telemetryRecorder={telemetryRecorder}
                    />
                </>
            }
        />
        <Route
            path="new"
            element={
                <>
                    <PageTitle title="New saved search" />
                    <PageHeader className="mb-3">
                        <PageHeader.Heading as="h3" styleAs="h1" className="mb-1">
                            <PageHeader.Breadcrumb icon={SavedSearchIcon} to={`${namespace.url}/searches`}>
                                Saved Searches
                            </PageHeader.Breadcrumb>
                            <PageHeader.Breadcrumb>New</PageHeader.Breadcrumb>
                        </PageHeader.Heading>
                    </PageHeader>
                    <CreateForm
                        namespace={namespace}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        telemetryRecorder={telemetryRecorder}
                    />
                </>
            }
        />
        <Route
            path=":id"
            element={
                <>
                    <PageTitle title="Edit saved search" />
                    <PageHeader className="mb-3">
                        <PageHeader.Heading as="h3" styleAs="h1" className="mb-1">
                            <PageHeader.Breadcrumb icon={SavedSearchIcon} to={`${namespace.url}/searches`}>
                                Saved Searches
                            </PageHeader.Breadcrumb>
                            <PageHeader.Breadcrumb>Edit</PageHeader.Breadcrumb>
                        </PageHeader.Heading>
                    </PageHeader>
                    <UpdateForm
                        namespace={namespace}
                        authenticatedUser={authenticatedUser}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        telemetryRecorder={telemetryRecorder}
                    />
                </>
            }
        />
        <Route path="*" element={<NotFoundPage pageType="saved search" />} />
    </Routes>
)

/** The saved search area. */
export const Area = withAuthenticatedUser(AuthenticatedArea)
