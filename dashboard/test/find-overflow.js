const { chromium } = require('@playwright/test');
(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 375, height: 667 } });
  await page.goto('http://10.64.0.5:9753');
  await page.waitForSelector('.kpi-grid', { timeout: 10000 });
  await page.waitForTimeout(800);
  await page.locator('.mobile-item').filter({ hasText: 'Timeline' }).click();
  await page.waitForSelector('.timeline-list', { timeout: 10000 });
  await page.waitForTimeout(1200);
  
  const overflowEls = await page.evaluate(() => {
    const results = [];
    const all = document.querySelectorAll('#content *');
    for (const el of all) {
      const r = el.getBoundingClientRect();
      if (r.right > 375 || r.left < 0) {
        results.push({
          tag: el.tagName, className: el.className.substring(0, 50),
          right: Math.round(r.right), left: Math.round(r.left),
          width: Math.round(r.width),
          scrollWidth: el.scrollWidth, clientWidth: el.clientWidth,
          whiteSpace: getComputedStyle(el).whiteSpace,
          overflow: getComputedStyle(el).overflow
        });
      }
    }
    // dedupe by className, keep first 15
    const seen = new Set();
    return results.filter(r => {
      if (seen.has(r.className + r.tag)) return false;
      seen.add(r.className + r.tag);
      return true;
    }).slice(0, 15);
  });
  console.log('Overflowing elements (right > 375 or left < 0):');
  console.log(JSON.stringify(overflowEls, null, 2));
  
  await browser.close();
})();
