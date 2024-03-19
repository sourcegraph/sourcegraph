import { useEffect, useMemo, useRef, useState } from 'react'

import classNames from 'classnames'

import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { H4, H5, RadioButton, Text, Button, Icon, Link } from '@sourcegraph/wildcard'

import { CodyColorIcon, CodySpeechBubbleIcon } from '../chat/CodyPageIcon'

import { ScopeSelector } from './ScopeSelector'
import type { ScopeSelectorProps } from './ScopeSelector/ScopeSelector'

import styles from './GettingStarted.module.scss'

type ConversationScope = 'general' | 'repo'

const DEFAULT_VERTICAL_OFFSET = '1rem'

export const GettingStarted: React.FC<
    ScopeSelectorProps & {
        isCodyChatPage?: boolean
        submitInput: (input: string, submitType: 'user' | 'suggestion' | 'example') => void
        authenticatedUser: AuthenticatedUser | null
    }
> = ({ isCodyChatPage, submitInput, authenticatedUser, ...scopeSelectorProps }) => {
    const [conversationScope, setConversationScope] = useState<ConversationScope>(
        !isCodyChatPage || scopeSelectorProps.scope.repositories.length > 0 ? 'repo' : 'general'
    )

    /*
    When content is vertically centered inside the container using CSS,
    any content height change (e.g. conditional rendering of additional examples, etc.)
    causes content top and bottom positions to change. This results in a "jumping" effect and not-optimal UX
    when interacting with conversation scope radio group.
    In order to avoid this, we calculate the vertical offset of the content and apply it as a margin. In this case
    when content height changes, the top position remains the same and only the bottom position changes.
    */
    const [contentVerticalOffset, setContentVerticalOffset] = useState<string>(DEFAULT_VERTICAL_OFFSET)
    const containerRef = useRef<HTMLDivElement>(null)
    const contentRef = useRef<HTMLDivElement>(null)
    useEffect(() => {
        const updateVerticalOffset = (): void =>
            setContentVerticalOffset(() => {
                if (!containerRef.current || !contentRef.current) {
                    return DEFAULT_VERTICAL_OFFSET
                }

                const containerHeight = containerRef.current.getBoundingClientRect().height
                const contentHeight = contentRef.current.getBoundingClientRect().height

                if (containerHeight <= contentHeight) {
                    return DEFAULT_VERTICAL_OFFSET
                }

                return `${(containerHeight - contentHeight) / 2}px`
            })
        updateVerticalOffset()
        window.addEventListener('resize', updateVerticalOffset)
        return () => window.removeEventListener('resize', updateVerticalOffset)
    }, [])

    useEffect(() => {
        if (scopeSelectorProps.scope.repositories.length > 0) {
            setConversationScope('repo')
        }
    }, [scopeSelectorProps.scope.repositories.length])

    const content: { title: string; examples: { label?: string; text: string }[] } = useMemo(() => {
        if (conversationScope === 'repo') {
            return {
                title: `Examples to start with${
                    scopeSelectorProps.scope.repositories.length === 1
                        ? ` for ${scopeSelectorProps.scope.repositories[0].split('/').slice(-2).join('/')}`
                        : ''
                }`,
                examples: [
                    {
                        text: 'What is the tech stack of this repo?',
                    },
                    {
                        text: 'Are there any automated tests in this repo?',
                    },
                    {
                        text: 'Can you describe the overall structure of this repo?',
                    },
                ],
            }
        }

        return {
            title: 'General coding examples to start with',
            examples: [
                {
                    label: 'Algorithms',
                    text: 'Can you explain how the QuickSort algorithm works in Python?',
                },
                {
                    label: 'Best practices',
                    text: "I'm working on a large-scale web application using React. What are some best practices or design patterns I should be aware of to maintain code readability and performance?",
                },
                {
                    label: 'Guidance',
                    text: "I'm trying to build a RESTful API using Node.js and Express. Can you provide an example of how to implement JWT authentication in this context?",
                },
            ],
        }
    }, [conversationScope, scopeSelectorProps.scope.repositories])

    return (
        <div ref={containerRef} className={styles.container}>
            {/* eslint-disable-next-line react/forbid-dom-props */}
            <div ref={contentRef} style={{ margin: `${contentVerticalOffset} 20%` }}>
                {isCodyChatPage ? null : (
                    <div className={styles.iconSection}>
                        <Icon as={CodyColorIcon} aria-hidden="true" className={styles.codyIcon} />
                        <div className={styles.greetingContainer}>
                            <Icon as={CodySpeechBubbleIcon} className="h-auto w-auto" aria-hidden="true" />
                            <Text as="span" className={styles.greetingText}>
                                Hi! I'm Cody
                            </Text>
                        </div>
                    </div>
                )}

                {isCodyChatPage ? (
                    <div className={classNames(styles.section, 'mb-3')}>
                        <fieldset>
                            <legend>
                                <H4 className="mb-1">Choose the context for this conversation</H4>
                            </legend>

                            <div className={styles.radioWrapper}>
                                <RadioButton
                                    id="general"
                                    name="general"
                                    label={
                                        <Text as="span" size="small">
                                            General knowledge
                                        </Text>
                                    }
                                    value="general"
                                    wrapperClassName="d-flex align-items-center"
                                    checked={conversationScope === 'general'}
                                    onChange={event => setConversationScope(event.target.value as ConversationScope)}
                                />
                            </div>
                            <div>
                                <RadioButton
                                    id="repo"
                                    name="repo"
                                    label={
                                        <Text as="span" size="small">
                                            Specific repositories:
                                        </Text>
                                    }
                                    value="repo"
                                    wrapperClassName="d-flex align-items-center mb-1"
                                    checked={conversationScope === 'repo'}
                                    onChange={event => setConversationScope(event.target.value as ConversationScope)}
                                />
                                <div className={styles.scopeSelectorWrapper}>
                                    <ScopeSelector
                                        {...scopeSelectorProps}
                                        encourageOverlap={true}
                                        authenticatedUser={authenticatedUser}
                                    />
                                </div>

                                {scopeSelectorProps.scope.repositories.length === 0 && (
                                    <>
                                        <hr className={styles.divider} />
                                        <Text size="small" className={classNames('text-muted', styles.hintTitle)}>
                                            Why is context important?
                                        </Text>
                                        <Text size="small" className={classNames('text-muted', styles.hintText)}>
                                            Without providing relevant repo(s) for context, Cody won't be able to answer
                                            questions specific to your project.
                                        </Text>

                                        <Text size="small" className="mb-0 text-muted">
                                            <Text as="span" weight="bold">
                                                Tip:
                                            </Text>{' '}
                                            The context selector is always available at the bottom of the screen
                                        </Text>
                                    </>
                                )}
                            </div>
                        </fieldset>
                    </div>
                ) : null}

                <div className={classNames(styles.section, 'mb-3')}>
                    <H5>{content.title}</H5>
                    <hr className={styles.divider} />
                    {content.examples.map(({ label, text }) => (
                        <div key={text} className={styles.exampleWrapper}>
                            {label ? (
                                <Text size="small" className="mb-1 text-muted">
                                    {label}
                                </Text>
                            ) : null}
                            <Button
                                variant="link"
                                size="sm"
                                outline={false}
                                className="p-0 text-left"
                                onClick={() => submitInput(text, 'example')}
                            >
                                {text}
                            </Button>
                        </div>
                    ))}
                </div>

                <Text alignment="center" size="small">
                    By using Cody, you agree to its{' '}
                    <Link to="https://sourcegraph.com/terms/cody-notice">license and privacy statement</Link>.
                </Text>
            </div>
        </div>
    )
}
