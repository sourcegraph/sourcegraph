import classNames from 'classnames'
import PencilOutlineIcon from 'mdi-react/PencilOutlineIcon'
import React, { useEffect, useRef, useState } from 'react'

import styles from './NotebookTitle.module.scss'

export interface NotebookTitleProps {
    title: string
    viewerCanManage: boolean
    onUpdateTitle: (title: string) => void
}

export const NotebookTitle: React.FunctionComponent<NotebookTitleProps> = ({
    title: initialTitle,
    viewerCanManage,
    onUpdateTitle,
}) => {
    const [isEditing, setIsEditing] = useState(false)
    const [title, setTitle] = useState(initialTitle)
    const inputReference = useRef<HTMLInputElement>(null)

    const updateTitle = (): void => {
        setIsEditing(false)
        onUpdateTitle(title)
    }

    const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>): void => {
        if (event.key === 'Escape' || event.key === 'Enter') {
            updateTitle()
        }
    }

    useEffect(() => {
        if (!isEditing) {
            return
        }
        inputReference.current?.focus()
    }, [isEditing])

    if (!viewerCanManage) {
        return <span>{title}</span>
    }

    if (!isEditing) {
        const onButtonKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>): void => {
            if (event.key === 'Enter') {
                setIsEditing(true)
            }
        }

        return (
            <button
                type="button"
                className={styles.titleButton}
                onClick={() => setIsEditing(true)}
                onKeyDown={onButtonKeyDown}
                data-testid="notebook-title-button"
            >
                <span>{title}</span>
                <span className={styles.titleEditIcon}>
                    <PencilOutlineIcon className="icon-inline" />
                </span>
            </button>
        )
    }

    return (
        <input
            ref={inputReference}
            className={classNames('form-control', styles.titleInput)}
            type="text"
            value={title}
            onChange={event => setTitle(event.target.value)}
            onKeyDown={onKeyDown}
            data-testid="notebook-title-input"
        />
    )
}
