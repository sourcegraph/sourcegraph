import { Modal, PageHeader } from '@sourcegraph/wildcard'

import type { FilterOption } from '../../components/FilteredConnection'

import {
    AddPackageFilterModalContent,
    type AddPackageFilterModalContentProps,
} from './modal-content/AddPackageFilterModalContent'

import styles from './PackagesModal.module.scss'

interface AddFilterModalProps extends AddPackageFilterModalContentProps {
    onDismiss: () => void
    filters: FilterOption[]
}

export const AddFilterModal: React.FunctionComponent<AddFilterModalProps> = props => (
    <Modal aria-label="Add package filters" onDismiss={props.onDismiss} className={styles.modal}>
        <PageHeader path={[{ text: 'Add package filter' }]} headingElement="h2" className={styles.header} />
        <AddPackageFilterModalContent node={props.node} filters={props.filters} onDismiss={props.onDismiss} />
    </Modal>
)
