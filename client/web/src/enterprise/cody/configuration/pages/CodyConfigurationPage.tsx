import { type FC, useCallback, useEffect, useMemo } from 'react'

import { useApolloClient } from '@apollo/client'
import classNames from 'classnames'
import { useLocation, useNavigate } from 'react-router-dom'
import { Subject } from 'rxjs'

import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { RepoLink } from '@sourcegraph/shared/src/components/RepoLink'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Button, Container, ErrorAlert, Link, PageHeader } from '@sourcegraph/wildcard'

import { CodyColorIcon } from '../../../../cody/chat/CodyPageIcon'
import { FilteredConnection, type FilteredConnectionQueryArguments } from '../../../../components/FilteredConnection'
import { PageTitle } from '../../../../components/PageTitle'
import type { CodeIntelligenceConfigurationPolicyFields } from '../../../../graphql-operations'
import { FlashMessage } from '../../../codeintel/configuration/components/FlashMessage'
import { queryPolicies as defaultQueryPolicies } from '../../../codeintel/configuration/hooks/queryPolicies'
import { useDeletePolicies } from '../../../codeintel/configuration/hooks/useDeletePolicies'
import {
    PoliciesNode,
    type UnprotectedPoliciesNodeProps,
} from '../../../codeintel/configuration/pages/CodeIntelConfigurationPage'
import { EmptyPoliciesList } from '../components/EmptyPoliciesList'

import styles from '../../../codeintel/configuration/pages/CodeIntelConfigurationPage.module.scss'

export interface CodyConfigurationPageProps extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    queryPolicies?: typeof defaultQueryPolicies
    repo?: { id: string; name: string }
}

/* eslint-disable  @sourcegraph/sourcegraph/check-help-links */

export const CodyConfigurationPage: FC<CodyConfigurationPageProps> = ({
    authenticatedUser,
    queryPolicies = defaultQueryPolicies,
    repo,
    telemetryService,
    telemetryRecorder,
}) => {
    useEffect(() => {
        telemetryService.logPageView('CodyConfigurationPage')
        if (repo) {
            telemetryRecorder.recordEvent('repo.cody.configuration', 'view')
        } else {
            telemetryRecorder.recordEvent('admin.cody.configuration', 'view')
        }
    }, [telemetryService, telemetryRecorder, repo])

    const navigate = useNavigate()
    const location = useLocation()
    const updates = useMemo(() => new Subject<void>(), [])

    const apolloClient = useApolloClient()
    const queryPoliciesCallback = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryPolicies({ ...args, repository: repo?.id, forEmbeddings: true }, apolloClient),
        [queryPolicies, repo?.id, apolloClient]
    )

    const { handleDeleteConfig, isDeleting, deleteError } = useDeletePolicies()

    const onDelete = useCallback(
        async (id: string, name: string) => {
            if (!window.confirm(`Delete policy ${name}?`)) {
                return
            }

            return handleDeleteConfig({
                variables: { id },
            }).then(() => {
                // Force update of filtered connection
                updates.next()

                navigate(
                    {
                        pathname: './',
                    },
                    {
                        relative: 'path',
                        state: { modal: 'SUCCESS', message: `Configuration policy ${name} has been deleted.` },
                    }
                )
            })
        },
        [handleDeleteConfig, updates, navigate]
    )

    return (
        <>
            <PageTitle title={repo ? 'Embeddings policies for repository' : 'Global embeddings policies'} />
            <PageHeader
                headingElement="h2"
                path={[
                    { icon: CodyColorIcon, text: 'Cody' },
                    {
                        text: repo ? (
                            <>
                                Embeddings policies for <RepoLink repoName={repo.name} to={null} />
                            </>
                        ) : (
                            'Global embeddings policies'
                        ),
                    },
                ]}
                description={
                    <>
                        Rules that control keeping embeddings up-to-date. See the{' '}
                        <Link target="_blank" to="/help/cody/explanations/policies">
                            documentation
                        </Link>{' '}
                        for more details.
                    </>
                }
                actions={
                    authenticatedUser?.siteAdmin && (
                        <Button to="./new?type=head" variant="primary" as={Link}>
                            Create new {!repo && 'global'} policy
                        </Button>
                    )
                }
                className="mb-3"
            />

            {deleteError && <ErrorAlert prefix="Error deleting configuration policy" error={deleteError} />}
            {location.state && <FlashMessage state={location.state.modal} message={location.state.message} />}

            {authenticatedUser?.siteAdmin && repo && (
                <Container className="mb-2">
                    View <Link to="/site-admin/embeddings/configuration">additional configuration policies</Link> that
                    do not affect this repository.
                </Container>
            )}

            <Container className="mb-3 pb-3">
                <FilteredConnection<
                    CodeIntelligenceConfigurationPolicyFields,
                    Omit<UnprotectedPoliciesNodeProps, 'node'>
                >
                    listComponent="div"
                    listClassName={classNames(styles.grid, 'mb-3')}
                    showMoreClassName="mb-0"
                    noun="configuration policy"
                    pluralNoun="configuration policies"
                    nodeComponent={PoliciesNode}
                    nodeComponentProps={{ isDeleting, onDelete, domain: 'embeddings' }}
                    queryConnection={queryPoliciesCallback}
                    cursorPaging={true}
                    inputClassName="ml-2 flex-1"
                    emptyElement={<EmptyPoliciesList />}
                    updates={updates}
                />
            </Container>
        </>
    )
}
