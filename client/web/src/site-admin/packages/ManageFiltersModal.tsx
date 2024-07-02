import { useState } from 'react'

import { Button, Modal, PageHeader } from '@sourcegraph/wildcard'

import type { FilterOption } from '../../components/FilteredConnection'
import type { PackageRepoFilterFields } from '../../graphql-operations'

import { EditPackageFilterModalContent } from './modal-content/EditPackageFilterModalContent'
import {
    ManagePackageFiltersModalContent,
    type ManagePackageFiltersModalContentProps,
} from './modal-content/ManagePackageFiltersModalContent'

import styles from './PackagesModal.module.scss'

interface ManageFiltersModalProps extends Omit<ManagePackageFiltersModalContentProps, 'setActiveFilter'> {
    onDismiss: () => void
    onAdd: () => void
    filters: FilterOption[]
}

export const ManageFiltersModal: React.FunctionComponent<ManageFiltersModalProps> = props => {
    const [activeFilter, setActiveFilter] = useState<PackageRepoFilterFields>()

    return (
        <Modal aria-label="Manage package filters" onDismiss={props.onDismiss} className={styles.modal}>
            {activeFilter ? (
                <>
                    <PageHeader
                        path={[{ text: 'Edit package filter' }]}
                        headingElement="h2"
                        className={styles.header}
                    />
                    <EditPackageFilterModalContent
                        packageFilter={activeFilter}
                        filters={props.filters}
                        onDismiss={props.onDismiss}
                    />
                </>
            ) : (
                <>
                    <PageHeader
                        path={[{ text: 'Manage package filters' }]}
                        headingElement="h2"
                        className={styles.header}
                        actions={
                            <Button variant="secondary" outline={true} onClick={props.onAdd}>
                                Add filter
                            </Button>
                        }
                    />
                    <ManagePackageFiltersModalContent setActiveFilter={setActiveFilter} onDismiss={props.onDismiss} />
                </>
            )}
        </Modal>
    )
}
