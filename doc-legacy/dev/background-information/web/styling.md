# Styling UI

Sourcegraph has many UI components. A unique constraint for these is that they need to run within different _environments_.
Wherever a component is running, it should look native to its environment and consistent with the design of that environment.
For example, our hover overlay needs to work and _behave_ the same in the Sourcegraph webapp and in the browser extension, where it is injected into a variety of code hosts, but _look_ native to the environment.
Components need to be able to adapt styles from CSS stylesheets already loaded on the page (no matter how those were architected).

## Goals

1. Components decoupled from styling, that look **consistent** with the host environment.
2. Support light and dark themes.
3. Tooling support:
   - Autocompletion for styles when writing components
   - Autocompletion when writing styles
   - Linting for styles
   - Browser dev tools to easily inspect and iterate on styles
   - Autoprefixer
4. Support for advanced CSS features, like state selectors, pseudo elements, flexbox, grid, media queries, CSS variables, ...

## Environments

### Host-agnostic UI

Components that need to run in different environments (any UI shared between our browser extension and the webapp) adopt styles from their environments through configurable CSS class names (as opposed to trying to replicate the styling with copied CSS).
A component may accept multiple class names for different elements of the component.
An example of this is `<HoverOverlay/>`: see how [the different props it accepts](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@4047521a92904054e782d341001d08d61945c86f/-/blob/shared/src/hover/HoverOverlay.tsx#L27-39) for its child components' class names (such as buttons) are passed in the [webapp](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@4047521a92904054e782d341001d08d61945c86f/-/blob/web/src/components/shared.tsx#L35-44) and in [code host integrations](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@4047521a92904054e782d341001d08d61945c86f/-/blob/browser/src/shared/code-hosts/shared/codeHost.tsx#L443:1).
They are defined for each code host referencing CSS class names **that the code host defines in its own styles**:

- for [GitHub](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b5af21f76dbb96ceece9f0908f56b3a7145ec4f7/-/blob/client/browser/src/shared/code-hosts/github/codeHost.ts#L353-361) using the class names from GitHub's [Primer design system](https://primer.style/react/Buttons)
- for [GitLab](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b5af21f76dbb96ceece9f0908f56b3a7145ec4f7/-/blob/client/browser/src/shared/code-hosts/gitlab/codeHost.ts#L203-211) using class names from Gitlab's [Pajamas design system](https://design.gitlab.com/components/button)
- for [Bitbucket](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b5af21f76dbb96ceece9f0908f56b3a7145ec4f7/-/blob/client/browser/src/shared/code-hosts/bitbucket/codeHost.tsx#L235-242) using the class names from [Atlassian UI](https://aui.atlassian.com/aui/7.10/docs/buttons.html)
- for [Phabricator](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b5af21f76dbb96ceece9f0908f56b3a7145ec4f7/-/blob/client/browser/src/shared/code-hosts/phabricator/codeHost.ts#L195-202) using class names found in the Phabricator UI with the browser dev tools (it doesn't have a documented design system, but is easy to reverse-engineer).

This means when one of the code hosts tweaks its design, or supports multiple themes, the UI elements contributed by our code host integrations automatically adapt with no effort on our part.

CSS classes as an approach represent the _lowest common denominator_ for styling between all environments, by staying close to the native web platform.

You may also notice that multiple of the above code hosts define the same or very similar classes to [our global CSS classes](#host-specific-ui), which makes it easy to map classes between our web app and code host environments.

### Host-specific UI

In the environments we control ourselves (such as our webapp, the options page of the browser extension, or our marketing website), we use components from [the Wildcard design system](https://storybook.sgdev.org/) and a set of global CSS classes extracted from [Bootstrap](https://getbootstrap.com/).
Any code inside our webapp can and should make use [Wildcard components](./wildcard.md) and global CSS classes defined in the `branded` package as building blocks (and should generally do so instead of writing custom styles).
This includes classes like [input groups](https://getbootstrap.com/docs/4.5/components/input-group/) and utility classes for [layout](https://getbootstrap.com/docs/4.5/utilities/flex/) and [spacing](https://getbootstrap.com/docs/4.5/utilities/spacing/). Check out available global CSS classes in the `branded` package.

Components only used in a specific host environment do not need to support customization through class names.
They can however utilize environment-agnostic components by passing our global CSS classes as custom `className` values.

## Our approach to styling

### General guidelines

- Colocate styles with the corresponding component. Stylesheet file should be named like the `.tsx` component file.
- Prefer classes over descendant/child selectors. It decouples styles from the DOM structure of the component, ensures encapsulation and avoids CSS specificity issues.
- Create utility classes for styles that should be shared horizontally between components.
- Avoid hardcoding colors, use CSS variables if they are available / the color makes sense to share.
- If possible, prefer CSS variables to SCSS variables.
- Try to _minimize_ the usage of advanced SCSS features. They can lead to bugs and complicate styles.
  - Encouraged features are nesting and imports (which is the intersection of Less', SCSS' and PostCSS' feature set).
- Think about mobile at least so much that no feature breaks when the browser window is resized.
- Prefer flexbox over absolute positioning.
- Avoid styling the children of your components. This couples your component to the implementation of the child.
- Order your rules so that layout rules (that describe how the component is laid out to its parents) come first, then rules that describe the layout of its children, and finally visual details.

### Structuring style sheets

A component may need styles that are common to all environments, like internal layout.
We write those styles in SCSS stylesheets that are imported into the host environment.
In some cases these can be overridden by passing another class name for that element.

#### CSS Modules

[CSS modules](https://github.com/css-modules/css-modules) is the way to avoid name conflicts in CSS classes.
To use this approach, colocate a SCSS stylesheet with the React component and use the `.module.scss.` suffix in a file name.

Example:

- `PageSelector.tsx` component would have a `PageSelector.module.scss` file next to it.
- Use `pnpm generate` to generate a TypeScript type declaration file: `PageSelector.module.scss.d.ts` in the same folder.
- After that, it's possible to type-safely use class names from the CSS module.

```tsx
import styles from './PageSelector.module.scss'

<button className={styles.pageSelectorButton} />
```

To use SCSS mixins/functions provided by the `branded` package in CSS modules use explicit imports to the required module.

```scss
@import 'branded/src/global-styles/functions.scss';
```

#### CSS classes naming convention

We do not use the [BEM convention](http://getbem.com/naming/) (Block - Element - Modifier) fully in CSS modules.

1. Prefer short descriptive classes specific only for the corresponding component because CSS modules provide scoping out of the box. It outputs shorter classes that are more readable in the component markup.
2. The use **modifiers**  is encouraged when it's needed. E.g. `--loading` or `--closed`

Example:

```scss
.some-component {
    // ... styles ...

    &--modifier {
        // ... styles ...
    }
}

.another-component {
    // ... styles ...
}
```

### Typography

Avoid ever overriding font family, text sizes or text colors.
These are set globally by the host environment for semantic HTML elements, e.g. `<h1>`, `<a>`, `<code>` or `<small>`.

### Theming

Theming is done through toggling top-level CSS classes `theme-light` and `theme-dark`.
Any style can be made different on either theme by scoping it to one of those two classes.
Where possible, we use CSS variables, but unfortunately they don't work with compile-time color manipulation (`darken()` etc)
and runtime color manipulation is not yet implemented in CSS (coming in CSS Color Level 4).

Example:

```scss
.some-component {
    // ... styles ...

    :global(.theme-dark) & {
        // ... styles ...
    }

    :global(.theme-light) & {
        // ... styles ...
    }
}
```

### Colors

The brand color palette is [OpenColor](https://yeun.github.io/open-color/).
In addition to these, we define a blueish grayscale palette for backgrounds, text and borders.
These colors are all available as CSS and SCSS variables.

However, directly referencing these may not work well in both light and dark themes, and may not match code host themes (if the component is shared).
The best approach is to not reference colors at all and use building blocks that have borders, text colors etc defined.
This saves code and makes it easy to maintain design consistency even if we want to change colors in the future.
When that is not possible (for example UI contributed by extensions), prefer to reference CSS variables with semantic colors like `var(--danger)`, `var(--success)`, `var(--border-color)`, `var(--body-bg)` etc.
The values of these variables are changed globally when the theme changes.
Be aware that this means our stylesheets for each host environment need to define these variables too.

### Spacing

We use `rem` units in all component styling and strive to use `0.25rem` steps.
This ensures our spacing generally aligns with an [8pt grid](https://medium.com/swlh/the-comprehensive-8pt-grid-guide-aa16ff402179), but also gracefully scales in environments that have a different base `rem` size.
In our webapp, it is recommended to make use of [Bootstrap's margin and padding utilities](https://getbootstrap.com/docs/4.5/utilities/spacing/), which are configured to align with the 8pt grid.

### Layout

We use modern CSS for our layouting needs. You can find a [small playground in our Storybook](https://main--5f0f381c0e50750022dc6bf7.chromatic.com/?path=/story/branded-global-styles--layout). The dev tools of modern browsers provide a lot of useful tooling to work with CSS layouts.

Layouts should always be _responsive_ to make sure Sourcegraph is usable with different screen resolutions and window sizes, e.g. when resizing the browser window and using Sourcegraph side-by-side with an editor.

[CSS Flexbox](https://css-tricks.com/snippets/css/a-guide-to-flexbox/) is used for **one-dimensional** layouts (single rows or columns, with optional wrapping). In the webapp, you can use utility classes for simple flexbox layouts and responsive layouts. This is the most common layout method.

For complex **two-dimensional** layouts, [CSS Grid](https://learncssgrid.com/) can be used.
