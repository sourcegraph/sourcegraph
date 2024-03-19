import { Suspense, type PropsWithChildren, type FC, useCallback, type ChangeEvent } from 'react'

import classNames from 'classnames'

import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { Input } from '@sourcegraph/wildcard'

import type { V2SearchInputProps } from './V2SearchInput'

import styles from './LazyV2SearchInput.module.scss'

const V2SearchInput = lazyComponent(() => import('./V2SearchInput'), 'V2SearchInput')

export const LazyV2SearchInput: FC<PropsWithChildren<V2SearchInputProps>> = props => (
    <Suspense fallback={<PlainQueryInput {...props} />}>
        <V2SearchInput {...props} />
    </Suspense>
)

/**
 * A plain query input displayed during lazy-loading of the LazyQueryInput. It has no suggestions
 * but still allows typing and submitting queries.
 */
const PlainQueryInput: FC<PropsWithChildren<Pick<V2SearchInputProps, 'queryState' | 'onChange' | 'className'>>> = ({
    queryState,
    onChange,
    className,
}) => {
    const onInputChange = useCallback(
        (event: ChangeEvent<HTMLInputElement>) => {
            onChange({ query: event.target.value })
        },
        [onChange]
    )
    return (
        <Input
            value={queryState.query}
            spellCheck={false}
            placeholder="Search for code or files..."
            className="w-100"
            inputClassName={classNames('text-code', styles.intermediateInput, className)}
            onChange={onInputChange}
        />
    )
}
