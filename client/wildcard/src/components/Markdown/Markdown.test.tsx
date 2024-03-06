import { describe, expect, it } from 'vitest'

import { renderWithBrandedContext } from '../../testing'

import { Markdown } from './Markdown'
import { mathjaxElementId } from './mathjax'

describe('Markdown', () => {
    it('renders', () => {
        const component = renderWithBrandedContext(<Markdown dangerousInnerHTML="hello" />)
        expect(component.asFragment()).toMatchSnapshot()
    })

    it('useMathJax', () => {
        const { unmount } = renderWithBrandedContext(
            <Markdown dangerousInnerHTML="$math mentioned$" enableMathJax={true} />
        )
        expect(getMathJax(), 'inject MathJax <script> to <head> on mount').not.toBeNull()

        unmount()

        expect(getMathJax(), 'remove MathJax <script> on unmount').toBeNull()
    })
})

const getMathJax = (): HTMLScriptElement | null => {
    return document.getElementById(mathjaxElementId) as HTMLScriptElement
}
