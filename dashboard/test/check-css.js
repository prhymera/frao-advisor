const { chromium } = require('@playwright/test');
(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 375, height: 667 } });
  await page.goto('http://10.64.0.5:9753');
  await page.waitForSelector('.kpi-grid', { timeout: 10000 });
  await page.waitForTimeout(1000);
  
  const info = await page.evaluate(() => {
    const sheets = Array.from(document.styleSheets);
    let cssRules = [];
    try {
      for (const sheet of sheets) {
        for (const rule of sheet.cssRules) {
          if (rule.media && rule.media.mediaText) {
            cssRules.push({ media: rule.media.mediaText, count: rule.cssRules.length });
          }
        }
      }
    } catch(e) { return { error: e.message, sheetCount: sheets.length }; }
    return { sheetCount: sheets.length, mediaRules: cssRules };
  });
  console.log('sheets:', info.sheetCount);
  console.log('media rules:', JSON.stringify(info.mediaRules, null, 2));
  
  // Check computed padding-bottom of content
  const padding = await page.evaluate(() => {
    const el = document.getElementById('content');
    const cs = getComputedStyle(el);
    return { paddingBottom: cs.paddingBottom, paddingTop: cs.paddingTop, paddingLeft: cs.paddingLeft, paddingRight: cs.paddingRight };
  });
  console.log('content padding:', JSON.stringify(padding));
  
  await browser.close();
})();
