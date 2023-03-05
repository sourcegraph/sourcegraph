import assert from 'assert'

import { readFile } from 'mz/fs'
import Simmer, { Options as SimmerOptions } from 'simmerjs'
import { SetIntersection } from 'utility-types'

import { DiffPart } from '@sourcegraph/codeintellify'

import { CodeHost, MountGetter } from './codeHost'
import { CodeView, DOMFunctions } from './codeViews'

const mountGetterKeys = ['getViewContextOnSourcegraphMount'] as const
type MountGetterKey = typeof mountGetterKeys[number]

/**
 * @param containerHtmlFixturePaths Paths to full-document fixtures keyed by the mount getter function name
 */
export function testCodeHostMountGetters<C extends CodeHost>(
    codeHost: C,
    containerHtmlFixturePaths: string | Record<SetIntersection<MountGetterKey, keyof C>, string>
): void {
    for (const mountGetterKey of mountGetterKeys) {
        const getMount = codeHost[mountGetterKey]
        if (!getMount) {
            continue
        }
        describe(mountGetterKey, () => {
            const fixturePath =
                typeof containerHtmlFixturePaths === 'string'
                    ? containerHtmlFixturePaths
                    : containerHtmlFixturePaths[mountGetterKey as keyof typeof containerHtmlFixturePaths]
            testMountGetter(fixturePath, getMount, true, true)
        })
    }
}

export function testToolbarMountGetter(
    codeViewHtmlFixturePath: string,
    getToolbarMount: NonNullable<CodeView['getToolbarMount']>
): void {
    testMountGetter(codeViewHtmlFixturePath, getToolbarMount, false, false)
}

export async function getFixtureBody({
    isFullDocument,
    htmlFixturePath,
}: {
    isFullDocument: boolean
    htmlFixturePath: string
}): Promise<HTMLElement> {
    const content = await readFile(htmlFixturePath, 'utf-8')
    // Do not append to global document to test that the mount getter only looks at the container
    if (isFullDocument) {
        // Create Document
        const fixtureDocument = document.implementation.createHTMLDocument()
        fixtureDocument.write(content)
        return fixtureDocument.documentElement
    }
    // Create DocumentFragment
    const template = document.createElement('template')
    template.innerHTML = content
    if (template.content.children.length !== 1) {
        throw new Error(
            `Fixture must have exactly one element, has ${template.content.children.length}: ${htmlFixturePath}`
        )
    }
    if (!(template.content.firstElementChild instanceof HTMLElement)) {
        throw new TypeError(`Fixture must be HTML: ${htmlFixturePath}`)
    }
    return template.content.firstElementChild
}

/**
 * @param isFullDocument Whether the fixture HTML is a document fragment or a full document (does it contain `<html>`?)
 * @param mayReturnNull Whether the mount getter might be called with containers where the mount does not belong.
 * It will be tested to return `null` in that case.
 */
export function testMountGetter(
    htmlFixturePath: string,
    getMount: MountGetter,
    isFullDocument: boolean,
    mayReturnNull: boolean
): void {
    it('creates and returns a new mount in the container', async () => {
        const container = await getFixtureBody({ isFullDocument, htmlFixturePath })
        const outerHtmlBefore = container.outerHTML
        const mount = getMount(container)
        expect(mount).toBeInstanceOf(HTMLElement)
        expect(container.contains(mount)).toBe(true)
        if (container.outerHTML === outerHtmlBefore) {
            // Don't use expect().not.toBe() because the output is gigantic
            assert.fail('Expected container outerHTML to have changed')
        }
    })
    it('is idempotent', async () => {
        const container = await getFixtureBody({ isFullDocument, htmlFixturePath })
        const first = getMount(container)
        const outerHTMLAfterFirstCall = container.outerHTML
        const second = getMount(container)
        expect(first).toBe(second)
        expect(container.outerHTML).toBe(outerHTMLAfterFirstCall)
    })
    if (mayReturnNull) {
        it('returns null if the mount does not belong into the container', () => {
            const container = document.createElement('div')
            container.innerHTML = '<div>Hello</div><div>World</div>'
            const mount = getMount(container)
            expect(mount).toBe(null)
        })
    } else {
        it('throws an Error if given an unexpected code view', () => {
            const container = document.createElement('div')
            container.innerHTML = '<div>Hello</div><div>World</div>'
            try {
                getMount(container)
                assert.fail('Expected function to throw an Error')
            } catch (error) {
                // "Cannot read property foo of null" does not count!
                expect(error).not.toBeInstanceOf(TypeError)
            }
        })
    }
}

interface Line {
    lineNumber: number
    /**
     * The part of the diff, if the code view is a diff code view.
     */
    diffPart?: DiffPart

    /**
     * Whether the first character of the line is a diff indicator
     */
    firstCharacterIsDiffIndicator?: boolean
}

export interface DOMFunctionsTest {
    htmlFixturePath: string

    /**
     * Descriptors for lines in the diff that will be tested
     */
    lineCases: Line[]

    url?: string // TODO DOM functions should not rely on global state like the URL
}

export function testDOMFunctions(
    domFunctions: DOMFunctions,
    { htmlFixturePath, lineCases: codeElements, url }: DOMFunctionsTest
): void {
    let codeViewElement: HTMLElement
    beforeEach(async () => {
        if (url) {
            jsdom.reconfigure({ url })
        }
        codeViewElement = await getFixtureBody({ htmlFixturePath, isFullDocument: false })
    })
    for (const { diffPart, lineNumber, firstCharacterIsDiffIndicator } of codeElements) {
        describe(
            `line number ${lineNumber}` + (diffPart !== undefined ? ` in ${String(diffPart)} diff part` : ''),
            () => {
                const simmerOptions: SimmerOptions = {
                    depth: 20,
                    specificityThreshold: 500,
                    selectorMaxLength: 1000,
                }

                describe('getLineElementFromLineNumber()', () => {
                    it('should return the right line element given the line number', () => {
                        const codeElement = domFunctions.getLineElementFromLineNumber(
                            codeViewElement,
                            lineNumber,
                            diffPart
                        )
                        expect(codeElement).toBeDefined()
                        expect(codeElement).not.toBeNull()
                        // Generate CSS selector for element
                        const simmer = new Simmer(codeViewElement, simmerOptions)
                        const selector = simmer(codeElement!)
                        expect(selector).toBeTruthy()
                        expect({ selector, content: codeElement!.textContent!.trim() }).toMatchSnapshot()
                    })
                })

                describe('getCodeElementFromLineNumber()', () => {
                    it('should return the right code element given the line number', () => {
                        const codeElement = domFunctions.getCodeElementFromLineNumber(
                            codeViewElement,
                            lineNumber,
                            diffPart
                        )
                        expect(codeElement).toBeDefined()
                        expect(codeElement).not.toBeNull()
                        // Generate CSS selector for element
                        const simmer = new Simmer(codeViewElement, simmerOptions)
                        const selector = simmer(codeElement!)
                        expect(selector).toBeTruthy()
                        expect({ selector, content: codeElement!.textContent!.trim() }).toMatchSnapshot()
                    })
                })

                let codeElement: HTMLElement
                const setCodeElement = (): void => {
                    codeElement = domFunctions.getCodeElementFromLineNumber(codeViewElement, lineNumber, diffPart)!
                    if (!codeElement) {
                        throw new Error('Test depends on test for getCodeElementFromLineNumber() passing')
                    }
                }
                // These tests depend on getCodeElementFromLineNumber() working as expected
                describe('getLineNumberFromCodeElement()', () => {
                    beforeEach(setCodeElement)
                    it('should return the right line number given the code element', () => {
                        const returnedLineNumber = domFunctions.getLineNumberFromCodeElement(codeElement)
                        expect(returnedLineNumber).toBe(lineNumber)
                    })
                })
                if (domFunctions.getDiffCodePart) {
                    describe('getDiffCodePart()', () => {
                        beforeEach(setCodeElement)
                        it(`should return "${String(diffPart)}" when given the code element`, () => {
                            expect(domFunctions.getDiffCodePart!(codeElement)).toBe(diffPart)
                        })
                    })
                }
                describe('isFirstCharacterDiffIndicator()', () => {
                    beforeEach(setCodeElement)
                    it('should return correctly whether the first character is a diff indicator', () => {
                        // Default is false
                        const actualFirstCharacterIsDiffIndicator = Boolean(
                            domFunctions.isFirstCharacterDiffIndicator?.(codeElement)
                        )
                        expect(actualFirstCharacterIsDiffIndicator).toBe(Boolean(firstCharacterIsDiffIndicator))
                        if (actualFirstCharacterIsDiffIndicator) {
                            // Check that the first character is truly a diff indicator
                            const diffIndicators = new Set(['+', '-', ' '])
                            expect(actualFirstCharacterIsDiffIndicator).toBe(
                                diffIndicators.has(codeElement.textContent![0])
                            )
                        }
                    })
                })
            }
        )
    }
}
