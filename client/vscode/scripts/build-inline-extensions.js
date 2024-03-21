const path = require('path')

const { fetchAndBuildCodeIntelExtensions } = require('../../shared/dev/buildCodeIntelExtensions')

const pathToExtensionBundles = path.join(process.cwd(), 'dist', 'extensions')

// keep revision up-to-date with "sourcegraph_extensions_bundle" in WORKSPACE
fetchAndBuildCodeIntelExtensions({ pathToExtensionBundles, revision: 'v5.0.1' })
