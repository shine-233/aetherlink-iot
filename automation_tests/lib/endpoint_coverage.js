/**
 * 文件用途：用于支撑 automation_tests 的API 端点覆盖率采集模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：覆盖率命中只证明执行或访问发生过，不能单独替代业务 oracle 和负向证据。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const fs = require('fs');
const path = require('path');
const writeJsonArtifact = require('./json_artifact');
const { ALL_ENDPOINTS } = require('./endpoint-coverage/catalog');

/**
 * 将实际请求路径归一化为路由模板格式
 * 例如: /api/v1/device/detail/abc123 -> /api/v1/device/detail/:id
 * @param {string} url - 实际请求 URL
 * @returns {string} 归一化后的路由模板
 */
function normalizePath(url) {
  // 去掉 query string
  let p = url.split('?')[0];
  // 去掉 baseURL 前缀
  p = p.replace(/^https?:\/\/[^/]+/, '');
  // 将 UUID / 数字 ID 替换为 :id / :device_id 等占位符
  // 匹配规则: 路径段为纯数字、UUID、或长十六进制串
  p = p.replace(/\/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?=\/|$)/gi, '/:id');
  p = p.replace(/\/(\d{10,})(?=\/|$)/g, '/:id');
  p = p.replace(/\/([a-f0-9]{16,})(?=\/|$)/gi, '/:id');
  // 短数字段也替换为 :id（但保留已知关键字如 v1, api）
  const reserved = new Set(['api', 'v1', 'v2', 'http', 'ws', 'wss']);
  p = p.replace(/\/(\d+)(?=\/|$)/g, (match, num) => {
    return reserved.has(num) ? match : '/:id';
  });
  return p;
}

class EndpointCoverage {
  constructor() {
    this.hitSet = new Map(); // key: "METHOD /path" -> { count, endpoint }
    this.totalEndpoints = ALL_ENDPOINTS.length;
    this.coverageFile = process.env.ENDPOINT_COVERAGE_FILE || '';
  }

  /**
   * 记录一次请求命中
   * @param {string} method - HTTP 方法 (GET/POST/PUT/DELETE)
   * @param {string} url - 实际请求 URL
   */
  hit(method, url) {
    const normalizedPath = normalizePath(url);
    const key = method.toUpperCase() + ' ' + normalizedPath;

    // 查找匹配的端点定义
    const matched = this.findEndpoint(method.toUpperCase(), normalizedPath);
    if (matched) {
      const endpointKey = matched.method + ' ' + matched.path;
      if (!this.hitSet.has(endpointKey)) {
        this.hitSet.set(endpointKey, { count: 0, endpoint: matched });
      }
      this.hitSet.get(endpointKey).count++;
    } else {
      // 未在清单中找到的端点，也记录下来（可能清单不完整）
      if (!this.hitSet.has(key)) {
        this.hitSet.set(key, { count: 0, endpoint: { method: method.toUpperCase(), path: normalizedPath, module: 'unknown', auth: null } });
      }
      this.hitSet.get(key).count++;
    }

    this.flush();
  }

  /**
   * 将当前命中数据序列化为普通对象，便于跨进程传递
   * @returns {{ hits: Array<{ key: string, count: number, endpoint: object }> }}
   */
  toJSON() {
    return {
      hits: Array.from(this.hitSet.entries()).map(([key, value]) => ({
        key,
        count: value.count,
        endpoint: value.endpoint
      }))
    };
  }

  /**
   * 合并序列化后的命中数据
   * @param {{ hits?: Array<{ key: string, count: number, endpoint: object }> }} payload
   */
  merge(payload) {
    if (!payload || !Array.isArray(payload.hits)) {
      return;
    }

    payload.hits.forEach(item => {
      if (!item || !item.key || !item.endpoint) {
        return;
      }

      const current = this.hitSet.get(item.key);
      if (current) {
        current.count += Number(item.count) || 0;
      } else {
        this.hitSet.set(item.key, {
          count: Number(item.count) || 0,
          endpoint: item.endpoint
        });
      }
    });
  }

  /**
   * 从文件合并其它进程写入的命中数据
   * @param {string} filePath
   * @returns {boolean}
   */
  mergeFromFile(filePath) {
    if (!filePath || !fs.existsSync(filePath)) {
      return false;
    }

    try {
      const payload = JSON.parse(fs.readFileSync(filePath, 'utf8'));
      this.merge(payload);
      return true;
    } catch (err) {
      console.warn('  读取端点覆盖率中间文件失败: ' + filePath + ' (' + err.message + ')');
      return false;
    }
  }

  /**
   * 将当前进程命中数据落盘，供父进程汇总
   */
  flush() {
    if (!this.coverageFile) {
      return;
    }

    writeJsonArtifact(this.coverageFile, this.toJSON());
  }

  /**
   * 在端点清单中查找匹配项
   * @param {string} method
   * @param {string} normalizedPath
   * @returns {object|null}
   */
  findEndpoint(method, normalizedPath) {
    const candidates = ALL_ENDPOINTS
      .filter(ep => ep.method === method && this.pathMatches(ep.path, normalizedPath))
      .map(ep => ({ endpoint: ep, score: this.matchScore(ep.path, normalizedPath) }))
      .sort((left, right) => {
        if (right.score.total !== left.score.total) {
          return right.score.total - left.score.total;
        }
        if (right.score.staticCount !== left.score.staticCount) {
          return right.score.staticCount - left.score.staticCount;
        }
        if (right.score.segmentCount !== left.score.segmentCount) {
          return right.score.segmentCount - left.score.segmentCount;
        }
        if (left.score.wildcardCount !== right.score.wildcardCount) {
          return left.score.wildcardCount - right.score.wildcardCount;
        }
        return 0;
      });

    return candidates.length > 0 ? candidates[0].endpoint : null;
  }

  /**
   * 判断路由模板是否匹配归一化后的路径
   * @param {string} template - 路由模板 (如 /api/v1/device/detail/:id)
   * @param {string} actual - 归一化后的实际路径 (如 /api/v1/device/detail/:id)
   */
  pathMatches(template, actual) {
    const tParts = template.split('/');
    const aParts = actual.split('/');
    const wildcardIndex = tParts.findIndex(part => part.startsWith('*'));
    if (wildcardIndex >= 0) {
      if (aParts.length < wildcardIndex) return false;
    } else if (tParts.length !== aParts.length) {
      return false;
    }

    for (let i = 0; i < tParts.length; i++) {
      if (tParts[i].startsWith('*')) return true;
      if (tParts[i].startsWith(':')) continue; // 参数段
      if (tParts[i] !== aParts[i]) return false;
    }
    return true;
  }

  /**
   * 计算匹配优先级，确保静态路由优先于参数路由，参数路由优先于通配路由
   * @param {string} template
   * @param {string} actual
   * @returns {{ total: number, staticCount: number, segmentCount: number, wildcardCount: number }}
   */
  matchScore(template, actual) {
    const tParts = template.split('/');
    const aParts = actual.split('/');
    let total = 0;
    let staticCount = 0;
    let wildcardCount = 0;

    for (let i = 0; i < tParts.length; i++) {
      const tPart = tParts[i];
      const aPart = aParts[i];

      if (tPart.startsWith('*')) {
        wildcardCount++;
        total += 1;
        break;
      }

      if (tPart.startsWith(':')) {
        total += 10;
        continue;
      }

      if (tPart === aPart) {
        staticCount++;
        total += 100;
      }
    }

    return {
      total,
      staticCount,
      segmentCount: tParts.length,
      wildcardCount
    };
  }

  /**
   * 获取覆盖率统计
   * @returns {{ total, covered, uncovered, rate, byModule, uncoveredList }}
   */
  getStats() {
    const covered = [];
    const uncovered = [];

    ALL_ENDPOINTS.forEach(ep => {
      const key = ep.method + ' ' + ep.path;
      if (this.hitSet.has(key)) {
        covered.push({ ...ep, hitCount: this.hitSet.get(key).count });
      } else {
        uncovered.push(ep);
      }
    });

    // 按模块分组
    const byModule = {};
    ALL_ENDPOINTS.forEach(ep => {
      if (!byModule[ep.module]) {
        byModule[ep.module] = { total: 0, covered: 0, endpoints: [] };
      }
      byModule[ep.module].total++;
      const key = ep.method + ' ' + ep.path;
      if (this.hitSet.has(key)) {
        byModule[ep.module].covered++;
        byModule[ep.module].endpoints.push({ ...ep, hitCount: this.hitSet.get(key).count });
      } else {
        byModule[ep.module].endpoints.push({ ...ep, hitCount: 0 });
      }
    });

    const rate = this.totalEndpoints > 0
      ? ((covered.length / this.totalEndpoints) * 100).toFixed(2)
      : '0.00';

    return {
      total: this.totalEndpoints,
      covered: covered.length,
      uncovered: uncovered.length,
      rate,
      byModule,
      coveredList: covered,
      uncoveredList: uncovered
    };
  }

  /**
   * 控制台输出覆盖率报告
   */
  report() {
    const stats = this.getStats();

    console.log('\n' + '='.repeat(70));
    console.log('  API 端点覆盖率报告');
    console.log('='.repeat(70));
    console.log('  总端点数: ' + stats.total);
    console.log('  已覆盖:   ' + stats.covered + '  (' + stats.rate + '%)');
    console.log('  未覆盖:   ' + stats.uncovered);
    console.log('  ───────────────────────────────');

    // 按模块输出
    const modules = Object.keys(stats.byModule).sort((a, b) => {
      const rateA = stats.byModule[a].covered / stats.byModule[a].total;
      const rateB = stats.byModule[b].covered / stats.byModule[b].total;
      return rateA - rateB;
    });

    modules.forEach(mod => {
      const m = stats.byModule[mod];
      const mRate = m.total > 0 ? ((m.covered / m.total) * 100).toFixed(1) : '0.0';
      const bar = this.progressBar(m.covered, m.total, 20);
      console.log('  ' + mod.padEnd(15) + ' ' + bar + ' ' + mRate + '% (' + m.covered + '/' + m.total + ')');
    });

    if (stats.uncovered > 0) {
      console.log('\n  未覆盖端点:');
      stats.uncoveredList.forEach(ep => {
        console.log('    ' + ep.method.padEnd(7) + ' ' + ep.path + '  [' + ep.module + ']');
      });
    }

    console.log('='.repeat(70) + '\n');
    return stats;
  }

  /**
   * 生成进度条
   */
  progressBar(filled, total, width) {
    const ratio = total > 0 ? filled / total : 0;
    const filledWidth = Math.round(ratio * width);
    const emptyWidth = width - filledWidth;
    return '[' + '#'.repeat(filledWidth) + '-'.repeat(emptyWidth) + ']';
  }

  /**
   * 写入覆盖率报告文件
   * @param {string} outputDir - 输出目录
   */
  writeReport(outputDir = './reports') {
    if (!fs.existsSync(outputDir)) {
      fs.mkdirSync(outputDir, { recursive: true });
    }

    const stats = this.getStats();

    // JSON 报告
    const jsonPath = path.join(outputDir, 'endpoint-coverage.json');
    fs.writeFileSync(jsonPath, JSON.stringify(stats, null, 2), 'utf8');

    // HTML 报告
    const html = this.buildHtmlReport(stats);
    const htmlPath = path.join(outputDir, 'endpoint-coverage.html');
    fs.writeFileSync(htmlPath, html, 'utf8');

    console.log('  端点覆盖率报告已生成:');
    console.log('    JSON: ' + path.resolve(jsonPath));
    console.log('    HTML: ' + path.resolve(htmlPath));

    return jsonPath;
  }

  /**
   * 构建 HTML 覆盖率报告
   */
  buildHtmlReport(stats) {
    const moduleRows = Object.keys(stats.byModule).sort().map(mod => {
      const m = stats.byModule[mod];
      const mRate = m.total > 0 ? ((m.covered / m.total) * 100).toFixed(1) : '0.0';
      const color = mRate >= 80 ? '#34a853' : mRate >= 50 ? '#fbbc04' : '#ea4335';
      return '<tr>' +
        '<td>' + mod + '</td>' +
        '<td>' + m.total + '</td>' +
        '<td>' + m.covered + '</td>' +
        '<td>' + (m.total - m.covered) + '</td>' +
        '<td style="color:' + color + ';font-weight:bold;">' + mRate + '%</td>' +
        '</tr>';
    }).join('');

    const endpointRows = stats.uncoveredList.map(ep => {
      return '<tr>' +
        '<td>' + ep.method + '</td>' +
        '<td>' + ep.path + '</td>' +
        '<td>' + ep.module + '</td>' +
        '<td>' + (ep.auth ? '是' : '否') + '</td>' +
        '</tr>';
    }).join('');

    return '<!DOCTYPE html><html lang="zh-CN"><head><meta charset="UTF-8">' +
      '<title>API 端点覆盖率报告</title>' +
      '<style>' +
      'body{font-family:"Microsoft YaHei",Arial,sans-serif;margin:20px;background:#f5f7fa;color:#333;}' +
      'h1{color:#1a73e8;}h2{border-left:4px solid #1a73e8;padding-left:10px;margin-top:30px;}' +
      '.summary{display:flex;gap:15px;flex-wrap:wrap;margin:20px 0;}' +
      '.card{background:#fff;border-radius:8px;padding:15px 20px;box-shadow:0 2px 6px rgba(0,0,0,.08);min-width:180px;}' +
      '.card-title{font-size:16px;font-weight:bold;margin-bottom:8px;color:#1a73e8;}' +
      '.card-stat{font-size:14px;margin:4px 0;}' +
      '.pass-text{color:#34a853;}.fail-text{color:#ea4335;}' +
      'table{width:100%;border-collapse:collapse;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 6px rgba(0,0,0,.08);margin-bottom:20px;}' +
      'th,td{padding:10px 12px;border-bottom:1px solid #eee;text-align:left;font-size:14px;}' +
      'th{background:#1a73e8;color:#fff;}' +
      '</style></head><body>' +
      '<h1>API 端点覆盖率报告</h1>' +
      '<div class="summary">' +
      '<div class="card"><div class="card-title">总端点</div><div class="card-stat">' + stats.total + '</div></div>' +
      '<div class="card"><div class="card-title">已覆盖</div><div class="card-stat pass-text">' + stats.covered + '</div></div>' +
      '<div class="card"><div class="card-title">未覆盖</div><div class="card-stat fail-text">' + stats.uncovered + '</div></div>' +
      '<div class="card"><div class="card-title">覆盖率</div><div class="card-stat" style="font-size:24px;font-weight:bold;">' + stats.rate + '%</div></div>' +
      '</div>' +
      '<h2>按模块统计</h2>' +
      '<table><thead><tr><th>模块</th><th>总数</th><th>已覆盖</th><th>未覆盖</th><th>覆盖率</th></tr></thead>' +
      '<tbody>' + moduleRows + '</tbody></table>' +
      '<h2>未覆盖端点清单</h2>' +
      '<table><thead><tr><th>方法</th><th>路径</th><th>模块</th><th>需鉴权</th></tr></thead>' +
      '<tbody>' + endpointRows + '</tbody></table>' +
      '</body></html>';
  }

  /**
   * 重置追踪数据
   */
  reset() {
    this.hitSet.clear();
    this.flush();
  }

  getCatalog() {
    return ALL_ENDPOINTS.map(endpoint => ({ ...endpoint }));
  }
}

const tracker = new EndpointCoverage();
tracker.ALL_ENDPOINTS = ALL_ENDPOINTS;
tracker.normalizePath = normalizePath;
module.exports = tracker;
