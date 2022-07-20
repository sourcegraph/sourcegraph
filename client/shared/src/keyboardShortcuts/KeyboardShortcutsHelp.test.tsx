import { ShortcutProvider } from '@slimsag/react-shortcuts'
import { fireEvent, waitFor, screen } from '@testing-library/react'

import { renderWithBrandedContext } from '../testing'
import { KEYBOARD_SHORTCUTS } from './keyboardShortcuts'

import { KeyboardShortcutsHelp } from './KeyboardShortcutsHelp'

describe('KeyboardShortcutsHelp', () => {
    test('is triggered correctly', async () => {
        renderWithBrandedContext(
            <ShortcutProvider>
                <KeyboardShortcutsHelp keyboardShortcuts={KEYBOARD_SHORTCUTS} isOpen={true} onDismiss={() => null} />
            </ShortcutProvider>
        )

        // couldn't trigger event with ctrl/alt/shift key so use shortcut without held keys
        fireEvent.keyDown(document, { key: 'x', keyCode: 88 })

        await waitFor(() => {
            expect(screen.getByText(/keyboard shortcuts/i)).toBeVisible()
        })

        expect(document.body).toMatchSnapshot()
    })
})
