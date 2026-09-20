// 页面上线验收脚本：逐路由判定 A（可上线）/ B（空壳/占位）/ C（报错）。
//
// 与 diag-spa-mount.js 的区别：diag 只证明「路由可达、SPA 能挂载」；
// 本脚本进一步量化「页面里到底有没有可用 UI、数据接口是否健康」。
//
// 跑法：
//   cd automation_tests && set -a && . ./.env.local && set +a
//   node scripts/verify-page-launch.js                       # 默认清单（18 条待验收路由）
//   node scripts/verify-page-launch.js /device/asset /x/y     # 指定路由
//   VPL_JSON=1 node scripts/verify-page-launch.js            # 输出 JSON 便于后续处理
//
// 角色：路由 -> storage state，见 ROLE_OF。/apply/* 属 SYS_ADMIN 专区，必须用 super_admin，
// 否则会命中路由守卫 403 —— 那是权限设计而非页面缺陷。
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const ROLE_OF = {
  '/apply/service': 'super-admin'
};

const DEFAULT_ROUTES = [
  '/dashboard',
  '/product',
  '/apply/service',
  '/automation/rule-chain',
  '/automation/rule-chain/edit',
  '/dashboard/rdi-overview',
  '/dashboard/workbench',
  '/dashboard/workspace',
  '/device/asset',
  '/device/entity-relation',
  '/management/entity-version',
  '/management/role',
  '/product/pre-register',
  '/product/update-ota',
  '/product/update-package',
  '/system-management-user/equipment-map',
  '/visualization/scada',
  '/visualization/scada-editor'
];

// 「敬请期待」类占位文案命中即说明页面尚未实现。
const PLACEHOLDER_RE = /敬请期待|即将上线|Coming\s*soon|TODO|FIXME|功能开发中|暂未开放|under construction/i;
// 页面自报降级：核心依赖（如地图服务）不可用时，有些页面会显式提示并退化为占位。
// 这类页面能挂载、无报错，但核心能力没真正工作，不能算可上线。
const DEGRADED_RE = /service unavailable|fallback view|is unavailable|map service/i;

(async () => {
  const routes =
    process.argv.slice(2).filter(Boolean).length > 0 ? process.argv.slice(2).filter(Boolean) : DEFAULT_ROUTES;

  // 按角色分组，同一角色的路由复用一个 context，避免重复加载 storage state。
  const byRole = new Map();
  for (const r of routes) {
    const role = ROLE_OF[r] || 'tenant-admin';
    if (!byRole.has(role)) byRole.set(role, []);
    byRole.get(role).push(r);
  }

  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const results = [];

  for (const [role, roleRoutes] of byRole) {
    const storage = path.resolve(__dirname, '..', 'e2e', '.auth', `${role}.json`);
    if (!fs.existsSync(storage)) {
      console.error('storage state missing:', storage);
      process.exit(2);
    }
    const context = await browser.newContext({ storageState: storage, baseURL: 'http://127.0.0.1:9725' });

    for (const route of roleRoutes) {
      const page = await context.newPage();
      const pageErrors = [];
      const badResponses = []; // status >= 400
      const failedRequests = [];
      const consoleErrors = [];
      // VPL_TRACE=1 时记录接口响应体：区分「列表真的为空」与「接口返回错误信封被前端吞掉显示成 No Data」。
      const trace = !!process.env.VPL_TRACE;
      const apiTrace = [];
      // 业务码≠200 的接口：HTTP 层是 200，但信封里是错误码。
      // 这类失败最容易被漏判——前端往往把它吞成 "No Data"，页面看着正常其实列表永远拉不到。
      const apiBizErrors = [];

      page.on('pageerror', err => pageErrors.push(`${err.name}: ${err.message}`));
      page.on('console', msg => {
        if (msg.type() === 'error') consoleErrors.push(msg.text().slice(0, 300));
      });
      page.on('requestfailed', req =>
        failedRequests.push(`${req.method()} ${req.url()} -> ${req.failure() && req.failure().errorText}`)
      );
      page.on('response', async resp => {
        const url = resp.url();
        if (resp.status() >= 400) badResponses.push(`${resp.status()} ${resp.request().method()} ${url}`);
        if (/\/api\//.test(url)) {
          let body = '';
          try {
            body = (await resp.text()).slice(0, 300);
          } catch (e) {
            body = `<body unavailable: ${e.message}>`;
          }
          // 信封形如 {"code":200,...}；非 200 即为业务失败。
          const m = /"code"\s*:\s*(\d+)/.exec(body);
          if (m && m[1] !== '200') {
            apiBizErrors.push(`${url.replace(/^https?:\/\/[^/]+/, '')} -> code=${m[1]} ${(/("message"\s*:\s*"([^"]*)")/.exec(body) || [, , ''])[2]}`);
          }
          if (trace) apiTrace.push(`${resp.status()} ${resp.request().method()} ${url.replace(/^https?:\/\/[^/]+/, '')} :: ${body}`);
        }
      });

      let gotoError = '';
      try {
        await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 20000 });
        // 给异步接口留出落地时间，太短会把「数据还在路上」误判成空壳。
        await page.waitForTimeout(4000);
      } catch (e) {
        gotoError = e.message;
      }

      const m = await page.evaluate(() => {
        const app = document.getElementById('app');
        const visible = el => {
          const r = el.getBoundingClientRect();
          return r.width > 0 && r.height > 0;
        };
        const all = app ? Array.from(app.querySelectorAll('*')) : [];
        const vis = all.filter(visible);
        const text = (app && app.innerText ? app.innerText : '').trim();
        return {
          url: location.href,
          title: document.title,
          textLen: text.length,
          textSample: text.slice(0, 700),
          // 注意：evaluate 在浏览器上下文执行，取不到 Node 作用域的常量，正则需就地定义。
          degradedHit: /service unavailable|fallback view|is unavailable|map service/i.test(text.slice(0, 8000)),
          ui: {
            buttons: vis.filter(e => /^(BUTTON)$/.test(e.tagName) || e.getAttribute('role') === 'button').length,
            inputs: vis.filter(e => /^(INPUT|TEXTAREA|SELECT)$/.test(e.tagName)).length,
            formItems: vis.filter(e => e.classList && e.classList.contains('n-form-item')).length,
            // Naive UI 数据表的真实数据行（表头行不带 n-data-table-tr 之外的标记，这里按 tbody 内的 tr 计）
            tableRows: vis.filter(e => e.tagName === 'TR' && e.closest('tbody')).length,
            cards: vis.filter(e => e.classList && e.classList.contains('n-card')).length,
            tabs: vis.filter(e => e.classList && e.classList.contains('n-tabs')).length,
            canvases: vis.filter(e => e.tagName === 'CANVAS').length,
            svgs: vis.filter(e => e.tagName === 'SVG').length,
            totalVisible: vis.length
          }
        };
      });

      const placeholderHit = PLACEHOLDER_RE.test(m.textSample);
      const noPermission = /No Permission/i.test(m.title) || /\/403/.test(m.url);
      const degradedHit = !!m.degradedHit;

      // 只统计站内 API 的失败（排除 CDN / 地图等第三方，后者失败不代表页面不可用）
      const isInternal = u => /127\.0\.0\.1:9725|127\.0\.0\.1:9999|localhost/.test(u);
      const badApi = badResponses.filter(u => isInternal(u));
      const failedApi = failedRequests.filter(u => isInternal(u));

      results.push({
        route,
        role,
        finalUrl: m.url.replace('http://127.0.0.1:9725', ''),
        title: m.title,
        gotoError,
        pageErrors,
        consoleErrors,
        apiTrace,
        apiBizErrors,
        badApi,
        failedApi,
        badExternal: badResponses.filter(u => !isInternal(u)),
        placeholderHit,
        degradedHit,
        noPermission,
        textLen: m.textLen,
        textSample: m.textSample.replace(/\s+/g, ' ').slice(0, 400),
        ui: m.ui
      });

      await page.close();
    }
    await context.close();
  }

  await browser.close();

  // ---- 判定 ----
  // C：白屏 / 未挂载 / pageerror / 整页报错
  // B：能挂载但内容是占位、空壳，或核心数据接口 4xx/5xx
  // A：有实际 UI、无 pageerror、无大面积接口失败
  const grade = r => {
    if (r.gotoError) return { g: 'C', why: `goto 失败: ${r.gotoError}` };
    if (r.noPermission) return { g: 'C', why: '路由守卫 403（当前角色无权限）' };
    if (r.ui.totalVisible === 0) return { g: 'C', why: '白屏，#app 内无可渲染元素' };
    if (r.pageErrors.length) return { g: 'C', why: `pageerror: ${r.pageErrors[0]}` };
    if (r.placeholderHit) return { g: 'B', why: '命中占位文案（敬请期待/开发中）' };
    if (r.degradedHit) return { g: 'B', why: '页面自报降级（核心依赖不可用，退化为占位）' };
    if (r.badApi.length) return { g: 'B', why: `站内 API 失败 ${r.badApi.length} 条: ${r.badApi[0]}` };
    // HTTP 200 但业务码非 200：前端通常吞成空列表，页面看着正常、数据其实永远拉不到。
    if (r.apiBizErrors.length) return { g: 'B', why: `接口业务码非 200: ${r.apiBizErrors[0]}` };
    if (r.failedApi.length) return { g: 'C', why: `站内请求失败: ${r.failedApi[0]}` };
    // 有实际控件 + 有内容
    const interactive = r.ui.buttons + r.ui.inputs + r.ui.formItems + r.ui.tableRows + r.ui.cards + r.ui.canvases;
    if (r.ui.totalVisible < 20) return { g: 'B', why: `DOM 元素过少（visible=${r.ui.totalVisible}），疑似空壳` };
    if (interactive === 0) return { g: 'B', why: '无任何可交互控件/表格/卡片' };
    if (r.textLen < 30) return { g: 'B', why: `正文文本过短（${r.textLen} 字符）` };
    return { g: 'A', why: `OK：visible=${r.ui.totalVisible} btn=${r.ui.buttons} input=${r.ui.inputs} form=${r.ui.formItems} rows=${r.ui.tableRows} text=${r.textLen}` };
  };

  for (const r of results) {
    const { g, why } = grade(r);
    r.grade = g;
    r.gradeWhy = why;
  }

  if (process.env.VPL_JSON) {
    console.log(JSON.stringify(results, null, 2));
    return;
  }

  for (const r of results) {
    console.log(`\n========== ${r.route}  [${r.grade}] ==========`);
    console.log(`  -> ${r.finalUrl}  title="${r.title}"  role=${r.role}`);
    console.log(`  判定: ${r.gradeWhy}`);
    console.log(`  UI: ${JSON.stringify(r.ui)}  textLen=${r.textLen}`);
    if (r.pageErrors.length) console.log(`  pageerror: ${r.pageErrors.join(' | ')}`);
    if (r.consoleErrors.length) console.log(`  console.error: ${r.consoleErrors.join(' | ')}`);
    if (r.badApi.length) console.log(`  API>=400: ${r.badApi.join(' | ')}`);
    if (r.apiBizErrors.length) console.log(`  API业务码非200: ${r.apiBizErrors.join(' | ')}`);
    if (r.failedApi.length) console.log(`  requestfailed: ${r.failedApi.join(' | ')}`);
    if (r.badExternal.length) console.log(`  外部请求>=400: ${r.badExternal.join(' | ')}`);
    if (r.apiTrace.length) {
      console.log('  --- api trace ---');
      for (const t of r.apiTrace) console.log(`    ${t}`);
    }
    console.log(`  文本: ${r.textSample}`);
  }

  console.log('\n========== GRADE SUMMARY ==========');
  for (const r of results) console.log(`${r.grade}  ${r.route.padEnd(42)} ${r.gradeWhy}`);
})().catch(e => {
  console.error('FATAL', e);
  process.exit(1);
});
