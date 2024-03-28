# Frontend packages

## List

- **web**: The web application deployed to http://sourcegraph.com/
- **browser**: The Sourcegraph browser extension adds tooltips to code on different code hosts.
- **vscode**: The Sourcegraph [VS Code extension](https://marketplace.visualstudio.com/items?itemName=sourcegraph.sourcegraph).
- **extension-api**: The Sourcegraph extension API types for the _Sourcegraph extensions_. Published as `sourcegraph`.
- **extension-api-types**: The Sourcegraph extension API types for _client applications_ that embed Sourcegraph extensions and need to communicate with them. Published as `@sourcegraph/extension-api-types`.
- **sandboxes**: All demos-mvp (minimum viable product) for the Sourcegraph web application.
- **shared**: Contains common TypeScript/React/SCSS client code shared between the browser extension and the web app. Everything in this package is code-host agnostic.
- **branded**: Contains React components and implements the visual design language we use across our web app and e.g. in the options menu of the browser extension. Over time, components from `shared` and `branded` packages should be moved into the `wildcard` package.
- **wildcard**: Package that encapsulates storybook configuration and contains our Wildcard design system components. If we're using a component in two or more different areas (e.g. `web-app` and `browser-extension`) then it should live in the `wildcard` package. Otherwise the components should be better colocated with the code where they're actually used.
- **search**: Search-related code that may be shared between all clients, both branded (e.g. web, VS Code extension) and unbranded (e.g. browser extension)
- **storybook**: Storybook configuration.

## Further migration plan

1. Fix circular dependency in TS project-references graph **wildcard** package should not rely on **web** and probably **shared**, **branded** too. Ideally it should be an independent self-contained package.

2. Decide on package naming and update existing package names. Especially it should be done for a **shared** package because we have multiple `shared` folders inside of other packages. It's hard to understand from where dependency is coming from and it's not possible to refactor import paths using find-and-replace.

3. Investigate if we can painlessly switch to `npm` workspaces.

4. Content of packages **shared** and **branded** should be moved to **wildcard** and refactored using the latest FE rules and conventions. Having different packages clearly communicates the migration plan. Developers first should look for components in the **wildcard** package and then fall-back to **legacy** packages if **wildcard** doesn't have the solution to their problem yet.

5. **shared** contains utility functions, types, polyfills, etc which is not a part of the Wildcard component library. These modules should be moved into **utils** package and other new packages: e.g. **api** for GraphQL client and type generators, etc.

6. Packages should use package name (e.g. `@sourcegraph/wildcard`) for imports instead of the relative paths (e.g. `../../../../wildcard/src/components/Markdown`) to avoid long relative-paths and make dependency graph between packages clear. (Typescript will warn if packages have circular dependencies). It's easy to refactor such isolated packages, extract functionality into new ones, or even into new repositories.
Hello World
