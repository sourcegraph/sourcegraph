# Shared

This folder contains common TypeScript/React/SCSS client code shared between the browser extension and the web app.

Everything in this folder is code-host agnostic and cannot make assumptions about whether it is running inside the Sourcegraph web app, in the browser extension on GitHub, Gitlab, Phabricator, Bitbucket Server, etc.
In particular, components cannot make use of global CSS classes but must accept CSS classes as props and/or have their own code host agnostic SCSS stylesheets.
For more details, see [Styling UI in the handbook](../../doc/dev/background-information/web/styling.md).

Code that is only used in branded contexts (web app, options menu of the browser extension, ...) should go into [`../branded`](../branded) instead.
Hello World
