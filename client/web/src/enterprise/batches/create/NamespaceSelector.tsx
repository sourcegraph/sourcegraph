import React, { useCallback, type ReactNode } from 'react'

import type { OrgSettingFields, UserSettingFields } from '@sourcegraph/shared/src/graphql-operations'
import { Select } from '@sourcegraph/wildcard'

import type { PartialNamespace } from '../../../namespaces'

const NAMESPACE_SELECTOR_ID = 'batch-spec-execution-namespace-selector'

interface NamespaceSelectorProps {
    namespaces: PartialNamespace[]
    selectedNamespace: string
    label?: string
    description?: ReactNode
    disabled?: boolean
    onSelect: (namespace: PartialNamespace) => void
    className?: string
}

export const NamespaceSelector: React.FunctionComponent<React.PropsWithChildren<NamespaceSelectorProps>> = ({
    namespaces,
    label = 'Namespace',
    description,
    disabled,
    selectedNamespace,
    onSelect,
    className,
}) => {
    const onSelectNamespace = useCallback<React.ChangeEventHandler<HTMLSelectElement>>(
        event => {
            if (disabled) {
                return
            }

            const selectedNamespace = namespaces.find(
                (namespace): namespace is UserSettingFields | OrgSettingFields => namespace.id === event.target.value
            )
            onSelect(selectedNamespace || namespaces[0])
        },
        [disabled, onSelect, namespaces]
    )

    return (
        <Select
            label={<span className="text-nowrap mb-2">{label}</span>}
            description={description}
            isCustomStyle={true}
            id={NAMESPACE_SELECTOR_ID}
            value={selectedNamespace}
            onChange={onSelectNamespace}
            disabled={disabled}
            className={className}
        >
            {namespaces.map(namespace => (
                <option key={namespace.id} value={namespace.id}>
                    {namespace.displayName || namespace.namespaceName}
                </option>
            ))}
        </Select>
    )
}
