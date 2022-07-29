import { MarkupKind } from '@sourcegraph/extension-api-classes'

import { LinkPreviewMerged } from '../../api/extension/extensionHostApi'

import { applyLinkPreview, ApplyLinkPreviewOptions } from './linkPreviews'

const OPTIONS: ApplyLinkPreviewOptions = {
    setElementTooltip: (element, text) =>
        text !== null ? element.setAttribute('aria-label', text) : element.removeAttribute('aria-label'),
}

describe('applyLinkPreview', () => {
    test('annotates element and is idempotent', () => {
        const div = document.createElement('div')
        const link = document.createElement('a')
        link.href = 'u'
        link.textContent = 'b'
        div.append(link)

        const LINK_PREVIEW_MERGED: LinkPreviewMerged = {
            content: [
                {
                    kind: MarkupKind.Markdown,
                    value: '**x**',
                },
            ],
            hover: [
                {
                    kind: MarkupKind.PlainText,
                    value: 'y',
                },
            ],
        }
        applyLinkPreview(OPTIONS, link, LINK_PREVIEW_MERGED)
        const WANT =
            '<a href="u" aria-label="y">b</a><span class="sg-link-preview-content" aria-label="y"><strong>x</strong></span>'
        expect(div.innerHTML).toBe(WANT)

        // Check for idempotence.
        applyLinkPreview(OPTIONS, link, LINK_PREVIEW_MERGED)
        expect(div.innerHTML).toBe(WANT)
    })
})
