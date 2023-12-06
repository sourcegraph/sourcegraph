// @ts-check

// Use faster experimental way of doing multi-project TypeScript linting. See
// https://github.com/typescript-eslint/typescript-eslint/pull/6754.
process.env.TYPESCRIPT_ESLINT_EXPERIMENTAL_TSSERVER = 'true'

const config = {
  root: true,
  ignorePatterns: [
    '**/graphql-operations.ts',
    '**/graphql-types.ts',
    '**/node_modules/**',
    'out/',
    'dist/',
    'src/schema/*',
    'graphql-operations.ts',
    'GH2SG.bookmarklet.js',
    '**/vendor/*.js',
    'svelte.config.js',
    'vite.config.ts',
    'vitest.config.ts',
    'postcss.config.js',
    'playwright.config.ts',
    'bundlesize.config.js',
    'prettier.config.js',
    'svgo.config.js',
    '.vscode-test',
    '**/*.json',
    '**/*.d.ts',
    'eslint-relative-formatter.js',
    'typedoc.js',
    'client/web/dev/**/*',
    'graphql-schema-linter.config.js',
    'mockServiceWorker.js',
  ],
  extends: ['@sourcegraph/eslint-config', 'plugin:storybook/recommended'],
  env: {
    browser: true,
    node: true,
    es6: true,
  },
  parserOptions: {
    ecmaVersion: 2018,
    sourceType: 'module',
    ecmaFeatures: {
      jsx: true,
    },
    EXPERIMENTAL_useProjectService: true,
    project: true,
  },
  settings: {
    react: {
      version: 'detect',
    },
    linkComponents: [
      {
        name: 'LinkOrSpan',
        linkAttribute: 'to',
      },
      {
        name: 'Link',
        linkAttribute: 'to',
      },
    ],
  },
  plugins: ['@sourcegraph/sourcegraph', 'monorepo', '@sourcegraph/wildcard', 'storybook'],
  rules: {
    // Rules that are specific to this repo
    // All other rules should go into https://github.com/sourcegraph/eslint-config
    'no-console': 'error',
    'monorepo/no-relative-import': 'error',
    '@sourcegraph/sourcegraph/check-help-links': 'error',
    // This rule doesn't understand type imports and we already have
    // import/no-duplicates enabled as well, which does understand type imports
    'no-duplicate-imports': 'off',
    'id-length': 'off',
    'no-void': 'off',
    '@typescript-eslint/consistent-type-exports': 'warn',
    '@typescript-eslint/consistent-type-imports': [
      'warn',
      {
        fixStyle: 'inline-type-imports',
        disallowTypeAnnotations: false,
      },
    ],
    // This converts 'import {type foo} from ...' to 'import type {foo} from ...'
    '@typescript-eslint/no-import-type-side-effects': ['warn'],

    // These rules are very slow on-save.
    '@typescript-eslint/no-unsafe-assignment': 'off',
    '@typescript-eslint/unbound-method': 'off',
    '@typescript-eslint/no-misused-promises': 'off',
    '@typescript-eslint/no-unnecessary-qualifier': 'off',
    '@typescript-eslint/no-unused-vars': 'off', // also duplicated by tsconfig noUnused{Locals,Parameters}
    'etc/no-deprecated': 'off',

    'no-restricted-imports': [
      'error',
      {
        paths: [
          'highlight.js',
          'marked',
          'rxjs/ajax',
          {
            name: 'rxjs',
            importNames: ['animationFrameScheduler'],
            message: 'Code using animationFrameScheduler breaks in Firefox when using Sentry.',
          },
          {
            name: 'react-router-dom',
            importNames: ['Link'],
            message: 'Use the <Link /> component from @sourcegraph/wildcard instead.',
          },
          {
            name: 'chromatic/isChromatic',
            message: 'Please use `isChromatic` from the `@sourcegraph/storybook` package.',
          },
        ],
        patterns: [
          {
            group: ['@sourcegraph/branded/src/search-ui/experimental'],
            message:
              'The experimental search input is not available for general use. If you have questions about it reach out to the search product team.',
          },
          {
            group: [
              '@sourcegraph/*/src/*',
              '@sourcegraph/*/src/testing/*',
              '@sourcegraph/*/src/stories/*',
              '!@sourcegraph/branded/src/*',
              '!@sourcegraph/branded/src/testing/*',
              '!@sourcegraph/shared/src/*',
              '!@sourcegraph/shared/src/testing/*',
              '!@sourcegraph/web/src/SourcegraphWebApp.scss',
              '!@sourcegraph/branded/src/search-ui/experimental',
              '!@sourcegraph/*/src/testing',
              '!@sourcegraph/*/src/stories',
              '!@sourcegraph/build-config/src/esbuild/*',
              '!@sourcegraph/build-config/src/*',
              '!@sourcegraph/testing/src/jestDomMatchers',
            ],
            message:
              'Imports from package internals are banned. Add relevant export to the entry point of the package to import it from the outside world.',
          },
          {
            group: ['**/out/*'],
            message:
              "Please don't import stuff from the 'out' directory. It’s generated code. Remove the 'out/' part and you should be good go to.",
          },
          {
            group: ['!@sourcegraph/cody-shared/*', '!@sourcegraph/cody-ui/*'],
            message:
              "Allowed imports from @sourcegraph/cody-* packages while those packages' APIs are being stabilized.",
          },
        ],
      },
    ],
    'react/forbid-elements': [
      'error',
      {
        forbid: [
          {
            element: 'form',
            message:
              'Use the Form component in src/components/Form.tsx instead of the native HTML form element to get proper form validation feedback',
          },
          {
            element: 'input',
            message: 'Use the <Input/> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'select',
            message: 'Use the <Select /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'textarea',
            message: 'Use the <TextArea /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'a',
            message: 'Use the <Link /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'h1',
            message: 'Use the <H1 /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'h2',
            message: 'Use the <H2 /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'h3',
            message: 'Use the <H3 /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'h4',
            message: 'Use the <H4 /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'h5',
            message: 'Use the <H5 /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'h6',
            message: 'Use the <H6 /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'p',
            message:
              'Use the <Text /> component from @sourcegraph/wildcard instead. Check out the RFC for more context: https://bit.ly/3PFw0HM',
          },
          {
            element: 'code',
            message: 'Use the <Code /> component from @sourcegraph/wildcard instead.',
          },
          {
            element: 'label',
            message: 'Use the <Label /> component from @sourcegraph/wildcard instead.',
          },
        ],
      },
    ],
    '@sourcegraph/wildcard/forbid-class-name': [
      'error',
      {
        forbid: [
          {
            className: 'badge',
            message: 'Use the <Badge /> component from @sourcegraph/wildcard instead.',
          },
          {
            className: 'icon-inline',
            message: 'Use the <Icon /> component from @sourcegraph/wildcard instead.',
          },
        ],
      },
    ],
    'react/jsx-no-target-blank': ['error', { allowReferrer: true }],
    'no-restricted-syntax': [
      'warn',
      {
        selector: 'CallExpression[callee.name="useLocalStorage"]',
        message:
          'Consider using useTemporarySetting instead of useLocalStorage so settings are synced when users log in elsewhere. More info at https://docs.sourcegraph.com/dev/background-information/web/temporary_settings',
      },
      {
        selector: 'JSXAttribute JSXIdentifier[name="data-tooltip"]',
        message:
          'The use of data-tooltip has been deprecated. Please wrap your trigger element with the <Tooltip> component from Wildcard instead. If there are problems using the new <Tooltip>, please contact the Frontend Platform Team.',
      },
      {
        selector:
          'JSXElement[openingElement.name.name="Tooltip"] > JSXElement[openingElement.name.name="Icon"]:has(JSXIdentifier[name="aria-hidden"])',
        message:
          'When using an icon as a Tooltip trigger, it must have an aria-label attribute and should not be hidden.',
      },
      {
        selector: 'JSXElement[openingElement.name.name="InputTooltip"]',
        message:
          'Prefer using the <Tooltip> component with an <Input> directly, when possible. Please only use <InputTooltip> when the legacy styling it provides is needed. We will be working to fix style issues with <Input> (especially for checkboxes) in the future.',
      },
      {
        selector: 'JSXSpreadAttribute[argument.name=/^(props|rest)$/]',
        message:
          "Spreading props can be unsafe. Prefer destructuring the props object, or continue only if you're sure.",
      },
      {
        selector: 'ImportDeclaration[source.value="react-router"]',
        message:
          'Use `react-router-dom-v5-compat` instead. We are in the process of migrating from react-router v5 to v6. More info https://github.com/sourcegraph/sourcegraph/issues/33834',
      },
    ],
    // https://reactjs.org/blog/2020/09/22/introducing-the-new-jsx-transform.html#eslint
    'react/jsx-uses-react': 'off',
    'react/react-in-jsx-scope': 'off',
    'import/extensions': [
      'error',
      'never',
      {
        schema: 'always',
        scss: 'always',
        css: 'always',
        yaml: 'always',
        svg: 'always',
        cjs: 'always',
      },
    ],
    'import/order': 'off',
    'unicorn/expiring-todo-comments': 'off',

    // These rules were newly introduced in @sourcegraph/eslint-config@0.35.0 and have not yet been
    // fixed in our existing code.
    'unicorn/prefer-top-level-await': 'warn',
    'unicorn/prefer-logical-operator-over-ternary': 'warn',
    'unicorn/prefer-blob-reading-methods': 'warn',
    'unicorn/prefer-event-target': 'warn',
    'etc/throw-error': 'warn',
    'rxjs/throw-error': 'warn',
    'prefer-promise-reject-errors': 'warn',
    '@typescript-eslint/no-redundant-type-constituents': 'warn',
    '@typescript-eslint/no-unsafe-enum-comparison': 'warn',
    '@typescript-eslint/prefer-optional-chain': 'warn',
    '@typescript-eslint/no-duplicate-enum-values': 'warn',
    '@typescript-eslint/no-floating-promises': 'warn',

    'jsdoc/check-alignment': 'off',

    'unicorn/no-negated-condition': 'off', // this one reduces code readability, should remove it from @sourcegraph/eslint-config too
  },
  overrides: [
    {
      files: ['*.d.ts'],
      rules: {
        'no-restricted-imports': 'off',
      },
    },
    {
      files: '*.story.tsx',
      rules: {
        'react/forbid-dom-props': 'off',
        'import/no-default-export': 'off',
      },
    },
    {
      files: ['**/dev/**/*.ts', '**/story/**.tsx', '**/story/**.ts', '*.story.tsx', 'client/build-config/**'],
      rules: {
        'no-console': 'off',
        'no-sync': 'off',
      },
    },
    {
      files: ['client/browser/**', 'client/jetbrains/**'],
      rules: {
        'no-console': 'off',
      },
    },

    // client/web
    {
      files: ['client/web/src/stores/**.ts', 'client/web/src/__mocks__/zustand.ts'],
      rules: { 'no-restricted-imports': 'off' },
    },
  ],
}

module.exports = config
