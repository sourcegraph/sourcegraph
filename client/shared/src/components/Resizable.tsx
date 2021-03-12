/* eslint-disable react/forbid-dom-props */
import classnames from 'classnames'
import React, { useRef, useCallback, useEffect } from 'react'
import { useLocalStorage } from '../util/useLocalStorage'

interface Props {
    /**
     * The element that is resizable.
     */
    children: JSX.Element
    /**
     * The kru for preserve the size on local storage
     */
    storageKey: string
    /**
     * The default size for the element.
     */
    defaultSize: number
    /**
     * Where the resize handle is (which also determines the axis along which the element can be
     * resized).
     */
    position: 'right' | 'left' | 'top'
}

export const Resizable: React.FunctionComponent<Props> = ({ children, defaultSize, position, storageKey }) => {
    const NAME = `Resizable:${storageKey}`
    const [isResizable, setIsResizable] = React.useState(false)
    const [size, setSize] = useLocalStorage(NAME, defaultSize)
    const reference = useRef<HTMLDivElement>(null)
    const onMouseUp = useCallback(() => setIsResizable(false), [])
    const onMouseDown = useCallback(() => setIsResizable(true), [])

    const onMouseMove = useCallback(
        (event: MouseEvent): void => {
            if (isResizable && reference.current) {
                if (position === 'right') {
                    setSize(event.pageX - reference.current.getBoundingClientRect().left)
                } else if (position === 'left') {
                    setSize(reference.current.getBoundingClientRect().right - event.pageX)
                } else if (position === 'top') {
                    setSize(reference.current.getBoundingClientRect().bottom - event.pageY)
                }
            }
        },
        [isResizable, position, setSize]
    )

    useEffect(() => {
        document.addEventListener('mousemove', onMouseMove)
        document.addEventListener('mouseup', onMouseUp)

        return () => {
            document.removeEventListener('mousemove', onMouseMove)
            document.removeEventListener('mouseup', onMouseUp)
        }
    }, [isResizable, onMouseMove, onMouseUp])

    return (
        <div
            className={classnames(
                {
                    'flex-column-reverse': position === 'top',
                    'justify-content-end': position === 'top',
                    'h-100': position === 'left',
                },
                'd-flex overflow-scroll'
            )}
            ref={reference}
            style={{ [position !== 'top' ? 'width' : 'height']: `${size}px`, minWidth: '6rem', minHeight: '6rem' }}
        >
            {children}
            <div
                onMouseDown={onMouseDown}
                className={classnames(
                    {
                        'resizable--top border-top': position === 'top',
                        'resizable--left border-left': position === 'left',
                        'resizable--right border-right': position === 'right',
                    },
                    'resizable',
                    'd-flex'
                )}
                aria-hidden={true}
            />
        </div>
    )
}
