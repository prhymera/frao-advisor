const { chromium } = require('@playwright/test');
(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
  
  // Intercept the timeline response
  page.on('response', async (res) => {
    if (res.url().includes('/dashboard/timeline')) {
      const body = await res.text().catch(() => 'ERROR');
      console.log('TIMELINE RESPONSE length:', body.length);
      console.log('first 100:', body.substring(0, 100));
      console.log('entry count:', (body.match(/timeline-entry/g) || []).length);
    }
  });
  
  await page.goto('http://10.64.0.5:9753');
  await page.waitForSelector('.kpi-grid', { timeout: 10000 });
  await page.waitForTimeout(500);
  
  // Click timeline nav
  await page.locator('a.nav-item').filter({ hasText: 'Timeline' }).click();
  await page.waitForTimeout(3000);
  
  await browser.close();
})();
