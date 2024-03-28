# Context

Ideally, this package entry point should be exposed via the `package.json` `exports` field,
so we can import it via `@sourcegraph/wildcard/testing`, but for that, we need to enable ES modules
support in the codebase. It means migrating all internal packages to ESM (`"type": "module"`) and using
`"moduleResolution": "nodeNext"` in `tsconfig.json`. This is required because common.js modules
cannot import ES modules using ES6 import statements. For example, there's no way to continue importing
`@apollo/client` as we do today because Typescript will complain:

> "The current file is a CommonJS module whose imports will produce 'require' calls; however, the referenced file is an ECMAScript module and cannot be imported with 'require'. Consider writing a dynamic 'import(\"{0}\")' call instead. To convert this file to an ECMAScript module, change its file extension to '.mts', or add the field `"type": "module"` to '/package.json'"
Hello World
