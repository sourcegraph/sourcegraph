export function assertEnvironment(environment: typeof window['EXTENSION_ENV']): void {
    if (globalThis.EXTENSION_ENV !== environment) {
        throw new Error(
            'Detected transitive import of an entrypoint! ' +
                String(globalThis.EXTENSION_ENV) +
                ' attempted to import a file that is only intended to be imported by ' +
                String(environment) +
                '.'
        )
    }
}
