import * as H from 'history'
import React, { FunctionComponent } from 'react'

import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'
import { Container, Tab, TabList, TabPanel, TabPanels, Tabs } from '@sourcegraph/wildcard'

import { ConfigurationEditor } from './ConfigurationEditor'
import { RepositoryPolicies } from './RepositoryPolicies'
import { RepositoryTab } from './RepositoryTab'

export interface RepositoryConfigurationProps extends ThemeProps, TelemetryProps {
    repo: { id: string }
    indexingEnabled: boolean
    history: H.History
    onHandleDisplayAction: React.Dispatch<React.SetStateAction<boolean>>
    onHandleIsDeleting: React.Dispatch<React.SetStateAction<boolean>>
    onHandleIsLoading: React.Dispatch<React.SetStateAction<boolean>>
}

export const RepositoryConfiguration: FunctionComponent<RepositoryConfigurationProps> = ({
    repo,
    indexingEnabled,
    history,
    onHandleDisplayAction,
    onHandleIsDeleting,
    onHandleIsLoading,
    ...props
}) => (
    <Tabs size="medium">
        <TabList>
            <RepositoryTab onHandleDisplayAction={onHandleDisplayAction}>Repository-specific policies</RepositoryTab>
            <RepositoryTab onHandleDisplayAction={onHandleDisplayAction}>Global policies</RepositoryTab>
            {indexingEnabled && <Tab>Index configuration</Tab>}
        </TabList>

        <TabPanels>
            <TabPanel>
                <RepositoryPolicies
                    isGlobal={false}
                    repo={repo}
                    indexingEnabled={indexingEnabled}
                    history={history}
                    onHandleDisplayAction={onHandleDisplayAction}
                    onHandleIsDeleting={onHandleIsDeleting}
                    onHandleIsLoading={onHandleIsLoading}
                />
            </TabPanel>

            <TabPanel>
                <RepositoryPolicies
                    isGlobal={true}
                    repo={repo}
                    indexingEnabled={indexingEnabled}
                    history={history}
                    onHandleDisplayAction={onHandleDisplayAction}
                    onHandleIsDeleting={onHandleIsDeleting}
                    onHandleIsLoading={onHandleIsLoading}
                />
            </TabPanel>

            {indexingEnabled && (
                <TabPanel>
                    <Container>
                        <h3>Auto-indexing configuration</h3>

                        <ConfigurationEditor repoId={repo.id} history={history} {...props} />
                    </Container>
                </TabPanel>
            )}
        </TabPanels>
    </Tabs>
)
