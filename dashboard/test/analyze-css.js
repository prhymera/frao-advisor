const { chromium } = require('@playwright/test');
(async () => {
  const browser = await chromium.launch();
  
  async function analyze(label, viewport, section) {
    const page = await browser.newPage({ viewport });
    await page.goto('http://10.64.0.5:9753');
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });
    await page.waitForTimeout(800);
    
    // Navigate to section
    const navSel = viewport.width < 768 ? '.mobile-item' : '.nav-item';
    await page.locator(navSel).filter({ hasText: section }).click();
    await page.waitForTimeout(1500);
    
    const layout = await page.evaluate(() => {
      const get = (el) => {
        if (!el) return null;
        const r = el.getBoundingClientRect();
        const cs = getComputedStyle(el);
        return {
          top: Math.round(r.top), bottom: Math.round(r.bottom),
          height: Math.round(r.height), left: Math.round(r.left), right: Math.round(r.right),
          width: Math.round(r.width),
          position: cs.position, zIndex: cs.zIndex,
          display: cs.display, overflow: cs.overflow,
          paddingBottom: cs.paddingBottom,
        };
      };
      return {
        viewport: { w: window.innerWidth, h: window.innerHeight },
        mobileBar: get(document.getElementById('mobile-bar')),
        sidebar: get(document.querySelector('.sidebar')),
        content: get(document.getElementById('content')),
        body: get(document.body),
        documentHeight: document.body.scrollHeight,
        // Check if mobile bar overlaps content
        contentBottom: document.getElementById('content').getBoundingClientRect().bottom,
        viewportHeight: window.innerHeight,
      };
    });
    console.log(`=== ${label} (${section}) ===`);
    console.log('viewport:', JSON.stringify(layout.viewport));
    console.log('mobileBar:', JSON.stringify(layout.mobileBar));
    console.log('content:', JSON.stringify(layout.content));
    console.log('document height:', layout.documentHeight);
    console.log('contentBottom vs viewportHeight:', layout.contentBottom, 'vs', layout.viewportHeight);
    console.log('overlap if mobileBar visible and contentBottom > viewportHeight - barHeight');
    
    // Check the element at the very bottom
    const bottomEl = await page.evaluate(() => {
      const all = document.querySelectorAll('#content *');
      let maxBottom = 0, maxEl = null;
      for (const el of all) {
        const r = el.getBoundingClientRect();
        if (r.bottom > maxBottom) { maxBottom = r.bottom; maxEl = el; }
      }
      return { maxBottom: Math.round(maxBottom), maxElClass: maxEl ? maxEl.className : null };
    });
    console.log('bottom element:', JSON.stringify(bottomEl));
    await page.close();
  }
  
  await analyze('Mobile', { width: 375, height: 667 }, 'Overview');
  await analyze('Mobile', { width: 375, height: 667 }, 'Timeline');
  await analyze('Mobile', { width: 375, height: 667 }, 'Metrics');
  await analyze('Desktop', { width: 1440, height: 900 }, 'Overview');
  await analyze('Desktop', { width: 1440, height: 900 }, 'Metrics');
  
  await browser.close();
})();
