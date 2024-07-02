import React, { useCallback, type ReactNode } from 'react'

import { Select } from '@sourcegraph/wildcard'

import type { ViewerAffiliatedNamespacesResult } from '../graphql-operations'

const NAMESPACE_SELECTOR_ID = 'namespace-selector'

type Namespace = ViewerAffiliatedNamespacesResult['viewer']['affiliatedNamespaces'][number]

export const NamespaceSelector: React.FunctionComponent<{
    namespaces?: Namespace[]
    loading?: boolean

    /** Selected namespace ID. */
    value?: string

    onSelect: (namespace: Namespace['id']) => void

    label?: string
    description?: ReactNode
    disabled?: boolean
    className?: string
}> = ({
    namespaces,
    loading,
    label = 'Namespace',
    description,
    disabled,
    value,
    onSelect: parentOnSelect,
    className,
}) => {
    const onSelect = useCallback<React.ChangeEventHandler<HTMLSelectElement>>(
        event => {
            if (disabled) {
                return
            }

            const selectedNamespace =
                namespaces?.find(namespace => namespace.id === event.target.value) || namespaces?.at(0)
            if (selectedNamespace) {
                parentOnSelect(selectedNamespace.id)
            }
        },
        [disabled, parentOnSelect, namespaces]
    )

    return (
        <Select
            label={<span className="text-nowrap mb-2">{label}</span>}
            description={description}
            isCustomStyle={true}
            id={NAMESPACE_SELECTOR_ID}
            value={value}
            onChange={onSelect}
            disabled={disabled || loading}
            className={className}
        >
            {loading ? (
                <option>Loading...</option>
            ) : (
                namespaces?.map(namespace => (
                    <option key={namespace.id} value={namespace.id}>
                        {(namespace.__typename === 'Org' && namespace.displayName) || namespace.namespaceName}
                    </option>
                ))
            )}
        </Select>
    )
}
