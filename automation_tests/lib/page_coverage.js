/**
 * 文件用途：用于支撑 automation_tests 的Playwright 页面覆盖率目录与采集模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：覆盖率命中只证明执行或访问发生过，不能单独替代业务 oracle 和负向证据。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const fs = require('fs');
const path = require('path');
const writeJsonArtifact = require('./json_artifact');

const ALL_PAGES = [
  { route: '/login', module: 'auth', name: 'Login', priority: 'P0' },
  { route: '/login/pwd-login', module: 'auth', name: 'Password login', priority: 'P1' },
  { route: '/login/register', module: 'auth', name: 'Register', priority: 'P1' },
  { route: '/login/reset-pwd', module: 'auth', name: 'Reset password', priority: 'P1' },

  { route: '/home', module: 'home', name: 'Home', priority: 'P0' },

  { route: '/dashboard', module: 'dashboard', name: 'Dashboard redirect', priority: 'P1' },
  { route: '/dashboard/workspace', module: 'dashboard', name: 'Visualization workspace', priority: 'P1' },
  { route: '/dashboard/rdi-overview', module: 'dashboard', name: 'RDI overview dashboard', priority: 'P0' },
  { route: '/dashboard/workbench', module: 'dashboard', name: 'Workbench', priority: 'P1' },

  { route: '/device/manage', module: 'device', name: 'Device management', priority: 'P0' },
  { route: '/device/command-center', module: 'device', name: 'Command center', priority: 'P0' },
  { route: '/device/template', module: 'device', name: 'Device template', priority: 'P1' },
  { route: '/device/config-detail', module: 'device', name: 'Device config detail', priority: 'P1' },
  { route: '/device/config-edit', module: 'device', name: 'Device config edit', priority: 'P1' },
  { route: '/device/details', module: 'device', name: 'Device details', priority: 'P0' },
  { route: '/device/details-child', module: 'device', name: 'Child device details', priority: 'P1' },
  { route: '/device/grouping', module: 'device', name: 'Device grouping', priority: 'P1' },
  { route: '/device/grouping-details', module: 'device', name: 'Device grouping details', priority: 'P1' },
  { route: '/device/service-access', module: 'device', name: 'Service access', priority: 'P1' },
  { route: '/device/service-details', module: 'device', name: 'Service details', priority: 'P1' },
  { route: '/device/share', module: 'device', name: 'Device share', priority: 'P1' },
  { route: '/device/shared-with-me', module: 'device', name: 'Shared with me', priority: 'P1' },
  { route: '/device/thingsmodel', module: 'device', name: 'Things model', priority: 'P0' },

  { route: '/alarm/notification-group', module: 'alarm', name: 'Notification group', priority: 'P1' },
  { route: '/alarm/notification-record', module: 'alarm', name: 'Notification record', priority: 'P1' },
  { route: '/alarm/rdi-overview', module: 'alarm', name: 'RDI alarm overview', priority: 'P0' },
  { route: '/alarm/warning-message', module: 'alarm', name: 'Warning messages', priority: 'P0' },

  { route: '/apply/plugin', module: 'apply', name: 'Apply plugin marketplace', priority: 'P1' },
  { route: '/apply/service', module: 'apply', name: 'Apply service marketplace', priority: 'P1' },

  { route: '/automation/scene-manage', module: 'automation', name: 'Scene management', priority: 'P1' },
  { route: '/automation/scene-edit', module: 'automation', name: 'Scene editor', priority: 'P1' },
  { route: '/automation/scene-linkage', module: 'automation', name: 'Scene linkage', priority: 'P1' },
  { route: '/automation/linkage-edit', module: 'automation', name: 'Linkage editor', priority: 'P1' },


  { route: '/management/user', module: 'management', name: 'Tenant management', priority: 'P1' },
  { route: '/management/role', module: 'management', name: 'Role management', priority: 'P1' },
  { route: '/management/api', module: 'management', name: 'API management', priority: 'P1' },
  { route: '/management/auth', module: 'management', name: 'Permission management', priority: 'P1' },
  { route: '/management/notification', module: 'management', name: 'Notification config', priority: 'P1' },
  { route: '/management/setting', module: 'management', name: 'System setting', priority: 'P1' },
  { route: '/product/update-ota', module: 'product', name: 'OTA update', priority: 'P1' },
  { route: '/product/update-package', module: 'product', name: 'Update package', priority: 'P1' },


  { route: '/system-management-user/system-log', module: 'system', name: 'System log', priority: 'P2' },
  { route: '/system-management-user/equipment-map', module: 'system', name: 'Equipment map', priority: 'P2' },

  { route: '/403', module: 'exception', name: 'Forbidden page', priority: 'P2' },
  { route: '/404', module: 'exception', name: 'Not found page', priority: 'P2' },
  { route: '/500', module: 'exception', name: 'Server error page', priority: 'P2' },
  { route: '/device-details-app', module: 'device', name: 'Standalone device details app', priority: 'P1' },
  { route: '/personal-center', module: 'user', name: 'Personal center', priority: 'P2' },

  { route: '/visualization/native-boards', module: 'visualization', name: 'Native dashboards', priority: 'P1' },
  { route: '/visualization/native-board', module: 'visualization', name: 'Native dashboard', priority: 'P1' },
  { route: '/visualization/native-board-editor', module: 'visualization', name: 'Native dashboard editor', priority: 'P1' },

  // Legacy ThingsVis routes remain cataloged for optional compatibility builds.
  { route: '/visualization/thingsvis', module: 'visualization', name: 'ThingsVis project list', priority: 'P1' },
  { route: '/visualization/thingsvis-dashboards', module: 'visualization', name: 'ThingsVis dashboards', priority: 'P1' },
  { route: '/visualization/thingsvis-editor', module: 'visualization', name: 'ThingsVis editor', priority: 'P1' },
  { route: '/visualization/thingsvis-menu-dashboard', module: 'visualization', name: 'Menu dashboard', priority: 'P1' },
  { route: '/visualization/thingsvis-preview', module: 'visualization', name: 'ThingsVis preview', priority: 'P1' }
];

const ALL_FLOWS = ALL_PAGES.map(page => ({
  id: 'route:' + page.route,
  module: page.module,
  name: page.name + ' route renders',
  priority: page.priority,
  pages: [page.route]
}));

// `/tv-preview` is the standalone constant route for the same ThingsVis
// preview component exposed by the generated `/visualization/thingsvis-preview`
// route. Keep one canonical coverage bucket so a real preview execution cannot
// disappear as an unknown page merely because it used the standalone entry.
const ROUTE_ALIASES = new Map([
  ['/tv-preview', '/visualization/thingsvis-preview']
]);

class PageCoverage {
  constructor() {
    this.hitPages = new Map();
    this.hitFlows = new Map();
    // Playwright may restart a worker after a failed test or retry.  The
    // replacement worker receives the same PAGE_COVERAGE_FILE, so keep a
    // snapshot of what this process has already flushed and append only the
    // delta to the on-disk aggregate.  Without this, a later worker silently
    // replaces all routes collected by earlier workers.
    this.flushedPages = new Map();
    this.flushedFlows = new Map();
    this.totalPages = ALL_PAGES.length;
    this.totalFlows = ALL_FLOWS.length;
    this.coverageFile = process.env.PAGE_COVERAGE_FILE || '';
  }

  hitPage(route, name) {
    const normalizedRoute = this.normalizeRoute(route);
    const page = this.findPage(normalizedRoute);
    const key = page ? page.route : normalizedRoute;

    if (!this.hitPages.has(key)) {
      this.hitPages.set(key, {
        count: 0,
        page: page || {
          route: normalizedRoute,
          name: name || normalizedRoute,
          module: 'unknown',
          priority: '?'
        }
      });
    }
    this.hitPages.get(key).count++;

    ALL_FLOWS
      .filter(flow => flow.pages.some(flowPage => this.routeMatches(flowPage, key)))
      .forEach(flow => this.hitFlow(flow.id, false));

    this.flush();
  }

  hitFlow(flowId, flush = true) {
    const flow = ALL_FLOWS.find(f => f.id === flowId);
    if (!this.hitFlows.has(flowId)) {
      this.hitFlows.set(flowId, {
        count: 0,
        flow: flow || { id: flowId, name: flowId, module: 'unknown', priority: '?', pages: [] }
      });
    }
    this.hitFlows.get(flowId).count++;
    if (flush) this.flush();
  }

  normalizeRoute(route) {
    let r = String(route || '/');
    try {
      r = new URL(r, 'http://local').pathname;
    } catch (err) {
      r = r.split('?')[0];
    }
    r = r.replace(/\/+$/g, '') || '/';
    r = ROUTE_ALIASES.get(r) || r;
    // Exception pages use numeric-looking static paths. Preserve any exact
    // catalog route before normalizing numeric resource IDs such as /123.
    if (ALL_PAGES.some(page => page.route === r)) {
      return r;
    }
    r = r.replace(/\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi, '/:id');
    r = r.replace(/\/\d+/g, '/:id');
    return r;
  }

  routeMatches(template, actual) {
    const tParts = template.split('/');
    const aParts = actual.split('/');
    if (tParts.length !== aParts.length) return false;
    for (let i = 0; i < tParts.length; i++) {
      if (tParts[i].startsWith(':')) continue;
      if (tParts[i] !== aParts[i]) return false;
    }
    return true;
  }

  findPage(normalizedRoute) {
    return ALL_PAGES.find(page => this.routeMatches(page.route, normalizedRoute)) || null;
  }

  toJSON() {
    return {
      pages: Array.from(this.hitPages.entries()).map(([key, value]) => ({
        key,
        count: value.count,
        page: value.page
      })),
      flows: Array.from(this.hitFlows.entries()).map(([key, value]) => ({
        key,
        count: value.count,
        flow: value.flow
      }))
    };
  }

  merge(payload) {
    if (!payload || typeof payload !== 'object') return;

    (payload.pages || []).forEach(item => {
      if (!item || !item.key || !item.page) return;
      const current = this.hitPages.get(item.key);
      if (current) {
        current.count += Number(item.count) || 0;
      } else {
        this.hitPages.set(item.key, {
          count: Number(item.count) || 0,
          page: item.page
        });
      }
    });

    (payload.flows || []).forEach(item => {
      if (!item || !item.key || !item.flow) return;
      const current = this.hitFlows.get(item.key);
      if (current) {
        current.count += Number(item.count) || 0;
      } else {
        this.hitFlows.set(item.key, {
          count: Number(item.count) || 0,
          flow: item.flow
        });
      }
    });
  }

  mergeFromFile(filePath) {
    if (!filePath || !fs.existsSync(filePath)) {
      return false;
    }

    try {
      this.merge(JSON.parse(fs.readFileSync(filePath, 'utf8')));
      return true;
    } catch (err) {
      console.warn('  Failed to read E2E page coverage temp file: ' + filePath + ' (' + err.message + ')');
      return false;
    }
  }

  flush() {
    if (!this.coverageFile) {
      return;
    }

    let persisted = { pages: [], flows: [] };
    if (fs.existsSync(this.coverageFile)) {
      try {
        const candidate = JSON.parse(fs.readFileSync(this.coverageFile, 'utf8'));
        if (candidate && typeof candidate === 'object') persisted = candidate;
      } catch (err) {
        // A truncated artifact should not make the browser run fail.  The
        // current process snapshot remains valid and will replace the bad
        // payload below.
      }
    }

    const mergeDelta = (items, current, flushed, valueKey) => {
      const aggregate = new Map();
      (Array.isArray(items) ? items : []).forEach(item => {
        if (!item || !item.key || !item[valueKey]) return;
        aggregate.set(item.key, {
          count: Number(item.count) || 0,
          [valueKey]: item[valueKey]
        });
      });

      current.forEach((value, key) => {
        const previousCount = Number(flushed.get(key) || 0);
        const delta = (Number(value.count) || 0) - previousCount;
        if (delta <= 0) return;
        const existing = aggregate.get(key);
        if (existing) {
          existing.count += delta;
        } else {
          aggregate.set(key, {
            count: delta,
            [valueKey]: value[valueKey]
          });
        }
      });

      return Array.from(aggregate.entries()).map(([key, value]) => ({
        key,
        count: value.count,
        [valueKey]: value[valueKey]
      }));
    };

    const output = {
      pages: mergeDelta(persisted.pages, this.hitPages, this.flushedPages, 'page'),
      flows: mergeDelta(persisted.flows, this.hitFlows, this.flushedFlows, 'flow')
    };

    writeJsonArtifact(this.coverageFile, output);
    this.flushedPages = new Map(
      Array.from(this.hitPages.entries()).map(([key, value]) => [key, Number(value.count) || 0])
    );
    this.flushedFlows = new Map(
      Array.from(this.hitFlows.entries()).map(([key, value]) => [key, Number(value.count) || 0])
    );
  }

  getStats() {
    const coveredPages = [];
    const uncoveredPages = [];

    ALL_PAGES.forEach(page => {
      if (this.hitPages.has(page.route)) {
        coveredPages.push({ ...page, hitCount: this.hitPages.get(page.route).count });
      } else {
        uncoveredPages.push(page);
      }
    });

    const coveredFlows = [];
    const uncoveredFlows = [];

    ALL_FLOWS.forEach(flow => {
      if (this.hitFlows.has(flow.id)) {
        coveredFlows.push({ ...flow, hitCount: this.hitFlows.get(flow.id).count });
      } else {
        uncoveredFlows.push(flow);
      }
    });

    const byModule = {};
    ALL_PAGES.forEach(page => {
      if (!byModule[page.module]) byModule[page.module] = { total: 0, covered: 0, pages: [] };
      byModule[page.module].total++;
      if (this.hitPages.has(page.route)) {
        byModule[page.module].covered++;
        byModule[page.module].pages.push({ ...page, hitCount: this.hitPages.get(page.route).count });
      } else {
        byModule[page.module].pages.push({ ...page, hitCount: 0 });
      }
    });

    return {
      pages: {
        total: this.totalPages,
        covered: coveredPages.length,
        uncovered: uncoveredPages.length,
        rate: this.totalPages > 0 ? ((coveredPages.length / this.totalPages) * 100).toFixed(2) : '0.00',
        coveredList: coveredPages,
        uncoveredList: uncoveredPages
      },
      flows: {
        total: this.totalFlows,
        covered: coveredFlows.length,
        uncovered: uncoveredFlows.length,
        rate: this.totalFlows > 0 ? ((coveredFlows.length / this.totalFlows) * 100).toFixed(2) : '0.00',
        coveredList: coveredFlows,
        uncoveredList: uncoveredFlows
      },
      byModule
    };
  }

  report() {
    const stats = this.getStats();

    console.log('\n' + '='.repeat(70));
    console.log('  E2E page and route-flow coverage report');
    console.log('='.repeat(70));
    console.log('\n  Pages: ' + stats.pages.covered + '/' + stats.pages.total + ' (' + stats.pages.rate + '%)');
    console.log('  Route flows: ' + stats.flows.covered + '/' + stats.flows.total + ' (' + stats.flows.rate + '%)');

    Object.keys(stats.byModule).sort().forEach(moduleName => {
      const moduleStats = stats.byModule[moduleName];
      const rate = moduleStats.total > 0
        ? ((moduleStats.covered / moduleStats.total) * 100).toFixed(1)
        : '0.0';
      console.log('    ' + moduleName.padEnd(14) + ' ' + rate + '% (' + moduleStats.covered + '/' + moduleStats.total + ')');
    });

    if (stats.pages.uncovered > 0) {
      console.log('\n  Uncovered pages:');
      stats.pages.uncoveredList.forEach(page => {
        console.log('    [' + page.priority + '] ' + page.route + ' (' + page.module + ')');
      });
    }

    console.log('='.repeat(70) + '\n');
    return stats;
  }

  writeReport(outputDir = './reports') {
    if (!fs.existsSync(outputDir)) {
      fs.mkdirSync(outputDir, { recursive: true });
    }

    const stats = this.getStats();
    const jsonPath = path.join(outputDir, 'page-coverage.json');
    fs.writeFileSync(jsonPath, JSON.stringify(stats, null, 2), 'utf8');

    const htmlPath = path.join(outputDir, 'page-coverage.html');
    fs.writeFileSync(htmlPath, this.buildHtmlReport(stats), 'utf8');

    console.log('  Page coverage reports generated:');
    console.log('    JSON: ' + path.resolve(jsonPath));
    console.log('    HTML: ' + path.resolve(htmlPath));

    return jsonPath;
  }

  buildHtmlReport(stats) {
    const moduleRows = Object.keys(stats.byModule).sort().map(moduleName => {
      const moduleStats = stats.byModule[moduleName];
      const rate = moduleStats.total > 0
        ? ((moduleStats.covered / moduleStats.total) * 100).toFixed(1)
        : '0.0';
      return '<tr><td>' + moduleName + '</td><td>' + moduleStats.total + '</td><td>' +
        moduleStats.covered + '</td><td>' + (moduleStats.total - moduleStats.covered) +
        '</td><td>' + rate + '%</td></tr>';
    }).join('');

    const pageRows = stats.pages.uncoveredList.map(page => {
      return '<tr><td>' + page.route + '</td><td>' + page.name + '</td><td>' +
        page.module + '</td><td>' + page.priority + '</td></tr>';
    }).join('');

    const flowRows = stats.flows.uncoveredList.map(flow => {
      return '<tr><td>' + flow.id + '</td><td>' + flow.name + '</td><td>' +
        flow.module + '</td><td>' + flow.priority + '</td></tr>';
    }).join('');

    return '<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8">' +
      '<title>E2E coverage report</title>' +
      '<style>body{font-family:Arial,sans-serif;margin:20px;background:#f5f7fa;color:#333;}' +
      'h1{color:#1a73e8;}h2{border-left:4px solid #1a73e8;padding-left:10px;margin-top:30px;}' +
      'table{width:100%;border-collapse:collapse;background:#fff;margin-bottom:20px;}' +
      'th,td{padding:10px 12px;border-bottom:1px solid #eee;text-align:left;font-size:14px;}' +
      'th{background:#1a73e8;color:#fff;}</style></head><body>' +
      '<h1>E2E page and route-flow coverage report</h1>' +
      '<p>Pages: ' + stats.pages.covered + '/' + stats.pages.total + ' (' + stats.pages.rate + '%)</p>' +
      '<p>Route flows: ' + stats.flows.covered + '/' + stats.flows.total + ' (' + stats.flows.rate + '%)</p>' +
      '<h2>By module</h2><table><thead><tr><th>Module</th><th>Total</th><th>Covered</th><th>Uncovered</th><th>Rate</th></tr></thead><tbody>' +
      moduleRows + '</tbody></table>' +
      '<h2>Uncovered pages</h2><table><thead><tr><th>Route</th><th>Name</th><th>Module</th><th>Priority</th></tr></thead><tbody>' +
      pageRows + '</tbody></table>' +
      '<h2>Uncovered route flows</h2><table><thead><tr><th>ID</th><th>Name</th><th>Module</th><th>Priority</th></tr></thead><tbody>' +
      flowRows + '</tbody></table>' +
      '</body></html>';
  }

  reset() {
    this.hitPages.clear();
    this.hitFlows.clear();
    this.flushedPages.clear();
    this.flushedFlows.clear();
    if (this.coverageFile) {
      writeJsonArtifact(this.coverageFile, { pages: [], flows: [] });
    }
  }

  getCatalog() {
    return {
      pages: ALL_PAGES.map(page => ({ ...page })),
      flows: ALL_FLOWS.map(flow => ({ ...flow, pages: [...flow.pages] }))
    };
  }
}

const tracker = new PageCoverage();
tracker.ALL_PAGES = ALL_PAGES;
tracker.ALL_FLOWS = ALL_FLOWS;
tracker.ROUTE_ALIASES = ROUTE_ALIASES;
module.exports = tracker;
