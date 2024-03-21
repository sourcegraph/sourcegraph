import { BehaviorSubject, type Observable } from 'rxjs'

/**
 * Adds correct theme class to <body> element,
 * returns Observable with latest theme.
 */
export function adaptSourcegraphThemeToEditorTheme(): Observable<'theme-dark' | 'theme-light' | undefined> {
    const body = document.querySelector<HTMLBodyElement>('body')

    const themes = new BehaviorSubject<'theme-dark' | 'theme-light' | undefined>(undefined)

    function applyVSCodeThemeToSourcegraph(): void {
        const vscodeTheme = body?.dataset.vscodeThemeKind
        const sourcegraphThemeClass = vscodeTheme === 'vscode-light' ? 'theme-light' : 'theme-dark'
        if (sourcegraphThemeClass !== themes.value) {
            if (sourcegraphThemeClass === 'theme-light') {
                body?.classList.remove('theme-dark')
                body?.classList.add('theme-light', 'sourcegraph-extension')
                themes.next('theme-light')
            } else {
                body?.classList.remove('theme-light')
                body?.classList.add('theme-dark', 'sourcegraph-extension')
                themes.next('theme-dark')
            }
        }
    }

    applyVSCodeThemeToSourcegraph()

    const mutationObserver = new MutationObserver(() => {
        applyVSCodeThemeToSourcegraph()
    })

    mutationObserver.observe(body!, { childList: false, attributes: true })

    return themes.asObservable()
}
