const { chromium } = require('@playwright/test');
(async () => {
  const browser = await chromium.launch();
  
  // Desktop
  const desktop = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  await desktop.goto('http://10.64.0.5:9753');
  await desktop.waitForSelector('.kpi-grid', { timeout: 10000 });
  await desktop.waitForTimeout(1500);
  await desktop.screenshot({ path: '/tmp/desktop-overview.png' });
  
  // Desktop timeline
  await desktop.locator('a.nav-item').filter({ hasText: 'Timeline' }).click();
  await desktop.waitForSelector('.timeline-list', { timeout: 10000 });
  await desktop.waitForTimeout(1500);
  await desktop.screenshot({ path: '/tmp/desktop-timeline.png' });
  
  // Desktop metrics (has heatmap)
  await desktop.locator('a.nav-item').filter({ hasText: 'Metrics' }).click();
  await desktop.waitForTimeout(2000);
  await desktop.screenshot({ path: '/tmp/desktop-metrics.png' });
  
  // Mobile
  const mobile = await browser.newPage({ viewport: { width: 375, height: 667 } });
  await mobile.goto('http://10.64.0.5:9753');
  await mobile.waitForSelector('.kpi-grid', { timeout: 10000 });
  await mobile.waitForTimeout(1500);
  await mobile.screenshot({ path: '/tmp/mobile-overview.png' });
  
  // Mobile timeline
  await mobile.locator('.mobile-item').filter({ hasText: 'Timeline' }).click();
  await mobile.waitForSelector('.timeline-list', { timeout: 10000 });
  await mobile.waitForTimeout(1500);
  await mobile.screenshot({ path: '/tmp/mobile-timeline.png' });
  
  // Mobile scroll to bottom
  await mobile.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
  await mobile.waitForTimeout(500);
  await mobile.screenshot({ path: '/tmp/mobile-timeline-bottom.png' });
  
  console.log('screenshots saved');
  await browser.close();
})();
