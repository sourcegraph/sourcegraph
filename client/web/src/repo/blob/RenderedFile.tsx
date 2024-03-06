import { forwardRef } from 'react'

import classNames from 'classnames'
import { useLocation } from 'react-router-dom'

import { Markdown } from '@sourcegraph/wildcard'

import { useScrollToLocationHash } from '../../components/useScrollToLocationHash'

import styles from './RenderedFile.module.scss'

interface Props {
    dangerousInnerHTML: string
    className?: string
}

/**
 * Displays a file whose contents are rendered to HTML, such as a Markdown file.
 */
export const RenderedFile = forwardRef<HTMLDivElement, Props>(function RenderedFile(props, reference) {
    const { dangerousInnerHTML, className } = props

    const location = useLocation()
    useScrollToLocationHash(location)

    return (
        <div ref={reference} className={classNames(styles.renderedFile, className)}>
            <div className={styles.container}>
                <Markdown dangerousInnerHTML={dangerousInnerHTML} enableMathJax={true} />
            </div>
        </div>
    )
})
