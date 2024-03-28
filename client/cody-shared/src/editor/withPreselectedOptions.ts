import type { Editor } from '.'

export type PrefilledOptions = [string[], string][]

export function withPreselectedOptions(editor: Editor, preselectedOptions: PrefilledOptions): Editor {
    const proxy = new Proxy<Editor>(editor, {
        get(target: Editor, property: string, receiver: unknown) {
            if (property === 'showQuickPick') {
                return async function showQuickPick(options: string[]): Promise<string | undefined> {
                    for (const [preselectedOption, selectedOption] of preselectedOptions) {
                        if (preselectedOption === options) {
                            return Promise.resolve(selectedOption)
                        }
                    }
                    return target.showQuickPick(options)
                }
            }
            return Reflect.get(target, property, receiver)
        },
    })

    return proxy
}
