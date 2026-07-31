const { chromium } = require('@playwright/test');

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
  const logs = [];
  page.on('console', m => logs.push(m.type() + ': ' + m.text()));
  
  await page.goto('http://10.64.0.5:9753');
  await page.waitForSelector('.kpi-grid', { timeout: 10000 });
  await page.waitForTimeout(1000);
  
  // Navigate to timeline
  await page.locator('a.nav-item').filter({ hasText: 'Timeline' }).click();
  await page.waitForTimeout(2000);
  
  // Inspect content
  const content = await page.locator('#content').innerHTML();
  console.log('CONTENT LENGTH:', content.length);
  console.log('ENTRY COUNT:', await page.locator('.timeline-entry').count());
  console.log('FIRST 500 CHARS:', content.substring(0, 500));
  console.log('LAST 200 CHARS:', content.substring(content.length - 200));
  console.log('LOGS:', logs.join('\n'));
  
  await browser.close();
})();
