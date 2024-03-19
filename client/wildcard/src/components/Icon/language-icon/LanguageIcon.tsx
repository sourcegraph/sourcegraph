import type { FC } from 'react'

import { mdiFileDocumentOutline } from '@mdi/js'
import classNames from 'classnames'

import { Icon } from '../Icon'

import { getFileIconInfo } from './language-icons'

import styles from './LanguageIcon.module.scss'

interface LanguageIconProps {
    language: string
    fileNameOrExtensions?: string
    defaultIcon?: string
    className?: string
}

export const LanguageIcon: FC<LanguageIconProps> = props => {
    const { fileNameOrExtensions, language, defaultIcon = mdiFileDocumentOutline, className } = props

    const fileIcon = getFileIconInfo(fileNameOrExtensions ?? '', language)

    if (fileIcon) {
        return (
            <Icon
                as={fileIcon.react.icon}
                className={classNames(styles.icon, fileIcon.react.className, className)}
                aria-hidden={true}
            />
        )
    }

    return <Icon svgPath={defaultIcon} className={classNames(styles.icon, className)} aria-hidden={true} />
}
