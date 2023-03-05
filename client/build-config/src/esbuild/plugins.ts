import * as esbuild from 'esbuild'
import signale from 'signale'

export * from './monacoPlugin'
export * from './packageResolutionPlugin'
export * from './stylePlugin'

export const buildTimerPlugin: esbuild.Plugin = {
    name: 'buildTimer',
    setup: (build: esbuild.PluginBuild): void => {
        let buildStarted: number
        build.onStart(() => {
            buildStarted = Date.now()
        })
        build.onEnd(() => console.log(`# esbuild: build took ${Date.now() - buildStarted}ms`))
    },
}

export const experimentalNoticePlugin: esbuild.Plugin = {
    name: 'experimentalNotice',
    setup: (): void => {
        signale.info(
            'esbuild usage is experimental. See https://docs.sourcegraph.com/dev/background-information/web/build#esbuild.'
        )
    },
}
