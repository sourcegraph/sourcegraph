import { useCallback, useState } from 'react'

import { mdiPlus } from '@mdi/js'
import classNames from 'classnames'

import { toRepoURL } from '@sourcegraph/shared/src/util/url'
import {
    Badge,
    Button,
    ErrorAlert,
    Form,
    Icon,
    Input,
    Label,
    Link,
    LoadingSpinner,
    Select,
    Tooltip,
    useDebounce,
} from '@sourcegraph/wildcard'

import type { FilterOption } from '../../../components/FilteredConnection'
import type { PackageRepoMatchFields, PackageRepoReferenceKind } from '../../../graphql-operations'
import { useMatchingPackages } from '../hooks/useMatchingPackages'
import { useMatchingVersions } from '../hooks/useMatchingVersions'
import type { BlockType } from '../modal-content/AddPackageFilterModalContent'

import { FilterPackagesActions } from './FilterPackagesActions'

import styles from '../modal-content/AddPackageFilterModalContent.module.scss'

export interface SinglePackageState {
    name: string
    ecosystem: PackageRepoReferenceKind
    versionFilter: string
}

interface SinglePackageFormProps {
    initialState: SinglePackageState
    filters: FilterOption[]
    setType: (type: BlockType) => void
    onDismiss: () => void
    onSave: (state: SinglePackageState) => Promise<unknown>
}

export const SinglePackageForm: React.FunctionComponent<SinglePackageFormProps> = ({
    initialState,
    filters,
    setType,
    onDismiss,
    onSave,
}) => {
    const [blockState, setBlockState] = useState<SinglePackageState>(initialState)

    const nameQuery = useDebounce(blockState.name, 200)
    const versionQuery = useDebounce(blockState.versionFilter, 200)

    const { nodes } = useMatchingPackages({
        first: 1,
        kind: blockState.ecosystem,
        filter: {
            nameFilter: {
                packageGlob: nameQuery,
            },
        },
    })

    const isValid = useCallback((): boolean => {
        if (blockState.versionFilter === '' || blockState.name === '') {
            return false
        }

        return true
    }, [blockState])

    const handleSubmit = useCallback(
        (event: React.FormEvent<HTMLFormElement>): Promise<unknown> => {
            event.preventDefault()

            if (!isValid()) {
                return Promise.resolve()
            }

            return onSave(blockState)
        },
        [blockState, isValid, onSave]
    )

    return (
        <Form onSubmit={handleSubmit} className="w-100 mb-3">
            <div>
                <Label className="mb-2" id="package-name">
                    Name
                </Label>
                <div className={styles.inputRow}>
                    <Select
                        name="single-ecosystem-select"
                        className={classNames('mr-1 mb-0', styles.ecosystemSelect)}
                        value={blockState.ecosystem}
                        isCustomStyle={true}
                        required={true}
                        aria-label="Ecosystem"
                    >
                        {filters.map(({ label, value }) => (
                            <option value={value} key={label}>
                                {label}
                            </option>
                        ))}
                    </Select>
                    <Input
                        name="single-package-input"
                        className="mr-2 flex-1"
                        value={blockState.name}
                        onChange={event => setBlockState({ ...blockState, name: event.target.value })}
                        required={true}
                        aria-labelledby="package-name"
                    />
                    <Tooltip content="Block multiple packages at once">
                        <Button
                            className={styles.inputRowButton}
                            variant="secondary"
                            outline={true}
                            onClick={() => setType('multiple')}
                        >
                            <Icon aria-hidden={true} svgPath={mdiPlus} className="mr-1" />
                            Filter
                        </Button>
                    </Tooltip>
                </div>
            </div>
            <div className="mt-3">
                <Label className="mb-2" id="package-version">
                    Version
                </Label>
                <div className={styles.inputRow}>
                    <Input
                        name="multi-version-input"
                        aria-labelledby="package-version"
                        className="mr-2 flex-1"
                        value={blockState.versionFilter || ''}
                        placeholder="e.g. v1.*"
                        required={true}
                        onChange={event => setBlockState({ ...blockState, versionFilter: event.target.value })}
                    />
                </div>
            </div>
            <VersionFilterSummary
                blockState={blockState}
                versionQuery={versionQuery}
                nameQuery={nameQuery}
                node={nodes[0]}
            />
            <FilterPackagesActions valid={isValid()} onDismiss={onDismiss} />
        </Form>
    )
}

interface VersionFilterSummaryProps {
    node: PackageRepoMatchFields
    blockState: SinglePackageState
    nameQuery: string
    versionQuery: string
}
const VersionFilterSummary: React.FunctionComponent<VersionFilterSummaryProps> = ({
    blockState,
    nameQuery,
    versionQuery,
    node,
}) => {
    const [versionFetchLimit, setVersionFetchLimit] = useState(15)
    const { versions, totalCount, loading, error } = useMatchingVersions({
        variables: {
            kind: blockState.ecosystem,
            filter: {
                versionFilter: {
                    packageName: nameQuery,
                    versionGlob: versionQuery,
                },
            },
            first: versionFetchLimit,
        },
        skip: !node,
    })

    // Limit fetching more than 1000 versions
    const nextFetchLimit = Math.min(totalCount, 1000)

    if (loading) {
        return <LoadingSpinner className="d-block mx-auto mt-2" />
    }

    if (error) {
        return <ErrorAlert error={error} className="mt-2" />
    }

    return (
        <div className="mt-3">
            <Label className="mb-2">Summary</Label>
            <div className="d-flex justify-content-between text-muted">
                <span>
                    {!node ? (
                        <>No package currently matches this filter</>
                    ) : (
                        <>
                            1 package currently matches this filter, across{' '}
                            {totalCount === 1 ? <>{totalCount} version</> : <>{totalCount} versions</>}
                            {versions.length < totalCount && <> (showing only {versions.length})</>}
                        </>
                    )}
                </span>
                {versions.length < totalCount && (
                    <Button variant="link" className="p-0 mr-3" onClick={() => setVersionFetchLimit(nextFetchLimit)}>
                        <>Show {nextFetchLimit === totalCount ? 'all ' : nextFetchLimit.toString()}</>
                    </Button>
                )}
            </div>
            {node && (
                <ul className={classNames('list-group mt-1', styles.list)}>
                    {versions.map(version => (
                        <li className="list-group-item" key={version}>
                            {node?.repository ? (
                                <div className="d-flex justify-content-between">
                                    <Link
                                        to={toRepoURL({
                                            repoName: node.repository.name,
                                            revision: `v${version}`,
                                        })}
                                    >
                                        <Badge className="px-2 py-0" as="code">
                                            {node.name}@{version}
                                        </Badge>
                                    </Link>
                                </div>
                            ) : (
                                <Badge className="px-2 py-0" as="code">
                                    {node.name}@{version}
                                </Badge>
                            )}
                        </li>
                    ))}
                </ul>
            )}
        </div>
    )
}
