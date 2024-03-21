// This file is used for esbuild's `inject` option
// in order to load node polyfills in the webworker
// extension host.
// See: https://esbuild.github.io/api/#inject.
export const Buffer = require('buffer/').Buffer
