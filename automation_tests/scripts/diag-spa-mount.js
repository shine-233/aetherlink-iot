// 诊断脚本：捕获 SPA 启动期的 console / pageerror / 失败请求，以及 #app 内部 HTML。
// 跑法：cd automation_tests && node scripts/diag-spa-mount.js
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

(async () => {
  // 角色可用 DIAG_ROLE 覆盖（文件名用连字符）：DIAG_ROLE=super_admin node scripts/diag-spa-mount.js ...
  // 用途：区分「路由真的不可达」与「当前角色本就不该看到」——
  // 例如 /apply/* 整棵只授 SYS_ADMIN，用 tenant_admin 扫会全是 403，那是预期而非缺陷。
  const role = (process.env.DIAG_ROLE || 'tenant_admin').replace(/_/g, '-');
  const storage = path.resolve(__dirname, '..', 'e2e', '.auth', `${role}.json`);
  if (!fs.existsSync(storage)) {
    console.error('storage state missing:', storage);
    process.exit(2);
  }

  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const context = await browser.newContext({ storageState: storage, baseURL: 'http://127.0.0.1:9725' });
  const page = await context.newPage();

  const logs = [];
  page.on('console', msg => logs.push(`[console.${msg.type()}] ${msg.text()}`));
  page.on('pageerror', err => logs.push(`[pageerror] ${err.name}: ${err.message}\n${err.stack || ''}`));
  page.on('requestfailed', req => logs.push(`[requestfailed] ${req.method()} ${req.url()} -> ${req.failure() && req.failure().errorText}`));
  page.on('response', resp => {
    if (resp.status() >= 400) logs.push(`[response.${resp.status()}] ${resp.request().method()} ${resp.url()}`);
  });

  // 路由可从命令行传入，便于做全量可达性扫描：
  //   node scripts/diag-spa-mount.js /a /b /c
  //   node scripts/diag-spa-mount.js $(cat /tmp/routes.txt)
  // 不传则用下面这份默认清单。
  const ROUTES =
    process.argv.slice(2).filter(Boolean).length > 0
      ? process.argv.slice(2).filter(Boolean)
      : ['/login', '/visualization/anomaly', '/visualization/report', '/market/browse', '/management/edge-nodes', '/management/license', '/device/grouping'];

  const summary = [];

  for (const route of ROUTES) {
    logs.push(`\n========== goto ${route} ==========`);
    try {
      await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 15000 });
      await page.waitForTimeout(2500);
    } catch (e) {
      logs.push(`[goto.error] ${e.message}`);
    }
    const info = await page.evaluate(() => {
      const app = document.getElementById('app');
      return {
        url: location.href,
        title: document.title,
        appChildren: app ? app.children.length : -1,
        appHTMLLen: app ? app.innerHTML.length : -1,
        appTextSample: app ? (app.innerText || '').slice(0, 200) : '',
        bodyTextSample: (document.body.innerText || '').slice(0, 200),
        hasRoot: !!document.querySelector('#app > *')
      };
    });
    logs.push(`[info] url=${info.url} title="${info.title}" appChildren=${info.appChildren} appHTMLLen=${info.appHTMLLen} hasRoot=${info.hasRoot}`);
    logs.push(`[body.text] ${info.bodyTextSample.replace(/\n/g, ' | ')}`);

    // 判定：路由表里有、菜单里没有的路径会被守卫跳 403（不是 404）；
    // appChildren=0 表示 SPA 根本没挂载（白屏，通常是运行期异常，看上面的 pageerror）。
    let verdict = 'OK';
    if (info.appChildren <= 0) verdict = 'BLANK';
    else if (/No Permission/i.test(info.title) || /403/.test(info.url)) verdict = '403';
    else if (/Not Found|404/i.test(info.title)) verdict = '404';
    summary.push(`${verdict.padEnd(5)} ${route}  (children=${info.appChildren}, len=${info.appHTMLLen})`);
  }

  console.log(logs.join('\n'));
  console.log('\n========== SUMMARY ==========');
  console.log(summary.join('\n'));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });