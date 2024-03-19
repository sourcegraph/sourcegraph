import React, { useCallback } from 'react'

import { mdiClose } from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import classNames from 'classnames'

import { Button, H3, Icon, Input, Link, Select, Text, Form, Code } from '@sourcegraph/wildcard'

import { isProjectPathValid } from './build-url'
import type { EditorSettings } from './editor-settings'
import { type EditorId, supportedEditors } from './editors'

import styles from './OpenInEditorPopover.module.scss'

export interface OpenInEditorPopoverProps {
    editorSettings?: EditorSettings
    togglePopover: () => void
    onSave: (selectedEditorId: EditorId, defaultProjectPath: string) => Promise<void>
    sourcegraphUrl: string
}

/**
 * A popover that displays a searchable list of revisions (grouped by type) for
 * the current repository.
 */
export const OpenInEditorPopover: React.FunctionComponent<
    React.PropsWithChildren<OpenInEditorPopoverProps>
> = props => {
    const { editorSettings, togglePopover } = props

    const [selectedEditorId, setSelectedEditorId] = React.useState<EditorId>(editorSettings?.editorIds?.[0] || '')
    const [defaultProjectPath, setDefaultProjectPath] = React.useState<string>(
        editorSettings?.['projectPaths.default'] || ''
    )
    const areSettingsValid = selectedEditorId && isProjectPathValid(defaultProjectPath)
    const [areValidSettingsSaved, setValidSettingsSaved] = React.useState<boolean>(false)

    const handleEditorChange = useCallback<React.ChangeEventHandler<HTMLSelectElement>>(event => {
        setSelectedEditorId(event.target.value)
    }, [])

    const onSubmit = useCallback<React.FormEventHandler<HTMLFormElement>>(
        event => {
            event.preventDefault()

            props
                .onSave(selectedEditorId || '', defaultProjectPath || '')
                .then(() => {
                    setValidSettingsSaved(true)
                })
                .catch(() => {
                    // TODO: Handle this failure nicely
                }) // Fallback values are only for TS
        },
        [defaultProjectPath, props, selectedEditorId]
    )

    const onProjectPathChange = useCallback((event: React.ChangeEvent<HTMLInputElement>): void => {
        event.preventDefault()
        setDefaultProjectPath(event.target.value)
    }, [])

    return (
        <div className={styles.openInEditorPopover}>
            <Button className={styles.close} onClick={togglePopover}>
                <VisuallyHidden>Close</VisuallyHidden>
                <Icon svgPath={mdiClose} inline={false} aria-hidden={true} />
            </Button>
            {(!areValidSettingsSaved ? renderForm : renderDone)()}
        </div>
    )

    function renderForm(): React.ReactNode {
        return (
            <>
                <H3>Set your preferred editor</H3>
                <Text>
                    Open this and other files directly in your editor. Set your path and editor to get started. Update
                    any time in your user settings.
                </Text>

                <Form onSubmit={onSubmit} noValidate={true}>
                    <Input
                        id="OpenInEditorForm-projectPath"
                        type="text"
                        label="Default projects path"
                        name="projectPath"
                        placeholder="/Users/username/projects"
                        required={true}
                        autoCorrect="off"
                        autoCapitalize="off"
                        spellCheck={false}
                        readOnly={false}
                        value={defaultProjectPath}
                        onChange={onProjectPathChange}
                        className={classNames('mr-sm-2')}
                    />
                    <aside className="small text-muted">
                        The directory that contains your repository checkouts. For example, if this repository is
                        checked out to <Code>/Users/username/projects/cody</Code>, then set your default projects path
                        to <Code>/Users/username/projects</Code>.
                    </aside>
                    <Select
                        id="OpenInEditorForm-editor"
                        label="Editor"
                        message={
                            <>
                                Use a different editor?{' '}
                                <Link to="/help/integration/open_in_editor" target="_blank" rel="noreferrer noopener">
                                    Set up another editor
                                </Link>
                            </>
                        }
                        value={selectedEditorId}
                        onChange={handleEditorChange}
                        className={styles.editorSelect}
                    >
                        <option value="" />
                        {[...supportedEditors]
                            .sort((a, b) => a.name.localeCompare(b.name))
                            .filter(editor => editor.id !== 'custom')
                            .map(editor => (
                                <option key={editor.id} value={editor.id}>
                                    {editor.name}
                                </option>
                            ))}
                    </Select>
                    <Button variant="primary" type="submit" disabled={!areSettingsValid}>
                        Save
                    </Button>
                </Form>
            </>
        )
    }

    function renderDone(): React.ReactNode {
        return (
            <>
                <H3>You’re all set</H3>
                <Text>
                    You can modify or add additional editor paths in your{' '}
                    <Link to={props.sourcegraphUrl + '/user/settings'} target="_blank" rel="noreferrer noopener">
                        user settings
                    </Link>{' '}
                    at any time.
                </Text>
                <Button variant="primary" onClick={togglePopover}>
                    Close
                </Button>
            </>
        )
    }
}
