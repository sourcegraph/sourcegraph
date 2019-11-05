const baseConfig = require('../.eslintrc')
module.exports = {
  extends: '../.eslintrc.js',
  parserOptions: {
    ...baseConfig.parserOptions,
    project: 'tsconfig.json',
  },
  rules: {
    'no-console': ['error'],
    'import/no-cycle': ['error'],
    'no-return-await': ['error'],
  },
  overrides: baseConfig.overrides,
}
