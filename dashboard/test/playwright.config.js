// ═══════════════════════════════════════════════════════════════════════════
// Playwright config for Frao Advisor Dashboard tests
// ═══════════════════════════════════════════════════════════════════════════
const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: '.',
  timeout: 30000,
  expect: { timeout: 10000 },
  fullyParallel: false,
  forbidOnly: true,
  retries: 1,
  workers: 1,
  reporter: [
    ['list'],
    ['json', { outputFile: 'test-results.json' }],
  ],
  use: {
    baseURL: 'http://10.64.0.5:9753',
    actionTimeout: 10000,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
});
