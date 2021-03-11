import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import React from 'react'
import { asError } from '../../../shared/src/util/errors'
import { upperFirst } from 'lodash'
import { Markdown } from '../../../shared/src/components/Markdown'
import { renderMarkdown } from '../../../shared/src/util/markdown'
import classNames from 'classnames'
import { useHistory } from 'react-router'

const renderError = (error: unknown): string =>
    renderMarkdown(upperFirst((asError(error).message || 'Unknown Error').replace(/\t/g, '')), { breaks: true })
        .trim()
        .replace(/^<p>/, '')
        .replace(/<\/p>$/, '')

export const ErrorMessage: React.FunctionComponent<{ error: unknown }> = ({ error }) => {
    const history = useHistory()

    return <Markdown wrapper="span" dangerousInnerHTML={renderError(error)} history={history} />
}

/**
 * Renders a given `Error` object in a Bootstrap danger alert.
 *
 * The error message is optimistically formatted as markdown to enrich links,
 * bullet points, respect line breaks, code and bolded elements.
 * Made to work with Go `multierror`.
 */
export const ErrorAlert: React.FunctionComponent<{
    /**
     * An Error-like object or a string.
     */
    error: unknown

    /**
     * Whether to show an icon.
     *
     * @default true
     */
    icon?: boolean

    /**
     * Optional prefix for the message
     */
    prefix?: string

    className?: string
    style?: React.CSSProperties
}> = ({ error, className, icon = true, prefix, ...rest }) => {
    prefix = prefix?.trim().replace(/:+$/, '')
    return (
        <div className={classNames('alert', 'alert-danger', className)} {...rest}>
            {icon && <AlertCircleIcon className="icon icon-inline" />} {prefix && <strong>{prefix}:</strong>}{' '}
            <ErrorMessage error={error} />
        </div>
    )
}
