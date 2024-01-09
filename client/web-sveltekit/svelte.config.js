import { join } from 'path'

import staticAdapter from '@sveltejs/adapter-static'
import { vitePreprocess } from '@sveltejs/kit/vite'

let adapter

if (process.env.BAZEL || process.env.DEPLOY_TYPE === 'dev') {
  // The folder to write the production files to.
  // We store the files in a separate folder to avoid any conflicts
  // with files generated by the web builder.
  const OUTPUT_DIR = '_sk'

  let out = 'build/'
  if (process.env.DEPLOY_TYPE === 'dev' && !process.env.BAZEL) {
    // When DEPLOY_TYPE is set to 'dev' we copy output files to the
    // 'assets' folder where the web server reads them from
    out = '../../client/web/dist/'
  }

  out += OUTPUT_DIR

  adapter = sgAdapter({
    out,
    // Path from which the web server will serve the SvelteKit files
    assetPath: `.assets/${OUTPUT_DIR}`,
    fallback: 'index.html',
  })
} else {
  // Default setup
  adapter = staticAdapter({ fallback: 'index.html' })
}

/** @type {import('@sveltejs/kit').Config} */
const config = {
  // Consult https://github.com/sveltejs/svelte-preprocess
  // for more information about preprocessors
  //preprocess: preprocess(),
  // Consult https://kit.svelte.dev/docs/integrations#preprocessors
  // for more information about preprocessors
  preprocess: vitePreprocess(),

  vitePlugin: {
    inspector: {
      showToggleButton: 'always',
      toggleButtonPos: 'bottom-right',
    },
  },

  kit: {
    adapter,
    alias: {
      // Makes it easier to refer to files outside packages (such as images)
      $root: '../../',
      // Used inside tests for easy access to helpers
      $testdata: 'src/testdata.ts',
      $mocks: 'src/testing/mocks.ts',
      $testing: 'src/testing',
      // Map node-module to browser version
      path: '../../node_modules/path-browserify',
      // These are directories and cannot be imported from directly in
      // production build. Need to import from _esm5, otherwise there will
      // be runtime compatibility issues.
      'rxjs/operators': '../../node_modules/rxjs/_esm5/operators/index',
      'rxjs/fetch': '../../node_modules/rxjs/_esm5/fetch/index',
      // Without it prod build doesnt work
      '@apollo/client$': '../../node_modules/@apollo/client/index.js',
      lodash: './node_modules/lodash-es',
    },
    typescript: {
      config: config => {
        config.extends = '../../../tsconfig.base.json'
        config.include = [...(config.include ?? []), '../src/**/*.tsx', '../.storybook/*.ts']
      },
    },
    paths: {
      relative: true,
    },
  },
}

export default config

/**
 * This adapter is a simplified version of @sveltejs/adapter-static.
 * In addition to copying the generate client code this adapater also
 * updates the asset paths in the fallback/index page to work properly
 * with the Sourcegraph backend.
 *
 * Longer explanation:
 *
 * In a single page app, the fallback page is returned by the server
 * for every path (i.e. both "/page" and "/some/nested/page" return
 * the same HTML page). Because of this every other resource included
 * by this page must be referenced via root-relative paths.
 * By default SvelteKit assumes that all files are deployed into the
 * web root directory.
 *
 * The SvelteKit artifacts however are deployed into a subdirectory,
 * to not conflict with any artifacts, which means they are
 * served from a different location than SvelteKit assumes.
 * In theory we could use the 'paths.assets' option to configure
 * this path, but at the moment it only accepts a fully-qualified
 * URL as value, and we don't know the final URL at build time.
 */
function sgAdapter(options) {
  return {
    name: 'sg adapter',
    async adapt(builder) {
      const out = options.out || 'build'
      const appDir = builder.config.kit.appDir
      const tmp = builder.getBuildDirectory('sg-adapter')
      const fallback = join(tmp, options.fallback)

      builder.rimraf(tmp)
      builder.rimraf(out)

      builder.writeClient(out)
      builder.writePrerendered(out)
      await builder.generateFallback(fallback)

      builder.copy(fallback, join(out, options.fallback), {
        replace: {
          [`${appDir.replace(/\./g, '\\.')}`]: `${options.assetPath}/${appDir}`,
        },
      })

      builder.log(`Wrote site to "${out}"`)
    },
  }
}
