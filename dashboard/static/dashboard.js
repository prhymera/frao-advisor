// ─── Frao Advisor Dashboard ───
// Vanilla JS SPA loader. No framework dependencies.
// Fetches Datastar-style SSE fragments from the Go server and patches #content.

// ════════════════════════════════════════════════════════════════════════════
// Chart.js defaults + FraoDashboard chart helper framework
// ════════════════════════════════════════════════════════════════════════════

// Chart.js global defaults — dark theme, animation, tooltips
Chart.defaults.animation.duration = 800;
Chart.defaults.animation.easing = 'easeOutQuart';
Chart.defaults.color = '#94a3b8';
Chart.defaults.borderColor = 'rgba(148,163,184,0.1)';
Chart.defaults.plugins.tooltip.backgroundColor = '#1e293b';
Chart.defaults.plugins.tooltip.titleColor = '#f1f5f9';
Chart.defaults.plugins.tooltip.bodyColor = '#94a3b8';
Chart.defaults.plugins.tooltip.borderColor = 'rgba(148,163,184,0.2)';
Chart.defaults.plugins.tooltip.borderWidth = 1;
Chart.defaults.plugins.tooltip.padding = 10;
Chart.defaults.plugins.tooltip.cornerRadius = 6;

// Color palette
var PALETTE = ['#06b6d4','#f59e0b','#8b5cf6','#10b981','#f43f5e','#14b8a6','#ec4899','#f97316'];

// Register a plugin for centered text on doughnut/gauge charts
var centerTextPlugin = {
  id: 'centerText',
  afterDraw: function(chart, args, opts) {
    if (!opts || !opts.text) return;
    var w = chart.width, h = chart.height, c = chart.ctx;
    c.save();
    c.textAlign = 'center';
    c.textBaseline = 'middle';
    var lines = Array.isArray(opts.text) ? opts.text : [opts.text];
    var lineH = Math.max(14, h / 12);
    var startY = (h - (lines.length - 1) * lineH) / 2;
    lines.forEach(function(line, i) {
      var isMain = i === 0;
      c.font = (isMain ? Math.max(16, h / 10) : Math.max(10, h / 16)).toFixed(0) + 'px "JetBrains Mono", monospace';
      c.fillStyle = isMain ? '#f1f5f9' : '#64748b';
      c.fillText(String(line), w / 2, startY + i * lineH);
    });
    c.restore();
  }
};
Chart.register(centerTextPlugin);

// Respect prefers-reduced-motion
var motionMedia = window.matchMedia('(prefers-reduced-motion: reduce)');
if (motionMedia.matches) { Chart.defaults.animation = false; }
motionMedia.addEventListener('change', function(e) {
  Chart.defaults.animation = !e.matches;
});

window.FraoDashboard = {
  destroyChart: function(id) {
    var existing = Chart.getChart(id);
    if (existing) { try { existing.destroy(); } catch(e) {} }
  },
  create: function(id, config) {
    this.destroyChart(id);
    var el = document.getElementById(id);
    if (!el) return;
    var ctx = el.getContext('2d');
    if (!ctx) return;
    try { new Chart(ctx, config); } catch(e) { console.warn('chart:', id, e); }
  },

  sparkline: function(id, values) {
    var labels = values.map(function(_, i) { return i + ''; });
    this.create(id, {
      type: 'line',
      data: { labels: labels, datasets: [{ data: values, borderColor: '#06b6d4', backgroundColor: 'transparent', borderWidth: 2, pointRadius: 0, fill: false, tension: 0.4 }] },
      options: { responsive: true, maintainAspectRatio: true, plugins: { legend: { display: false }, tooltip: { callbacks: { title: function() { return ''; }, label: function(ctx) { return '$' + ctx.parsed.y.toFixed(4); } } } }, scales: { x: { display: false }, y: { display: false } } }
    });
  },

  bar: function(id, labels, values, label, colors) {
    if (!Array.isArray(colors)) {
      var c = colors || 'rgba(6,182,212,0.7)';
      colors = labels.map(function() { return c; });
    }
    this.create(id, {
      type: 'bar',
      data: { labels: labels, datasets: [{ label: label || '', data: values, backgroundColor: colors, borderRadius: 4 }] },
      options: { responsive: true, maintainAspectRatio: true, plugins: { legend: { display: false } }, scales: { x: { grid: { display: false }, ticks: { color: '#94a3b8' } }, y: { grid: { color: 'rgba(148,163,184,0.1)' }, ticks: { color: '#94a3b8' } } } }
    });
  },

  pie: function(id, labels, values) {
    this.create(id, {
      type: 'doughnut',
      data: { labels: labels, datasets: [{ data: values, backgroundColor: PALETTE }] },
      options: { responsive: true, maintainAspectRatio: true, plugins: { legend: { labels: { color: '#94a3b8', padding: 12 } }, tooltip: { callbacks: { label: function(ctx) { var t = ctx.dataset.data.reduce(function(a,b){return a+b;},0); return ctx.label + ': ' + ((ctx.parsed/t)*100).toFixed(1) + '%'; } } } } }
    });
  },

  line: function(id, labels, values, label, fill) {
    this.create(id, {
      type: 'line',
      data: {
        labels: labels,
        datasets: [{
          label: label || '', data: values,
          borderColor: '#06b6d4', backgroundColor: fill !== false ? 'rgba(6,182,212,0.1)' : 'transparent',
          fill: fill !== false, tension: 0.3, pointRadius: 2, pointHoverRadius: 5
        }]
      },
      options: { responsive: true, maintainAspectRatio: true, plugins: { legend: { display: false } }, scales: { x: { grid: { display: false }, ticks: { color: '#94a3b8', maxTicksLimit: 12 } }, y: { grid: { color: 'rgba(148,163,184,0.1)' }, ticks: { color: '#94a3b8' } } } }
    });
  },

  latencyLine: function(id, labels, values) {
    this.create(id, {
      type: 'line',
      data: { labels: labels, datasets: [{ label: 'Avg Latency', data: values, borderColor: '#8b5cf6', backgroundColor: 'rgba(139,92,246,0.1)', fill: true, tension: 0.3, pointRadius: 3, pointHoverRadius: 6 }] },
      options: { responsive: true, maintainAspectRatio: true, plugins: { legend: { display: false }, tooltip: { callbacks: { label: function(ctx) { var ms = ctx.parsed.y; return ms < 1000 ? ms.toFixed(0) + 'ms' : (ms/1000).toFixed(1) + 's'; } } } }, scales: { x: { grid: { display: false }, ticks: { color: '#94a3b8', maxTicksLimit: 12 } }, y: { grid: { color: 'rgba(148,163,184,0.1)' }, ticks: { color: '#94a3b8', callback: function(v) { return v < 1000 ? v.toFixed(0) + 'ms' : (v/1000).toFixed(1) + 's'; } } } } }
    });
  },

  gauge: function(id, value, max, label) {
    var pct = max > 0 ? Math.min(value / max, 1) : 0;
    var color;
    if (pct < 0.5) color = '#10b981';
    else if (pct < 0.8) color = '#f59e0b';
    else color = '#ef4444';
    var disp = value < 0.01 ? value.toFixed(4) : value.toFixed(2);
    this.create(id, {
      type: 'doughnut',
      data: { datasets: [{ data: [value, Math.max(0, max - value)], backgroundColor: [color, 'rgba(148,163,184,0.15)'], borderWidth: 0 }] },
      options: { responsive: true, maintainAspectRatio: true, cutout: '80%', plugins: { legend: { display: false }, centerText: { text: ['$' + disp, label || 'Spend'] }, tooltip: { callbacks: { label: function() { return (label || 'Spend') + ': $' + disp; } } } } }
    });
  }
};

// ════════════════════════════════════════════════════════════════════════════
// View Loader — fetches SSE fragment and patches #content
// ════════════════════════════════════════════════════════════════════════════

// AbortController to cancel in-flight requests on rapid navigation
var currentAbort = null;
// Track the currently-loaded view so auto-refresh never jumps the user
// away from the section they're looking at.
var currentView = '/dashboard/overview';

function loadView(url) {
  if (!url) url = '/dashboard/overview';

  // Remember the view the user is on (before query-string params).
  currentView = url.split('?')[0];

  // Cancel any in-flight request to avoid racing patches
  if (currentAbort) { currentAbort.abort(); }
  currentAbort = new AbortController();

  fetch(url, {
    headers: { 'Accept': 'text/event-stream', 'Datastar-Request': 'true' },
    signal: currentAbort.signal
  })
    .then(function(res) { return res.text(); })
    .then(function(text) {
      var html = parseSseFragment(text);
      if (!html) return;
      var el = document.getElementById('content');
      if (!el) return;
      // The server wraps content in <div id="content"> but the layout uses
      // <main class="content">. Patch innerHTML instead of outerHTML so the
      // main element keeps its class + layout styles.
      var parsed = new DOMParser().parseFromString(html, 'text/html');
      var wrapper = parsed.getElementById('content');
      el.innerHTML = wrapper ? wrapper.innerHTML : html;
      runContentScripts();
    })
    .catch(function(e) {
      if (e.name === 'AbortError') return; // expected on rapid nav
      console.warn('dashboard: failed to load', url, e);
      // Show error state
      var content = document.getElementById('content');
      if (content && !document.querySelector('.kpi-grid')) {
        content.innerHTML = '<div class="empty-state"><div class="empty-icon">&#9888;</div>' +
          '<h2>Failed to load</h2><p class="empty-desc">Could not reach the dashboard server.</p>' +
          '<a href="#" onclick="location.reload()" class="retry-link">Reload</a></div>';
      }
    });
}

// Parse an SSE stream body for the first `data: elements <html>` fragment.
// Handles multi-line `data:` fields (the server splits long HTML across lines).
// Returns the full HTML content after the `elements ` prefix.
function parseSseFragment(text) {
  var lines = text.split('\n');
  var eventType = null;
  var dataParts = [];
  var result = null;

  function flush() {
    if (eventType === 'datastar-patch-elements' && dataParts.length) {
      // The server emits `elements ` prefix on EVERY data line (datastar-go quirk).
      // Strip it from each line, then join with newlines to reconstruct the HTML.
      var cleaned = dataParts.map(function(p) {
        return p.indexOf('elements ') === 0 ? p.slice('elements '.length) : p;
      }).join('\n');
      result = cleaned;
      return true;
    }
    eventType = null;
    dataParts = [];
    return false;
  }

  for (var i = 0; i < lines.length; i++) {
    var line = lines[i];

    if (line.startsWith('event: ')) {
      eventType = line.slice('event: '.length).trim();
      dataParts = [];
      continue;
    }

    if (eventType && line.startsWith('data: ')) {
      dataParts.push(line.slice('data: '.length));
      continue;
    }

    // Blank line ends the SSE event
    if (line === '') {
      if (flush()) return result;
    }
  }

  // Handle a trailing event with no terminating blank line
  if (flush()) return result;

  // Fallback: bare `data: elements <html>` with no event line
  for (var k = 0; k < lines.length; k++) {
    if (lines[k].startsWith('data: elements ')) {
      var collected = [lines[k].slice('data: elements '.length)];
      // Collect continuation data lines until a blank line
      for (var m = k + 1; m < lines.length && lines[m].startsWith('data: ') && lines[m] !== ''; m++) {
        var cont = lines[m].slice('data: '.length);
        collected.push(cont.indexOf('elements ') === 0 ? cont.slice('elements '.length) : cont);
      }
      return collected.join('\n');
    }
  }
  return null;
}

// Re-run any inline <script> tags in the newly-patched content
// (used for chart initialization emitted by the server).
function runContentScripts() {
  var newContent = document.getElementById('content');
  if (!newContent) return;
  var scripts = newContent.querySelectorAll('script');
  for (var i = 0; i < scripts.length; i++) {
    try { eval(scripts[i].textContent); } catch(e) { console.warn('dashboard: script error', e); }
  }
}

// ════════════════════════════════════════════════════════════════════════════
// Event delegation — handle data-on-click="$$get('/url')" on any element
// ════════════════════════════════════════════════════════════════════════════

document.addEventListener('click', function(e) {
  // Find the closest element with data-on-click
  var target = e.target.closest('[data-on-click]');
  if (!target) return;

  var expr = target.getAttribute('data-on-click');
  if (!expr) return;

  // Handle $$get('/url') — extract the URL argument
  // Also supports: $$set('offset',0);$$get('/url')
  var match = expr.match(/\$\$get\(\s*'([^']+)'\s*\)/);
  if (match) {
    e.preventDefault();
    loadView(match[1]);
    return;
  }

  // Handle plain openDetail('type','id') legacy calls
  var detailMatch = expr.match(/openDetail\(\s*'([^']+)'\s*,\s*'([^']+)'\s*\)/);
  if (detailMatch) {
    e.preventDefault();
    loadView('/dashboard/detail?type=' + encodeURIComponent(detailMatch[1]) + '&id=' + encodeURIComponent(detailMatch[2]));
    return;
  }
});

// ════════════════════════════════════════════════════════════════════════════
// Filter support — reloads timeline with typeFilter query param
// ════════════════════════════════════════════════════════════════════════════

// Expose a global function referenced by the filter <select> element
window.dashboardFilter = function(value) {
  loadView('/dashboard/timeline?typeFilter=' + encodeURIComponent(value));
};

// ════════════════════════════════════════════════════════════════════════════
// Initial load + auto-refresh
// ════════════════════════════════════════════════════════════════════════════

document.addEventListener('DOMContentLoaded', function() {
  setTimeout(function() { loadView('/dashboard/overview'); }, 50);
});

// Auto-refresh overview every 30s — but only when the user is actually ON
// the overview. Never yank them away from another section.
setInterval(function() {
  if (currentView === '/dashboard/overview') {
    loadView('/dashboard/overview');
  }
}, 30000);

// Refresh on visibility change — only refresh overview if user is on it.
document.addEventListener('visibilitychange', function() {
  if (!document.hidden && currentView === '/dashboard/overview') {
    loadView('/dashboard/overview');
  }
});
