import { useState } from 'react'

import { useMutation } from '@sourcegraph/http-client'
import { ErrorAlert } from '@sourcegraph/wildcard'

import type { FilterOption } from '../../../components/FilteredConnection'
import type {
    PackageMatchBehaviour,
    PackageRepoFilterFields,
    UpdatePackageRepoFilterResult,
    UpdatePackageRepoFilterVariables,
} from '../../../graphql-operations'
import { updatePackageRepoFilterMutation } from '../backend'
import { BehaviourSelect } from '../components/BehaviourSelect'
import { MultiPackageForm, type MultiPackageState } from '../components/MultiPackageForm'
import { SinglePackageForm, type SinglePackageState } from '../components/SinglePackageForm'

import type { BlockType } from './AddPackageFilterModalContent'

import styles from './AddPackageFilterModalContent.module.scss'

const getInitialState = (packageFilter: PackageRepoFilterFields): SinglePackageState | MultiPackageState => {
    const nameGlob = packageFilter.nameFilter?.packageGlob || ''

    if (nameGlob !== '') {
        return {
            ecosystem: packageFilter.kind,
            nameFilter: nameGlob,
        }
    }

    if (packageFilter.versionFilter) {
        return {
            ecosystem: packageFilter.kind,
            name: packageFilter.versionFilter.packageName,
            versionFilter: packageFilter.versionFilter.versionGlob,
        }
    }

    throw new Error(`Unable to find filter for package filter ${packageFilter.id}`)
}

export interface EditPackageFilterModalContentProps {
    packageFilter: PackageRepoFilterFields
    filters: FilterOption[]
    onDismiss: () => void
}

export const EditPackageFilterModalContent: React.FunctionComponent<EditPackageFilterModalContentProps> = ({
    packageFilter,
    filters,
    onDismiss,
}) => {
    const [behaviour, setBehaviour] = useState<PackageMatchBehaviour>(packageFilter.behaviour)
    const initialState = getInitialState(packageFilter)
    const [blockType, setBlockType] = useState<BlockType>('name' in initialState ? 'single' : 'multiple')

    const [updatePackageRepoFilter, { error }] = useMutation<
        UpdatePackageRepoFilterResult,
        UpdatePackageRepoFilterVariables
    >(updatePackageRepoFilterMutation, { onCompleted: onDismiss })

    return (
        <div className={styles.content}>
            <BehaviourSelect value={behaviour} onChange={setBehaviour} />
            {blockType === 'single' ? (
                <SinglePackageForm
                    initialState={initialState as SinglePackageState}
                    filters={filters}
                    setType={setBlockType}
                    onDismiss={onDismiss}
                    onSave={blockState =>
                        updatePackageRepoFilter({
                            variables: {
                                behaviour,
                                id: packageFilter.id,
                                kind: blockState.ecosystem,
                                filter: {
                                    versionFilter: {
                                        packageName: blockState.name,
                                        versionGlob: blockState.versionFilter,
                                    },
                                },
                            },
                        })
                    }
                />
            ) : (
                <MultiPackageForm
                    initialState={initialState as MultiPackageState}
                    filters={filters}
                    setType={setBlockType}
                    onDismiss={onDismiss}
                    onSave={blockState =>
                        updatePackageRepoFilter({
                            variables: {
                                behaviour,
                                id: packageFilter.id,
                                kind: blockState.ecosystem,
                                filter: {
                                    nameFilter: {
                                        packageGlob: blockState.nameFilter,
                                    },
                                },
                            },
                        })
                    }
                />
            )}
            {error && <ErrorAlert error={error} />}
        </div>
    )
}
