/**
 * Built-in VS Code API exposed to webviews to communicate with the "Core" extension.
 * We typically use this as a low-level building block for the APIs used in our webviews
 * (wrapped w/ Comlink).
 */
export interface VsCodeApi<State = any> {
    postMessage: (message: any) => void
    getState: () => State | undefined
    setState: (state: State) => void
}
