# Storybook configuration

Check out the [Storybook section](https://sourcegraph.com/docs/dev/background-information/web/web_app#storybook) in the [Developing the Sourcegraph web app](https://docs.sourcegraph.com/dev/background-information/web/web_app) docs.

## Usage

Storybook configuration is set up as a `pnpm workspace` symlink.

Important commands are exposed via root `package.json`:

```sh
pnpm storybook
pnpm storybook:build
```

## Environment variables

```sh
# Load only a subset of stories to boost build performance.
STORIES_GLOB='client/web/src/**/*.story.tsx' pnpm start
```
Hello World
