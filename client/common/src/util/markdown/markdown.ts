import DOMPurify, { type Config as DOMPurifyConfig } from 'dompurify'
import { highlight, highlightAuto } from 'highlight.js/lib/core'
// This is the only file allowed to import this module, all other modules must use renderMarkdown() exported from here
// eslint-disable-next-line no-restricted-imports
import { marked } from 'marked'

import { logger } from '../logger'

/**
 * Escapes HTML by replacing characters like `<` with their HTML escape sequences like `&lt;`
 */
const escapeHTML = (html: string): string => {
    const span = document.createElement('span')
    span.textContent = html
    return span.innerHTML
}

/**
 * Attempts to syntax-highlight the given code.
 * If the language is not given, it is auto-detected.
 * If an error occurs, the code is returned as plain text with escaped HTML entities
 * @param code The code to highlight
 * @param language The language of the code, if known
 * @returns Safe HTML
 */
export const highlightCodeSafe = (code: string, language?: string): string => {
    try {
        if (language === 'plaintext' || language === 'text') {
            return escapeHTML(code)
        }
        if (language === 'sourcegraph') {
            return code
        }
        if (language) {
            return highlight(code, { language, ignoreIllegals: true }).value
        }
        return highlightAuto(code).value
    } catch (error) {
        logger.warn('Error syntax-highlighting hover markdown code block', error)
        return escapeHTML(code)
    }
}

export interface RenderMarkdownOptions {
    /**
     * Whether to render markdown inline, without paragraph tags
     */
    inline?: boolean
    /**
     * Whether to render line breaks as HTML `<br>`s
     */
    breaks?: boolean
    /**
     * Whether to disable autolinks. Explicit links using `[text](url)` are still allowed.
     */
    disableAutolinks?: boolean
    /**
     * A custom renderer to use
     */
    renderer?: marked.Renderer
    /**
     * A prefix to add to all header IDs
     */
    headerPrefix?: string
    /**
     * Strip off any HTML and return a plain text string, useful for previews
     */
    plainText?: boolean
    /**
     * DOMPurify configuration to use
     */
    dompurifyConfig?: DOMPurifyConfig & { RETURN_DOM_FRAGMENT?: false; RETURN_DOM?: false }
    /**
     * Add target="_blank" and rel="noopener" to all <a> links that have a href value.
     * This affects all markdown-formatted links and all inline HTML links.
     */
    addTargetBlankToAllLinks?: boolean
}

/**
 * Renders the given markdown to HTML, highlighting code and sanitizing dangerous HTML.
 * Can throw an exception on parse errors.
 * @param markdown The markdown to render
 * @param options Options to customize rendering
 * @param options.breaks Whether to render line breaks as HTML `<br>`s
 * @param options.disableAutolinks Whether to disable autolinks. Explicit links using `[text](url)` are still allowed.
 * @param options.renderer A custom renderer to use
 * @param options.headerPrefix A prefix to add to all header IDs
 * @param options.plainText Strip off any HTML and return a plain text string, useful for previews
 * @param options.dompurifyConfig DOMPurify configuration to use
 * @param options.addTargetBlankToAllLinks Add target="_blank" and rel="noopener" to all <a> links
 * that have a href value. This affects all markdown-formatted links and all inline HTML links.
 */
export const renderMarkdown = (markdown: string, options: RenderMarkdownOptions = {}): string => {
    const tokenizer = new marked.Tokenizer()
    if (options.disableAutolinks) {
        // Why the odd double-casting below?
        // Because returning undefined is the recommended way to easily disable autolinks
        // but the type definition does not allow it.
        // More context here: https://github.com/markedjs/marked/issues/882
        tokenizer.url = () => undefined as unknown as marked.Tokens.Link
    }

    const rendered = (options.inline ? marked.parseInline : marked)(markdown, {
        gfm: true,
        breaks: options.breaks,
        highlight: (code, language) => highlightCodeSafe(code, language),
        renderer: options.renderer,
        headerPrefix: options.headerPrefix ?? '',
        tokenizer,
    })

    const dompurifyConfig: DOMPurifyConfig & { RETURN_DOM_FRAGMENT?: false; RETURN_DOM?: false } =
        typeof options.dompurifyConfig === 'object'
            ? options.dompurifyConfig
            : options.plainText
            ? {
                  ALLOWED_TAGS: [],
                  ALLOWED_ATTR: [],
                  KEEP_CONTENT: true,
              }
            : {
                  USE_PROFILES: { html: true },
                  FORBID_TAGS: ['style', 'form', 'input', 'button'],
                  FORBID_ATTR: ['rel', 'style', 'method', 'action'],
              }

    if (options.addTargetBlankToAllLinks) {
        // Add a hook that adds target="_blank" and rel="noopener" to all links. DOMPurify does not
        // support setting hooks per individual call to sanitize() so we have to
        // temporarily add the hook on the global module. This hook is removed
        // after the call to sanitize().
        DOMPurify.addHook('afterSanitizeAttributes', node => {
            if (node.tagName.toLowerCase() === 'a' && node.getAttribute('href')) {
                node.setAttribute('target', '_blank')
                node.setAttribute('rel', 'noopener')
            }
        })
    }

    const result = DOMPurify.sanitize(rendered, dompurifyConfig).trim()

    if (options.addTargetBlankToAllLinks) {
        // Because DOMPurify doesn't have a way to set hooks per individual call
        // to sanitize(), we have to clean up by removing the hook that we added
        // for addTargetBlankToAllLinks.
        DOMPurify.removeHook('afterSanitizeAttributes')
    }

    return result
}

export const markdownLexer = (markdown: string): marked.TokensList => marked.lexer(markdown)

/**
 * Escapes markdown by escaping all ASCII punctuation.
 *
 * Note: this does not escape whitespace, so when rendered markdown will
 * likely collapse adjacent whitespace.
 */
export const escapeMarkdown = (text: string): string => {
    /*
     * GFM you can escape any ASCII punctuation [1]. So we do that, with two
     * special notes:
     * - we escape "\" first to prevent double escaping it
     * - we replace < and > with HTML escape codes to prevent needing to do
     *   HTML escaping.
     * [1]: https://github.github.com/gfm/#backslash-escapes
     */
    const punctuation = '\\!"#%&\'()*+,-./:;=?@[]^_`{|}~'
    for (const char of punctuation) {
        text = text.replaceAll(char, '\\' + char)
    }
    return text.replaceAll('<', '&lt;').replaceAll('>', '&gt;')
}
