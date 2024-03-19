# Frontend contribution guidelines

To work on most frontend issues, it is necessary to run three applications locally:

1. The web app in the [client/web directory](https://github.com/sourcegraph/sourcegraph/tree/main/client/web).
2. Storybook ([client/storybook](https://github.com/sourcegraph/sourcegraph/tree/main/client/storybook))
3. The browser extension ([client/browser](https://github.com/sourcegraph/sourcegraph/tree/main/client/browser))

## Project Setup

1. Clone the repo: `git clone https://github.com/sourcegraph/sourcegraph/`.
2. Make sure your node environment is running version `16.x.x`.
3. Go through the local development section [here](https://docs.sourcegraph.com/dev/background-information/web/web_app#local-development).

## CI checks to run locally

1. Typescript checks.

    ```sh
    # Generate Typescript types
    pnpm generate

    # Verify Typescript build
    pnpm build-ts
    ```

2. Linters.

    ```sh
    pnpm lint:js:all
    pnpm lint:css:all
    pnpm lint:graphql
    pnpm format:check
    ```

3. Unit tests

    ```sh
    # Run unit tests
    pnpm test
    ```

4. Integration tests

    ```sh
    # Prepare web application for integration tests
    DISABLE_TYPECHECKING=true pnpm run build-web
    # Run integration tests
    pnpm test-integration
    ```

## Relevant development docs

### Getting applications up and running

- [Developing the Sourcegraph web app](https://docs.sourcegraph.com/dev/background-information/web/web_app#commands)
- [Table of contents related to the web app](https://docs.sourcegraph.com/dev/background-information/web)
- Configuring backend services locally is not required for most frontend issues. However, a guide on how to do this can be found [here](https://docs.sourcegraph.com/dev/getting-started).

### How to style UI

- [Guidelines](https://docs.sourcegraph.com/dev/background-information/web/styling)
- [Wildcard Component Library](https://docs.sourcegraph.com/dev/background-information/web/wildcard)

### Client packages [overview](https://github.com/sourcegraph/sourcegraph/blob/main/client/README.md)
