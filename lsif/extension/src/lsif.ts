import * as sourcegraph from 'sourcegraph'
import * as LSP from 'vscode-languageserver-types'

function repositoryFromDoc(doc: sourcegraph.TextDocument): string {
    const url = new URL(doc.uri)
    return url.hostname + url.pathname
}

function commitFromDoc(doc: sourcegraph.TextDocument): string {
    const url = new URL(doc.uri)
    return url.search.slice(1)
}

function pathFromDoc(doc: sourcegraph.TextDocument): string {
    const url = new URL(doc.uri)
    return url.hash.slice(1)
}

function setPath(doc: sourcegraph.TextDocument, path: string): string {
    const url = new URL(doc.uri)
    url.hash = path
    return url.href
}

async function send({
    doc,
    method,
    params,
}: {
    doc: sourcegraph.TextDocument
    method: string
    params: any[]
}): Promise<any> {
    const urlParams = new URLSearchParams()
    urlParams.set('repository', repositoryFromDoc(doc))
    urlParams.set('commit', commitFromDoc(doc))

    const response = await fetch(
        path.join(sourcegraph.internal.sourcegraphURL + `.api/lsif/request?${urlParams.toString()}`),
        {
            method: 'POST',
            headers: new Headers({
                'content-type': 'application/json',
                'x-requested-with': 'Sourcegraph LSIF extension',
            }),
            body: JSON.stringify({
                method,
                params,
            }),
        }
    )
    const body = await response.json()
    if (body.error) {
        if (body.error === 'No result found') {
            return null
        }
        throw new Error(body.error)
    }
    return body
}

const lsifDocs = new Map<string, Promise<boolean>>()

async function hasLSIF(doc: sourcegraph.TextDocument): Promise<boolean> {
    if (lsifDocs.has(doc.uri)) {
        return lsifDocs.get(doc.uri)!
    }

    const urlParams = new URLSearchParams()
    urlParams.set('repository', repositoryFromDoc(doc))
    urlParams.set('commit', commitFromDoc(doc))
    urlParams.set('file', pathFromDoc(doc))

    const hasLSIFPromise = (async () => {
        const response = await fetch(
            path.join(sourcegraph.internal.sourcegraphURL + `.api/lsif/exists?${urlParams.toString()}`),
            {
                method: 'POST',
                headers: new Headers({ 'x-requested-with': 'Sourcegraph LSIF extension' }),
            }
        )
        const body = await response.json()
        if (body.error) {
            throw new Error(body.error)
        }
        if (typeof body !== 'boolean') {
            throw new Error(body)
        }
        return body
    })()

    lsifDocs.set(doc.uri, hasLSIFPromise)

    return hasLSIFPromise
}

export function activate(ctx: sourcegraph.ExtensionContext): void {
    ctx.subscriptions.add(
        sourcegraph.languages.registerHoverProvider(['*'], {
            provideHover: async (doc, pos) => {
                if (!(await hasLSIF(doc))) {
                    return null
                }
                const body = await send({ doc, method: 'hover', params: [pathFromDoc(doc), pos] })
                if (!body) {
                    return null
                }
                return {
                    ...body,
                    contents: {
                        value: body.contents
                            .map((content: { language: string; value: string } | string) =>
                                typeof content === 'string'
                                    ? content
                                    : content.language
                                    ? ['```' + content.language, content.value, '```'].join('\n')
                                    : content.value
                            )
                            .join('\n'),
                        kind: sourcegraph.MarkupKind.Markdown,
                    },
                }
            },
        })
    )

    ctx.subscriptions.add(
        sourcegraph.languages.registerDefinitionProvider(['*'], {
            provideDefinition: async (doc, pos) => {
                if (!(await hasLSIF(doc))) {
                    return null
                }
                const body = await send({ doc, method: 'definitions', params: [pathFromDoc(doc), pos] })
                if (!body) {
                    return null
                }
                return body.map((definition: LSP.Location) => ({
                    ...definition,
                    uri: setPath(doc, definition.uri),
                }))
            },
        })
    )

    ctx.subscriptions.add(
        sourcegraph.languages.registerReferenceProvider(['*'], {
            provideReferences: async (doc, pos) => {
                if (!(await hasLSIF(doc))) {
                    return null
                }
                const body = await send({ doc, method: 'references', params: [pathFromDoc(doc), pos] })
                if (!body) {
                    return null
                }
                return body.map((reference: LSP.Location) => ({
                    ...reference,
                    uri: setPath(doc, reference.uri),
                }))
            },
        })
    )
}
