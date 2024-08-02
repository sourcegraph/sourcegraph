import React from 'react'

import { Select } from '@sourcegraph/wildcard'

import type { BasicFilterArgs, FilterOption } from '../../../../components/FilteredConnection'

import type { EnterprisePortalEnvironment } from './enterpriseportal'

interface Props {
    env: EnterprisePortalEnvironment | undefined
    setEnv: (env: EnterprisePortalEnvironment) => void
}

export const EnterprisePortalEnvSelector: React.FunctionComponent<Props> = ({ env, setEnv }) => (
    <Select
        id="enterprise-portal-env"
        name="enterprise-portal-env"
        onChange={event => {
            setEnv(event.target.value as EnterprisePortalEnvironment)
        }}
        value={env ?? undefined}
        isCustomStyle={true}
        label="Environment"
    >
        {[
            { label: 'Production', value: 'prod' },
            { label: 'Development', value: 'dev' },
        ]
            .concat(window.context.deployType === 'dev' ? [{ label: 'Local', value: 'local' }] : [])
            .map(opt => (
                <option key={opt.value} value={opt.value} label={opt.label} />
            ))}
    </Select>
)

export function getDefaultEnterprisePortalEnv(): EnterprisePortalEnvironment {
    return window.context.deployType === 'dev' ? 'local' : 'prod'
}

export function getEnterprisePortalEnvFilterOptions(): FilterOption<BasicFilterArgs>[] {
    return [
        {
            label: 'Production',
            value: 'prod',
            args: {},
        },
        {
            label: 'Development',
            value: 'dev',
            args: {},
        },
    ].concat(
        window.context.deployType === 'dev'
            ? [
                  {
                      label: 'Local',
                      value: 'local',
                      args: {},
                  },
              ]
            : []
    )
}
