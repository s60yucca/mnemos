module.exports = {
  testEnvironment: 'node',
  coverageDirectory: 'coverage',
  collectCoverageFrom: ['lib/**/*.js', 'cli.js'],
  testMatch: ['**/test/**/*.test.js'],
  verbose: true,
};
