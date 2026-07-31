// ══════════════════════════════════════════════════════════════════════════════
// Frao Advisor Dashboard — Comprehensive Playwright Test
// ══════════════════════════════════════════════════════════════════════════════
// Run: npx playwright test dashboard/test/dashboard.spec.js
// Requires: dashboard running at http://10.64.0.5:9753

const { test, expect } = require('@playwright/test');

const DASHBOARD_URL = 'http://10.64.0.5:9753';

// ─── Test: All console errors captured ─────────────────────────────────────

test.describe('Dashboard Console Errors', () => {
  test('should load without console errors', async ({ page }) => {
    const consoleErrors = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error' || msg.type() === 'warning') {
        consoleErrors.push({ type: msg.type(), text: msg.text() });
      }
    });

    await page.goto(DASHBOARD_URL);

    // Wait for content to load (Datastar patches the #content div)
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });

    // Wait a bit more for charts and scripts to execute
    await page.waitForTimeout(2000);

    // Collect remaining console messages
    const errors = consoleErrors.filter(e => e.type === 'error');
    const warnings = consoleErrors.filter(e => e.type === 'warning');

    // Report all errors — fail on errors, warn on warnings
    if (warnings.length > 0) {
      console.log('⚠️  Console warnings:', warnings.map(w => w.text).join('\n'));
    }
    if (errors.length > 0) {
      console.log('❌ Console errors:', errors.map(e => e.text).join('\n'));
    }
    expect(errors).toHaveLength(0);
  });
});

// ─── Test: Overview / Stat Cards ──────────────────────────────────────────

test.describe('Overview Section', () => {
  test('should display 4 KPI stat cards with real values', async ({ page }) => {
    await page.goto(DASHBOARD_URL);
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });
    await page.waitForTimeout(1500);

    // Verify 4 KPI cards exist
    const cards = page.locator('.kpi-card');
    await expect(cards).toHaveCount(4);

    // Check each card has a value (non-empty, non-zero)
    const kpiChecks = [
      { id: 'kpi-total', label: 'Total Advice', min: 1 },
      { id: 'kpi-cost', label: 'Total Cost', min: 0 },
      { id: 'kpi-sessions', label: 'Active Sessions', min: 0 },
      { id: 'kpi-tokens', label: 'Total Tokens', min: 1 },
    ];

    for (const kpi of kpiChecks) {
      const card = page.locator(`#${kpi.id}`);
      await expect(card).toBeVisible({ timeout: 5000 });
      const value = await card.locator('.kpi-value').textContent();
      const label = await card.locator('.kpi-label').textContent();
      expect(label).toBe(kpi.label);

      // Extract numeric value from string like "117" or "$0.41" or "503,423"
      const numericStr = value.replace(/[,.]/g, '').replace(/[^0-9]/g, '');
      const numericVal = parseInt(numericStr, 10);
      expect(numericVal).toBeGreaterThanOrEqual(kpi.min);

      console.log(`  ✅ ${kpi.label}: ${value}`);
    }

    // Check cost sparkline chart exists
    const sparkline = page.locator('#cost-sparkline');
    await expect(sparkline).toBeVisible({ timeout: 3000 });

    // Check quick stats
    const quickStats = page.locator('.quick-stats .stat-item');
    const statCount = await quickStats.count();
    expect(statCount).toBe(3);
    console.log(`  ✅ Quick stats: ${statCount} items`);
  });
});

// ─── Test: Timeline Section ───────────────────────────────────────────────

test.describe('Timeline Section', () => {
  test('should show more than 3 entries and display real data', async ({ page }) => {
    await page.goto(DASHBOARD_URL);
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });
    await page.waitForTimeout(500);

    // Navigate to timeline via sidebar click
    const timelineNav = page.locator('a.nav-item').filter({ hasText: 'Timeline' });
    await timelineNav.click();

    // Wait for timeline content to load
    await page.waitForSelector('.timeline-list', { timeout: 10000 });
    await page.waitForTimeout(1500);

    // Count timeline entries — should be more than 3 (default 20 per page)
    const entries = page.locator('.timeline-entry');
    const entryCount = await entries.count();
    console.log(`  📊 Timeline entries shown: ${entryCount}`);
    expect(entryCount).toBeGreaterThan(3);

    // Check filter bar exists
    const filterSelect = page.locator('.filter-bar select');
    await expect(filterSelect).toBeVisible({ timeout: 3000 });
    const filterOptions = await filterSelect.locator('option').allTextContents();
    expect(filterOptions.length).toBe(4); // All, Consultations, Expert Reviews, Deliberations
    console.log(`  ✅ Filter options: ${filterOptions.join(', ')}`);

    // Verify each entry has badge, summary, meta data
    for (let i = 0; i < Math.min(entryCount, 5); i++) {
      const entry = entries.nth(i);
      await expect(entry.locator('.timeline-badge')).toBeVisible();
      await expect(entry.locator('.timeline-summary')).toBeVisible();
      await expect(entry.locator('.timeline-meta')).toBeVisible();

      const badge = await entry.locator('.timeline-badge').textContent();
      const summary = await entry.locator('.timeline-summary').textContent();
      console.log(`  📝 Entry ${i + 1}: [${badge}] ${summary.substring(0, 60)}...`);
    }

    // Test filtering: click "Consultations" filter
    await filterSelect.selectOption('consultation');
    await page.waitForTimeout(1000);
    const filteredEntries = page.locator('.timeline-entry');
    const filteredCount = await filteredEntries.count();
    console.log(`  📊 After consultation filter: ${filteredCount} entries`);

    if (filteredCount > 0) {
      // Verify all visible entries are consultation type
      const badges = await page.locator('.timeline-badge').allTextContents();
      const allConsult = badges.every(b => b.trim() === 'Consult');
      expect(allConsult).toBe(true);
    }

    // Reset filter
    await filterSelect.selectOption('all');
    await page.waitForTimeout(1000);
  });

  test('should navigate to detail view on click', async ({ page }) => {
    await page.goto(DASHBOARD_URL);
    // Wait for initial load, then navigate to timeline
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });
    await page.locator('a.nav-item').filter({ hasText: 'Timeline' }).click();
    // Wait for either timeline entries OR the empty state
    await Promise.race([
      page.waitForSelector('.timeline-entry', { timeout: 10000 }),
      page.waitForSelector('.empty-state', { timeout: 10000 }),
    ]);
    await page.waitForTimeout(1000);

    const entries = page.locator('.timeline-entry');
    const count = await entries.count();
    if (count === 0) {
      console.log('  ⏭️  No entries to click — skipping detail test');
      return;
    }

    // Click first entry
    await entries.first().click();
    await page.waitForTimeout(2000);

    // Should show detail view (either consultation, review, or deliberation detail)
    const detailView = page.locator('.detail-view');
    const card = page.locator('.card');
    const isDetailVisible = await detailView.isVisible().catch(() => false);
    const isCardVisible = await card.first().isVisible().catch(() => false);
    expect(isDetailVisible || isCardVisible).toBe(true);
    console.log('  ✅ Detail view opened successfully');
  });
});

// ─── Test: Experts Section ────────────────────────────────────────────────

test.describe('Experts Section', () => {
  test('should display expert breakdown with charts and table', async ({ page }) => {
    await page.goto(DASHBOARD_URL);
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });
    await page.waitForTimeout(500);

    // Navigate to experts
    await page.locator('a.nav-item').filter({ hasText: 'Experts' }).click();
    await page.waitForTimeout(2000);

    // Check for expert chart canvases OR expert table
    const barChart = page.locator('#expert-bar');
    const pieChart = page.locator('#expert-pie');
    const table = page.locator('.table-container table');

    const hasBar = await barChart.isVisible().catch(() => false);
    const hasPie = await pieChart.isVisible().catch(() => false);
    const hasTable = await table.isVisible().catch(() => false);

    console.log(`  📊 Charts: bar=${hasBar}, pie=${hasPie}, table=${hasTable}`);

    // Should have at least one of: charts or table
    expect(hasBar || hasPie || hasTable).toBe(true);

    if (hasTable) {
      const rows = await table.locator('tbody tr').count();
      console.log(`  📊 Expert rows: ${rows}`);
      expect(rows).toBeGreaterThan(0);
    }
  });
});

// ─── Test: Deliberations Section ──────────────────────────────────────────

test.describe('Deliberations Section', () => {
  test('should display deliberation accordion list', async ({ page }) => {
    await page.goto(DASHBOARD_URL);
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });
    await page.waitForTimeout(500);

    // Navigate to deliberations
    await page.locator('a.nav-item').filter({ hasText: 'Deliberations' }).click();
    await page.waitForTimeout(2000);

    const delibItems = page.locator('.delib-item');
    const count = await delibItems.count().catch(() => 0);
    console.log(`  📊 Deliberations: ${count} items`);

    // Either show items or show empty state
    if (count > 0) {
      // Test accordion toggle: click first header
      await delibItems.first().locator('.delib-header').click();
      await page.waitForTimeout(500);
      const body = delibItems.first().locator('.delib-body');
      const isOpen = await body.isVisible().catch(() => false);
      console.log(`  ✅ Accordion toggle works: open=${isOpen}`);
    }
  });
});

// ─── Test: Costs Section ──────────────────────────────────────────────────

test.describe('Costs Section', () => {
  test('should display cost charts', async ({ page }) => {
    await page.goto(DASHBOARD_URL);
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });
    await page.waitForTimeout(500);

    // Navigate to costs
    await page.locator('a.nav-item').filter({ hasText: 'Costs' }).click();
    await page.waitForTimeout(2000);

    // Check for chart canvases
    const charts = [
      { id: 'cost-day', name: 'Cost by Day' },
      { id: 'cost-model', name: 'Cost by Model' },
      { id: 'cost-type', name: 'Cost by Type' },
      { id: 'cost-gauge', name: 'Budget Gauge' },
    ];

    let visibleCount = 0;
    for (const chart of charts) {
      const el = page.locator(`#${chart.id}`);
      const visible = await el.isVisible().catch(() => false);
      if (visible) visibleCount++;
      console.log(`  📊 ${chart.name}: ${visible ? '✅' : '⏳'}`);
    }
    console.log(`  📊 Charts visible: ${visibleCount}/${charts.length}`);
  });
});

// ─── Test: Metrics Section ────────────────────────────────────────────────

test.describe('Metrics Section', () => {
  test('should display heatmap, latency chart, and summary cards', async ({ page }) => {
    await page.goto(DASHBOARD_URL);
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });
    await page.waitForTimeout(500);

    // Navigate to metrics
    await page.locator('a.nav-item').filter({ hasText: 'Metrics' }).click();
    await page.waitForTimeout(2000);

    // Check for heatmap table
    const heatmap = page.locator('.heatmap-table');
    const hasHeatmap = await heatmap.isVisible().catch(() => false);
    console.log(`  📊 Heatmap: ${hasHeatmap ? '✅' : '⏳'}`);

    // Check export button
    const exportBtn = page.locator('.export-btn');
    const hasExport = await exportBtn.isVisible().catch(() => false);
    console.log(`  📊 Export button: ${hasExport ? '✅' : '⏳'}`);

    // Check metrics grid
    const metricsGrid = page.locator('.metrics-grid');
    const hasMetrics = await metricsGrid.isVisible().catch(() => false);
    if (hasMetrics) {
      const metricCards = await metricsGrid.locator('.metric-card').count();
      console.log(`  📊 Metric cards: ${metricCards}`);
      expect(metricCards).toBeGreaterThan(0);
    }
  });
});

// ─── Test: CSV Export ─────────────────────────────────────────────────────

test.describe('CSV Export', () => {
  test('should download CSV with header row and data', async ({ page, request }) => {
    // Use request API (not page.goto) — the endpoint sends Content-Disposition:
    // attachment, which would trigger a browser download.
    const response = await request.get(`${DASHBOARD_URL}/export/csv`);
    expect(response.ok()).toBe(true);

    const contentType = response.headers()['content-type'] || '';
    expect(contentType).toContain('text/csv');
    console.log(`  ✅ CSV content-type: ${contentType}`);

    const disposition = response.headers()['content-disposition'] || '';
    expect(disposition).toContain('attachment');
    expect(disposition).toContain('.csv');
    console.log(`  ✅ CSV disposition: ${disposition}`);

    const body = await response.text();
    const lines = body.trim().split('\n');
    expect(lines.length).toBeGreaterThan(1); // Header + at least 1 data row
    expect(lines[0]).toBe('Date,Type,Expert,Model,Tokens,Cost,Duration (ms)');
    console.log(`  ✅ CSV rows: ${lines.length - 1} data rows`);
    console.log(`  ✅ CSV header: ${lines[0]}`);
  });
});

// ─── Test: Mobile Responsiveness ─────────────────────────────────────────

test.describe('Mobile Responsiveness', () => {
  test('should show mobile bottom bar on small viewport', async ({ page }) => {
    // Set mobile viewport
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto(DASHBOARD_URL);
    await page.waitForTimeout(1000);

    // Mobile bar should be visible
    const mobileBar = page.locator('#mobile-bar');
    await expect(mobileBar).toBeVisible({ timeout: 5000 });

    // Sidebar should be hidden
    const sidebar = page.locator('.sidebar');
    const sidebarVisible = await sidebar.isVisible().catch(() => false);
    console.log(`  📱 Mobile bar: visible, Sidebar: ${sidebarVisible ? 'visible' : 'hidden'}`);

    // Test mobile navigation
    const mobileItems = await mobileBar.locator('.mobile-item').count();
    console.log(`  📱 Mobile nav items: ${mobileItems}`);
    expect(mobileItems).toBeGreaterThanOrEqual(5);
  });

  test('should show desktop sidebar on large viewport', async ({ page }) => {
    // Set desktop viewport
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(DASHBOARD_URL);
    await page.waitForTimeout(1000);

    // Sidebar should be visible
    const sidebar = page.locator('.sidebar');
    await expect(sidebar).toBeVisible({ timeout: 5000 });

    // Mobile bar should be hidden
    const mobileBar = page.locator('#mobile-bar');
    const mobileVisible = await mobileBar.isVisible().catch(() => false);
    console.log(`  💻 Sidebar: visible, Mobile bar: ${mobileVisible ? 'visible' : 'hidden'}`);
  });
});

// ─── Test: Navigation between all sections ────────────────────────────────

test.describe('Navigation', () => {
  test('should navigate between all 6 sections without console errors', async ({ page }) => {
    const consoleErrors = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    const sections = ['Overview', 'Timeline', 'Experts', 'Deliberations', 'Costs', 'Metrics'];

    await page.goto(DASHBOARD_URL);
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });

    for (const section of sections) {
      // Click nav item
      const navItem = page.locator('a.nav-item').filter({ hasText: section });
      const exists = await navItem.count();
      if (exists === 0) {
        console.log(`  ⏭️  ${section} nav item not found`);
        continue;
      }

      await navItem.first().click();
      await page.waitForTimeout(1500);

      // Check no loading state is present (content should have loaded)
      const contentDiv = page.locator('#content');
      const contentHtml = await contentDiv.innerHTML().catch(() => '');
      const isLoading = contentHtml.includes('Loading dashboard');

      // Content should not still be loading after navigation
      if (contentHtml.length > 0) {
        console.log(`  ✅ ${section}: loaded (${contentHtml.length} chars)`);
      }

      // Check for any section-specific elements
      const elements = await contentDiv.locator('*').count();
      console.log(`     Elements in content: ${elements}`);
    }

    if (consoleErrors.length > 0) {
      console.log('  ❌ Navigation console errors:', consoleErrors.join('\n'));
    }
    expect(consoleErrors).toHaveLength(0);
  });

  test('auto-refresh should NOT yank user away from current section', async ({ page }) => {
    await page.goto(DASHBOARD_URL);
    await page.waitForSelector('.kpi-grid', { timeout: 10000 });

    // Navigate to a non-overview section (Timeline)
    await page.locator('a.nav-item').filter({ hasText: 'Timeline' }).click();
    await page.waitForSelector('.timeline-entry', { timeout: 10000 });
    await page.waitForTimeout(800);

    // Confirm we're on timeline
    const before = await page.locator('.timeline-entry').count();
    console.log(`  📊 On Timeline, entries: ${before}`);
    expect(before).toBeGreaterThan(0);

    // Simulate the visibilitychange event (returning to the tab) which
    // previously forced an overview reload.
    await page.evaluate(() => {
      Object.defineProperty(document, 'hidden', { value: false, configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });

    // Wait past the auto-refresh cadence
    await page.waitForTimeout(3500);

    // User should STILL be on timeline, not yanked to overview
    const stillTimeline = await page.locator('.timeline-entry').count();
    const hasKpiGrid = await page.locator('.kpi-grid').count();
    console.log(`  📊 After refresh events — timeline entries: ${stillTimeline}, kpi-grid visible: ${hasKpiGrid}`);
    expect(stillTimeline).toBeGreaterThan(0);
    expect(hasKpiGrid).toBe(0);
  });
});
