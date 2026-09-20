/**
 * 文件用途：用于支撑 automation_tests 的Playwright 页面覆盖率目录与采集模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：覆盖率命中只证明执行或访问发生过，不能单独替代业务 oracle 和负向证据。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const fs = require('fs');
const path = require('path');
const writeJsonArtifact = require('./json_artifact');
const provenance = require('./coverage_provenance');

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
  { route: '/device/asset', module: 'device', name: 'Tenant asset management', priority: 'P1' },
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
  { route: '/automation/rule-chain', module: 'automation', name: 'Rule chain list', priority: 'P1' },
  { route: '/automation/rule-chain/edit', module: 'automation', name: 'Rule chain editor', priority: 'P1' },

  { route: '/product/pre-register', module: 'device', name: 'Device pre-register import', priority: 'P1' },


  { route: '/management/user', module: 'management', name: 'Tenant management', priority: 'P1' },
  { route: '/management/role', module: 'management', name: 'Role management', priority: 'P1' },
  { route: '/management/api', module: 'management', name: 'API management', priority: 'P1' },
  { route: '/management/auth', module: 'management', name: 'Permission management', priority: 'P1' },
  { route: '/management/notification', module: 'management', name: 'Notification config', priority: 'P1' },
  { route: '/management/entity-version', module: 'management', name: 'Entity version management', priority: 'P1' },
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
  { route: '/visualization/report', module: 'visualization', name: 'Scheduled reports', priority: 'P1' },

  // Legacy ThingsVis routes remain cataloged for optional compatibility builds.
  { route: '/visualization/thingsvis', module: 'visualization', name: 'ThingsVis project list', priority: 'P1' },
  { route: '/visualization/thingsvis-dashboards', module: 'visualization', name: 'ThingsVis dashboards', priority: 'P1' },
  { route: '/visualization/thingsvis-editor', module: 'visualization', name: 'ThingsVis editor', priority: 'P1' },
  { route: '/visualization/thingsvis-menu-dashboard', module: 'visualization', name: 'Menu dashboard', priority: 'P1' },
  { route: '/visualization/thingsvis-preview', module: 'visualization', name: 'ThingsVis preview', priority: 'P1' }
];

const BUSINESS_FLOWS = [
  {
    id: 'auth.password-login',
    module: 'auth',
    name: 'Password login reaches authenticated home',
    priority: 'P0',
    pages: ['/login', '/home'],
    requiredDimensions: ['userAction', 'response', 'visibleResult']
  },
  {
    id: 'device.create-readback-cleanup',
    module: 'device',
    name: 'Create, read back, display, and clean up a device',
    priority: 'P0',
    pages: ['/device/manage'],
    requiredDimensions: ['userAction', 'response', 'stateReadback', 'visibleResult', 'cleanup']
  },
  {
    id: 'native-board.create-publish',
    module: 'visualization',
    name: 'Create and publish a native dashboard',
    priority: 'P1',
    pages: ['/visualization/native-boards', '/visualization/native-board-editor'],
    requiredDimensions: ['userAction', 'response', 'stateReadback', 'visibleResult', 'cleanup']
  },
  {
    id: 'report.workspace.schedule-lifecycle',
    module: 'visualization',
    name: 'Create, edit, run, inspect, and clean up a scheduled report',
    priority: 'P1',
    pages: ['/visualization/report'],
    requiredDimensions: ['userAction', 'response', 'stateReadback', 'visibleResult', 'cleanup']
  },
  {
    id: 'csv.pre-register-import',
    module: 'device',
    name: 'Upload and verify device pre-registration CSV',
    priority: 'P0',
    pages: ['/product/pre-register'],
    requiredDimensions: ['userAction', 'response', 'stateReadback', 'visibleResult', 'cleanup']
  }
];

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
    this.hitBusinessFlows = new Map();
    this.events = [];
    // Playwright may restart a worker after a failed test or retry.  The
    // replacement worker receives the same PAGE_COVERAGE_FILE, so keep a
    // snapshot of what this process has already flushed and append only the
    // delta to the on-disk aggregate.  Without this, a later worker silently
    // replaces all routes collected by earlier workers.
    this.flushedPages = new Map();
    this.flushedBusinessFlows = new Map();
    this.totalPages = ALL_PAGES.length;
    this.totalBusinessFlows = BUSINESS_FLOWS.length;
    this.coverageFile = process.env.PAGE_COVERAGE_FILE || '';
    this.provenanceFile = process.env.COVERAGE_PROVENANCE_FILE ||
      provenance.provenanceFileForCoverage(this.coverageFile);
  }

  hitPage(route, name, observation = {}) {
    const normalizedRoute = this.normalizeRoute(route);
    const page = this.findPage(normalizedRoute);
    const key = page ? page.route : normalizedRoute;
    const event = provenance.createEvent({
      eventId: observation.eventId,
      runId: observation.runId,
      module: observation.module || (page && page.module) || 'unknown',
      kind: 'page',
      target: key,
      case: observation.case,
      attempt: observation.attempt,
      outcome: observation.outcome || 'pending',
      statusCode: observation.statusCode,
      disposition: observation.disposition || 'candidate',
      diagnostics: {
        catalogMatched: Boolean(page),
        observedURL: String(route || ''),
        ...(observation.diagnostics || {})
      }
    });
    this.events.push(event);

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

    this.flush();
    this.flushProvenance();
    return event;
  }

  hitBusinessFlow(flowId, dimensions = {}, flush = true) {
    const flow = BUSINESS_FLOWS.find(item => item.id === flowId);
    if (!flow) {
      throw new Error('Unknown browser business flow: ' + flowId);
    }
    const normalizedDimensions = Object.fromEntries(
      flow.requiredDimensions.map(dimension => [dimension, dimensions[dimension] === true])
    );
    const missingDimensions = flow.requiredDimensions.filter(
      dimension => normalizedDimensions[dimension] !== true
    );
    if (missingDimensions.length > 0) {
      throw new Error(
        'Browser business flow ' + flowId + ' is missing required dimensions: ' + missingDimensions.join(', ')
      );
    }
    if (!this.hitBusinessFlows.has(flowId)) {
      this.hitBusinessFlows.set(flowId, {
        count: 0,
        flow,
        dimensions: normalizedDimensions
      });
    }
    const hit = this.hitBusinessFlows.get(flowId);
    hit.count++;
    hit.dimensions = normalizedDimensions;
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
      businessFlows: Array.from(this.hitBusinessFlows.entries()).map(([key, value]) => ({
        key,
        count: value.count,
        flow: value.flow,
        dimensions: value.dimensions
      })),
      provenance: {
        schema: provenance.SCHEMA,
        events: provenance.mergeEvents(this.events)
      }
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

    (payload.businessFlows || []).forEach(item => {
      if (!item || !item.key || !item.flow) return;
      const current = this.hitBusinessFlows.get(item.key);
      if (current) {
        current.count += Number(item.count) || 0;
        current.dimensions = { ...current.dimensions, ...(item.dimensions || {}) };
      } else {
        this.hitBusinessFlows.set(item.key, {
          count: Number(item.count) || 0,
          flow: item.flow,
          dimensions: item.dimensions || {}
        });
      }
    });
    this.events = provenance.mergeEvents(
      this.events,
      payload.provenance && payload.provenance.events
    );
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

    let persisted = { pages: [], businessFlows: [] };
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

    const mergeDelta = (items, current, flushed, valueKey, extraKeys = []) => {
      const aggregate = new Map();
      (Array.isArray(items) ? items : []).forEach(item => {
        if (!item || !item.key || !item[valueKey]) return;
        aggregate.set(item.key, {
          count: Number(item.count) || 0,
          [valueKey]: item[valueKey],
          ...Object.fromEntries(extraKeys.map(key => [key, item[key]]))
        });
      });

      current.forEach((value, key) => {
        const previousCount = Number(flushed.get(key) || 0);
        const delta = (Number(value.count) || 0) - previousCount;
        if (delta <= 0) return;
        const extraValues = Object.fromEntries(extraKeys.map(extraKey => [extraKey, value[extraKey]]));
        const existing = aggregate.get(key);
        if (existing) {
          existing.count += delta;
          Object.assign(existing, extraValues);
        } else {
          aggregate.set(key, {
            count: delta,
            [valueKey]: value[valueKey],
            ...extraValues
          });
        }
      });

      return Array.from(aggregate.entries()).map(([key, value]) => ({
        key,
        count: value.count,
        [valueKey]: value[valueKey],
        ...Object.fromEntries(extraKeys.map(extraKey => [extraKey, value[extraKey]]))
      }));
    };

    const output = {
      pages: mergeDelta(persisted.pages, this.hitPages, this.flushedPages, 'page'),
      businessFlows: mergeDelta(
        persisted.businessFlows,
        this.hitBusinessFlows,
        this.flushedBusinessFlows,
        'flow',
        ['dimensions']
      ),
      provenance: {
        schema: provenance.SCHEMA,
        events: provenance.mergeEvents(
          persisted.provenance && persisted.provenance.events,
          this.events
        )
      }
    };

    writeJsonArtifact(this.coverageFile, output);
    this.flushedPages = new Map(
      Array.from(this.hitPages.entries()).map(([key, value]) => [key, Number(value.count) || 0])
    );
    this.flushedBusinessFlows = new Map(
      Array.from(this.hitBusinessFlows.entries()).map(([key, value]) => [key, Number(value.count) || 0])
    );
  }

  flushProvenance() {
    if (!this.provenanceFile) return;
    provenance.writeLedger(this.provenanceFile, this.events);
  }

  getProvenanceEvents() {
    return provenance.mergeEvents(this.events);
  }

  replaceProvenanceEvents(events) {
    this.events = provenance.mergeEvents(events);
    provenance.replaceLedger(this.provenanceFile, this.events);
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

    const coveredBusinessFlows = [];
    const uncoveredBusinessFlows = [];

    BUSINESS_FLOWS.forEach(flow => {
      if (this.hitBusinessFlows.has(flow.id)) {
        const hit = this.hitBusinessFlows.get(flow.id);
        coveredBusinessFlows.push({ ...flow, hitCount: hit.count, dimensions: hit.dimensions });
      } else {
        uncoveredBusinessFlows.push(flow);
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
      businessFlows: {
        total: this.totalBusinessFlows,
        covered: coveredBusinessFlows.length,
        uncovered: uncoveredBusinessFlows.length,
        rate: this.totalBusinessFlows > 0
          ? ((coveredBusinessFlows.length / this.totalBusinessFlows) * 100).toFixed(2)
          : '0.00',
        coveredList: coveredBusinessFlows,
        uncoveredList: uncoveredBusinessFlows
      },
      byModule
    };
  }

  report() {
    const stats = this.getStats();

    console.log('\n' + '='.repeat(70));
    console.log('  E2E route-render and business-flow coverage report');
    console.log('='.repeat(70));
    console.log('\n  Route renders: ' + stats.pages.covered + '/' + stats.pages.total + ' (' + stats.pages.rate + '%)');
    console.log('  Business flows: ' + stats.businessFlows.covered + '/' + stats.businessFlows.total + ' (' + stats.businessFlows.rate + '%)');

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

  writeReport(outputDir = './reports', interval = {}) {
    if (!fs.existsSync(outputDir)) {
      fs.mkdirSync(outputDir, { recursive: true });
    }

    const stats = {
      startedAt: interval.startedAt || null,
      finishedAt: interval.finishedAt || null,
      ...this.getStats()
    };
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

    const flowRows = stats.businessFlows.uncoveredList.map(flow => {
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
      '<h1>E2E route-render and business-flow coverage report</h1>' +
      '<p>Route renders: ' + stats.pages.covered + '/' + stats.pages.total + ' (' + stats.pages.rate + '%)</p>' +
      '<p>Business flows: ' + stats.businessFlows.covered + '/' + stats.businessFlows.total + ' (' + stats.businessFlows.rate + '%)</p>' +
      '<h2>By module</h2><table><thead><tr><th>Module</th><th>Total</th><th>Covered</th><th>Uncovered</th><th>Rate</th></tr></thead><tbody>' +
      moduleRows + '</tbody></table>' +
      '<h2>Uncovered pages</h2><table><thead><tr><th>Route</th><th>Name</th><th>Module</th><th>Priority</th></tr></thead><tbody>' +
      pageRows + '</tbody></table>' +
      '<h2>Uncovered business flows</h2><table><thead><tr><th>ID</th><th>Name</th><th>Module</th><th>Priority</th></tr></thead><tbody>' +
      flowRows + '</tbody></table>' +
      '</body></html>';
  }

  reset() {
    this.hitPages.clear();
    this.hitBusinessFlows.clear();
    this.events = [];
    this.flushedPages.clear();
    this.flushedBusinessFlows.clear();
    if (this.coverageFile) {
      writeJsonArtifact(this.coverageFile, {
        pages: [],
        businessFlows: [],
        provenance: { schema: provenance.SCHEMA, events: [] }
      });
    }
    provenance.replaceLedger(this.provenanceFile, []);
  }

  getCatalog() {
    return {
      pages: ALL_PAGES.map(page => ({ ...page })),
      businessFlows: BUSINESS_FLOWS.map(flow => ({
        ...flow,
        pages: [...flow.pages],
        requiredDimensions: [...flow.requiredDimensions]
      }))
    };
  }
}

const tracker = new PageCoverage();
tracker.ALL_PAGES = ALL_PAGES;
tracker.BUSINESS_FLOWS = BUSINESS_FLOWS;
tracker.ROUTE_ALIASES = ROUTE_ALIASES;
module.exports = tracker;
