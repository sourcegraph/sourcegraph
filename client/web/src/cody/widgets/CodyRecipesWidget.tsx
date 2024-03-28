import React, { useEffect } from 'react'

import { mdiCardBulletedOutline, mdiDotsVertical, mdiProgressPencil, mdiShuffleVariant } from '@mdi/js'

import { TranslateToLanguage } from '@sourcegraph/cody-shared'
import type { TelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import { eventLogger } from '../../tracking/eventLogger'
import { EventName } from '../../util/constants'
import type { CodeMirrorEditor } from '../components/CodeMirrorEditor'
import type { useCodySidebar } from '../sidebar/Provider'

import { Recipe } from './components/Recipe'
import { RecipeAction } from './components/RecipeAction'
import { Recipes } from './components/Recipes'

export const CodyRecipesWidget: React.FC<{ editor?: CodeMirrorEditor; telemetryRecorder: TelemetryRecorder }> = ({
    editor,
    telemetryRecorder,
}) => {
    useEffect(() => {
        eventLogger.log(EventName.CODY_CHAT_EDITOR_WIDGET_VIEWED)
        telemetryRecorder.recordEvent('cody.chat.editor-widget', 'view')
    }, [telemetryRecorder])

    // dirty fix becasue it is rendered under a separate React DOM tree.
    const codySidebarStore = (window as any).codySidebarStore as ReturnType<typeof useCodySidebar>
    if (!codySidebarStore) {
        return null
    }

    const { executeRecipe, isMessageInProgress, loaded } = codySidebarStore

    if (!loaded) {
        return null
    }

    return (
        <Recipes>
            <Recipe title="Explain" icon={mdiCardBulletedOutline}>
                <RecipeAction
                    title="Detailed"
                    onClick={() => void executeRecipe('explain-code-detailed', { scope: { editor } })}
                    disabled={isMessageInProgress}
                />
                <RecipeAction
                    title="High level"
                    onClick={() => void executeRecipe('explain-code-high-level', { scope: { editor } })}
                    disabled={isMessageInProgress}
                />
            </Recipe>

            <Recipe title="Generate" icon={mdiProgressPencil}>
                <RecipeAction
                    title="A unit test"
                    onClick={() => void executeRecipe('generate-unit-test', { scope: { editor } })}
                    disabled={isMessageInProgress}
                />
                <RecipeAction
                    title="A docstring"
                    onClick={() => void executeRecipe('generate-docstring', { scope: { editor } })}
                    disabled={isMessageInProgress}
                />
            </Recipe>

            <Recipe title="Transpile" icon={mdiShuffleVariant}>
                {TranslateToLanguage.options.map(language => (
                    <RecipeAction
                        key={language}
                        title={language}
                        disabled={isMessageInProgress}
                        onClick={() =>
                            void executeRecipe('translate-to-language', {
                                prefilledOptions: [[TranslateToLanguage.options, language]],
                                scope: { editor },
                            })
                        }
                    />
                ))}
            </Recipe>

            <Recipe icon={mdiDotsVertical}>
                <RecipeAction
                    title="Improve variable names"
                    disabled={isMessageInProgress}
                    onClick={() => void executeRecipe('improve-variable-names', { scope: { editor } })}
                />
                <RecipeAction
                    title="Smell code"
                    onClick={() => void executeRecipe('find-code-smells', { scope: { editor } })}
                    disabled={isMessageInProgress}
                />
                <RecipeAction title="Get Cody in your editor" to="/cody/manage" disabled={isMessageInProgress} />
            </Recipe>
        </Recipes>
    )
}
