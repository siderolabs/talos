module.exports = {
  root: true,
  env: {
    browser: true,
    node: true
  },
  parserOptions: {
    parser: 'babel-eslint'
  },
  extends: [
    '@nuxtjs',
    'standard',
    'prettier',
    'prettier/vue',
    'prettier/standard',
    'plugin:prettier/recommended',
    'plugin:nuxt/recommended'
  ],
  plugins: ['prettier', 'standard'],
  // add your custom rules here
  rules: {}
}
