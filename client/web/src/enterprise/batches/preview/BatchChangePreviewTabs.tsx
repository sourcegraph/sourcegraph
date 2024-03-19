import React, { useCallback } from 'react'

import { mdiSourceBranch, mdiFileDocument } from '@mdi/js'
import { useNavigate, useLocation } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Badge, Container, Icon, Tab, TabPanel, TabPanels } from '@sourcegraph/wildcard'

import { resetFilteredConnectionURLQuery } from '../../../components/FilteredConnection'
import type { BatchSpecFields } from '../../../graphql-operations'
import { BatchChangeTabList, BatchChangeTabs } from '../BatchChangeTabs'
import { BatchSpec, BatchSpecDownloadButton } from '../BatchSpec'

import type { PreviewPageAuthenticatedUser } from './BatchChangePreviewPage'
import type {
    queryChangesetSpecFileDiffs as _queryChangesetSpecFileDiffs,
    queryChangesetApplyPreview as _queryChangesetApplyPreview,
} from './list/backend'
import { PreviewList } from './list/PreviewList'

import styles from './BatchChangePreviewTabs.module.scss'

export interface BatchChangePreviewProps extends TelemetryProps, TelemetryV2Props {
    batchSpecID: string
    authenticatedUser: PreviewPageAuthenticatedUser

    /** Used for testing. */
    queryChangesetApplyPreview?: typeof _queryChangesetApplyPreview
    /** Used for testing. */
    queryChangesetSpecFileDiffs?: typeof _queryChangesetSpecFileDiffs
    /** Expand changeset descriptions, for testing only. */
    expandChangesetDescriptions?: boolean
}

interface BatchChangePreviewTabsProps extends BatchChangePreviewProps {
    spec: BatchSpecFields
}

const SPEC_TAB_NAME = 'spec'

export const BatchChangePreviewTabs: React.FunctionComponent<React.PropsWithChildren<BatchChangePreviewTabsProps>> = ({
    authenticatedUser,
    batchSpecID,
    expandChangesetDescriptions,
    queryChangesetApplyPreview,
    queryChangesetSpecFileDiffs,
    spec,
    telemetryRecorder,
}) => {
    // We track the current tab in a URL parameter so that tabs are easy to navigate to
    // and share.
    const navigate = useNavigate()
    const location = useLocation()
    const initialTab = new URLSearchParams(location.search).get('tab')

    const onTabChange = useCallback(
        (index: number) => {
            const urlParameters = new URLSearchParams(location.search)
            resetFilteredConnectionURLQuery(urlParameters)

            // The first tab is the default, so it's not necessary to set it in the URL.
            if (index === 0) {
                urlParameters.delete('tab')
            } else {
                urlParameters.set('tab', SPEC_TAB_NAME)
            }

            navigate({ search: urlParameters.toString() })
        },
        [navigate, location.search]
    )

    return (
        <BatchChangeTabs defaultIndex={initialTab === SPEC_TAB_NAME ? 1 : 0} onChange={onTabChange}>
            <BatchChangeTabList>
                <Tab>
                    <span>
                        <Icon aria-hidden={true} className="text-muted mr-1" svgPath={mdiSourceBranch} />
                        <span className="text-content" data-tab-content="Preview changesets">
                            Preview changesets
                        </span>{' '}
                        <Badge variant="secondary" pill={true} className="ml-1">
                            {spec.applyPreview.totalCount}
                        </Badge>
                    </span>
                </Tab>
                <Tab>
                    <span>
                        <Icon aria-hidden={true} className="text-muted mr-1" svgPath={mdiFileDocument} />{' '}
                        <span className="text-content" data-tab-content="Spec">
                            Spec
                        </span>
                    </span>
                </Tab>
            </BatchChangeTabList>
            <TabPanels>
                <TabPanel>
                    <PreviewList
                        batchSpecID={batchSpecID}
                        authenticatedUser={authenticatedUser}
                        queryChangesetApplyPreview={queryChangesetApplyPreview}
                        queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
                        expandChangesetDescriptions={expandChangesetDescriptions}
                    />
                </TabPanel>
                <TabPanel>
                    <div className="d-flex mb-2 justify-content-end">
                        <BatchSpecDownloadButton
                            name={spec.description.name}
                            originalInput={spec.originalInput}
                            telemetryRecorder={telemetryRecorder}
                        />
                    </div>
                    <Container>
                        <BatchSpec
                            name={spec.description.name}
                            originalInput={spec.originalInput}
                            className={styles.batchSpec}
                            telemetryRecorder={telemetryRecorder}
                        />
                    </Container>
                </TabPanel>
            </TabPanels>
        </BatchChangeTabs>
    )
}
