/**
 * Converts VS Code language ID to Sourcegraph-compatible language ID
 * if necessary (e.g. "typescriptreact" -> "typescript")
 */
export function toSourcegraphLanguage(vscodeLanguageID: string): string {
    if (vscodeLanugageIDReplacements[vscodeLanguageID]) {
        return vscodeLanugageIDReplacements[vscodeLanguageID]!
    }
    return vscodeLanguageID
}

const vscodeLanugageIDReplacements: Record<string, string | undefined> = {
    typescriptreact: 'typescript',
}
