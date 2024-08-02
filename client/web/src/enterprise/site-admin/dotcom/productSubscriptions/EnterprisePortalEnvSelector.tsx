import React from 'react'

import { mdiInformationOutline } from '@mdi/js'

import { Icon, Select, Tooltip } from '@sourcegraph/wildcard'

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
        selectSize="sm"
        className="mr-2 ml-2 mb-0 mt-0"
        label={
            <>
                Enterprise Portal{' '}
                <Tooltip content="Selects the Enterprise Portal server to interact with.">
                    <Icon aria-label="Show help text" svgPath={mdiInformationOutline} />
                </Tooltip>
            </>
        }
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
