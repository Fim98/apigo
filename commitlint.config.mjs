export default {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'header-max-length': [2, 'always', 100],
    'scope-enum': [
      2,
      'always',
      [
        'app',
        'ui',
        'bru',
        'collection',
        'request',
        'auth',
        'vars',
        'script',
        'assert',
        'runner',
        'cli',
        'proto',
        'import',
        'export',
        'git',
        'store',
        'docs',
        'build',
        'ci',
        'deps',
        'release',
        'test'
      ]
    ]
  }
};
