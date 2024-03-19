import React from 'react'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { UserExecutorSecretsListPageProps } from './secrets/ExecutorSecretsListPage'

const UserExecutorSecretsListPage = lazyComponent<UserExecutorSecretsListPageProps, 'UserExecutorSecretsListPage'>(
    () => import('./secrets/ExecutorSecretsListPage'),
    'UserExecutorSecretsListPage'
)

export interface ExecutorsUserAreaProps extends TelemetryV2Props {
    namespaceID: string
}

/** The page area for all executors settings in user settings. */
export const ExecutorsUserArea: React.FunctionComponent<React.PropsWithChildren<ExecutorsUserAreaProps>> = props => (
    <UserExecutorSecretsListPage userID={props.namespaceID} {...props} />
)
